package yggdrasil

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-memdb"
)

const (
	// SignalProcessSpawn is emitted when a process is spawned. The value
	// emitted on the channel is a yggdrasil.Process.
	SignalProcessSpawn = "process-spawn"

	// SignalProcessDie is emitted when a process dies. The value emitted on the
	// channel is the PID of the process that died.
	SignalProcessDie = "process-die"

	// SignalProcessBootstrap is emitted when all workers detected at startup
	// have been spawned. The value emitted on the channel is a bool.
	SignalProcessBootstrap = "process-bootstrap"
)

// Process encapsulates the information about a process monitored by
// ProcessManager.
type Process struct {
	pid       int
	file      string
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	startedAt time.Time
	pidFile   string
}

// ProcessManager spawns processes and monitors them for unexpected exits.
// If a managed process unexpectedly exits, it is spawned again.
type ProcessManager struct {
	logger     *log.Logger
	sig        signalEmitter
	db         *memdb.MemDB
	workerEnv  []string
	rw         sync.RWMutex
	delayStart map[string]time.Duration
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(db *memdb.MemDB, workerEnv []string) (*ProcessManager, error) {
	p := new(ProcessManager)
	p.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), p), log.Flags(), log.CurrentLevel())

	p.db = db
	p.workerEnv = workerEnv
	p.delayStart = make(map[string]time.Duration)

	return p, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (p *ProcessManager) Connect(name string) <-chan interface{} {
	return p.sig.connect(name, 1)
}

// StartProcess executes file and asynchronously waits for the process to die.
func (p *ProcessManager) StartProcess(file string, delay time.Duration) {
	p.logger.Debugf("StartProcess(%v)", file)
	cmd := exec.Command(file)
	cmd.Env = p.workerEnv

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.Error(err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.logger.Error(err)
		return
	}

	if delay < 0 {
		p.logger.Warnf("process %v has failed to start too many times", file)
		return
	}

	p.rw.Lock()
	p.delayStart[file] = delay
	p.rw.Unlock()

	p.logger.Tracef("sleeping for %v", delay)
	time.Sleep(delay)

	if err := cmd.Start(); err != nil {
		p.logger.Error(err)
		return
	}

	process := Process{
		pid:       cmd.Process.Pid,
		file:      file,
		stdout:    stdout,
		stderr:    stderr,
		startedAt: time.Now(),
		pidFile:   filepath.Join(LocalstateDir, "run", LongName, "workers", filepath.Base(file)+".pid"),
	}

	if err := os.MkdirAll(filepath.Dir(process.pidFile), 0755); err != nil {
		p.logger.Error(err)
		return
	}
	pidFile, err := os.Create(process.pidFile)
	if err != nil {
		p.logger.Error(err)
		return
	}
	defer pidFile.Close()
	if _, err := pidFile.WriteString(fmt.Sprintf("%v", cmd.Process.Pid)); err != nil {
		p.logger.Error(err)
		return
	}
	pidFile.Close()

	p.sig.emit(SignalProcessSpawn, process)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessSpawn)
	p.logger.Tracef("emitted value: %#v", process)

	tx := p.db.Txn(true)
	tx.Insert(tableNameProcess, &process)
	tx.Commit()

	go p.WaitProcess(process)
}

// WaitProcess waits for the process with pid to exit.
func (p *ProcessManager) WaitProcess(process Process) {
	p.logger.Debugf("WaitProcess(%v)", process)
	proc, err := os.FindProcess(process.pid)
	if err != nil {
		p.logger.Error(err)
	}
	_, err = proc.Wait()
	if err != nil {
		p.logger.Error(err)
	}

	var delay time.Duration
	timeAlive := time.Now().Sub(process.startedAt)
	if timeAlive < 1*time.Second {
		p.rw.Lock()
		p.delayStart[process.file] += 5 * time.Second
		delay = p.delayStart[process.file]
		p.rw.Unlock()
	}

	stdout := bufio.NewScanner(process.stdout)
	for stdout.Scan() {
		p.logger.Tracef("[%v] %v", process.file, stdout.Text())
	}
	if err := stdout.Err(); err != nil {
		p.logger.Error(err)
		return
	}

	stderr := bufio.NewScanner(process.stderr)
	for stderr.Scan() {
		p.logger.Errorf("[%v] %v", process.file, stderr.Text())
	}
	if err := stderr.Err(); err != nil {
		p.logger.Error(err)
		return
	}

	p.sig.emit(SignalProcessDie, process.pid)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
	p.logger.Tracef("emitted value: %#v", process.pid)

	tx := p.db.Txn(true)
	obj, err := tx.First(tableNameProcess, indexNameID, process.pid)
	if err != nil {
		p.logger.Error(err)
		return
	}
	tx.Delete(tableNameProcess, obj)
	tx.Commit()

	if delay > 30*time.Second {
		p.logger.Warnf("file %v has spawned too many times", process.file)
		return
	}

	go p.StartProcess(process.file, delay)
}

// BootstrapWorkers identifies any worker programs in dir and spawns them.
func (p *ProcessManager) BootstrapWorkers(dir string) error {
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range fileInfos {
		if strings.HasSuffix(info.Name(), "worker") {
			p.logger.Debugf("found worker: %v", info.Name())
			p.StartProcess(filepath.Join(dir, info.Name()), 0)
		}
	}

	p.sig.emit(SignalProcessBootstrap, true)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessBootstrap)
	p.logger.Tracef("emitted value: %#v", true)

	return err
}

// KillAllWorkers gets all actively managed worker processes and kills them.
func (p *ProcessManager) KillAllWorkers() error {
	p.logger.Debug("KillAllWorkers")

	tx := p.db.Txn(true)
	defer tx.Abort()

	all, err := tx.Get("process", "id")
	if err != nil {
		return err
	}
	for obj := all.Next(); obj != nil; obj = all.Next() {
		process := obj.(*Process)
		proc, err := os.FindProcess(process.pid)
		if err != nil {
			return err
		}
		if err := proc.Kill(); err != nil {
			return err
		}
		if err := os.Remove(process.pidFile); err != nil {
			return err
		}
		p.sig.emit(SignalProcessDie, process.pid)
		p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
		p.logger.Tracef("emitted value: %#v", process.pid)
	}

	if _, err := tx.DeleteAll(tableNameProcess, indexNameID); err != nil {
		return err
	}

	tx.Commit()

	return nil
}

// KillAllOrphans reads any PID files found left over from a previous process
// finds the process, kills it, and removes the PID file.
func (p *ProcessManager) KillAllOrphans() error {
	p.logger.Debug("KillAllOrphans")

	dir := filepath.Join(LocalstateDir, "run", LongName, "workers")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, info := range fileInfos {
		file := filepath.Join(dir, info.Name())
		data, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}

		pid, err := strconv.ParseInt(string(data), 10, 64)
		if err != nil {
			return err
		}

		proc, err := os.FindProcess(int(pid))
		if err != nil {
			return err
		}
		p.logger.Debugf("found orphaned worker process with PID %v", proc.Pid)

		if err := proc.Kill(); err != nil {
			return err
		}
		p.logger.Debugf("killed orphaned worker process with PID %v", proc.Pid)

		if err := os.Remove(file); err != nil {
			return err
		}
	}

	return nil
}
