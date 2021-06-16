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
	"github.com/rjeczalik/notify"
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

// ProcessManager is a specialized process lifecycle manager. It spawns
// processes and waits for the process to exit. If a managed process exits, it
// is spawned again.
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

// Disconnect removes and closes the channel from the signal table under name
// for the caller.
func (p *ProcessManager) Disconnect(name string, ch <-chan interface{}) {
	p.sig.disconnect(name, ch)
}

// Close closes all channels that have been assigned to signal listeners.
func (p *ProcessManager) Close() {
	p.sig.close()
}

// StartProcess executes file. The child process's stderr and stdout are
// connected to pipes that are read in two goroutines. If the log level is set
// to debug or higher, the child process output is logged. Once a process is
// started a file is created with the process PID. The process is emitted on the
// "process-spawn" signal. Finally, it calls WaitProcess asynchronously to wait
// for the process to exit.
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

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			p.logger.Debugf("[%v] %v", file, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			p.logger.Error(err)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			p.logger.Debugf("[%v] %v", file, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			p.logger.Error(err)
		}
	}()

	if err := cmd.Start(); err != nil {
		p.logger.Error(err)
		return
	}

	process := &Process{
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
	if err := tx.Insert(tableNameProcess, process); err != nil {
		p.logger.Error(err)
		tx.Abort()
		return
	}
	tx.Commit()

	go p.WaitProcess(process)
}

// WaitProcess waits for the process to exit. When the process exits, a
// time-alive value is calculated. If the time alive was less than 1 second, a 5
// second start time delay penalty is added to the process. Any output from
// stdout and stderr are printed and the process PID is emitted on the
// "process-die" signal. If the delay penalty is greater than 30 seconds, the
// process is assumed to be permanently dead and is not restarted. Otherwise,
// the process is asyncrhonously restarted with the delay penalty.
func (p *ProcessManager) WaitProcess(process *Process) {
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
	timeAlive := time.Since(process.startedAt)
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

	tx := p.db.Txn(true)
	if err := tx.Delete(tableNameProcess, process); err != nil {
		p.logger.Error(err)
		return
	}
	tx.Commit()

	p.sig.emit(SignalProcessDie, process.pid)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
	p.logger.Tracef("emitted value: %#v", process.pid)

	if delay > 30*time.Second {
		p.logger.Warnf("file %v has spawned too many times", process.file)
		return
	}

	go p.StartProcess(process.file, delay)
}

// StopProcess kills the process and removes the PID file.
func (p *ProcessManager) StopProcess(process *Process) error {
	proc, err := os.FindProcess(process.pid)
	if err != nil {
		return err
	}
	if err := proc.Kill(); err != nil {
		return err
	}
	p.logger.Debugf("stopped process %v", process.pid)
	if err := os.Remove(process.pidFile); err != nil {
		return err
	}
	p.logger.Debugf("removed PID file %v", process.pidFile)

	return nil
}

// BootstrapWorkers detects qualifying worker programs in dir and spawns them.
// To be considered for execution, the file must be executable, exist in the
// given directory, and end with the suffix "worker". Once all detected workers
// are started, the "process-bootstrap" signal is emitted.
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

// KillAllWorkers gets all actively managed worker processes and kills them,
// emitting "process-die" for each one.
func (p *ProcessManager) KillAllWorkers() error {
	p.logger.Debug("KillAllWorkers")

	tx := p.db.Txn(false)
	all, err := tx.Get(tableNameProcess, indexNameID)
	if err != nil {
		return err
	}

	for obj := all.Next(); obj != nil; obj = all.Next() {
		process := obj.(*Process)
		if err := p.StopProcess(process); err != nil {
			p.logger.Error(err)
		}

		tx := p.db.Txn(true)
		if err := tx.Delete(tableNameProcess, process); err != nil {
			tx.Abort()
			return err
		}
		tx.Commit()

		p.sig.emit(SignalProcessDie, process.pid)
		p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
		p.logger.Tracef("emitted value: %#v", process.pid)
	}

	return nil
}

// KillAllOrphans looks for PID files written by previous processes, and
// attempts to kill the process. Regardless of whether the process was
// successfully killed, the PID file is removed.
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
		p.logger.Debugf("found orphaned process with PID %v", proc.Pid)

		if err := proc.Kill(); err != nil {
			p.logger.Errorf("error killing process: %v", err)
		}

		if err := os.Remove(file); err != nil {
			return err
		}
	}

	return nil
}

// WatchForProcesses watches dir for inotify activity and waits for create and
// delete events. When a file is created, if it qualifies as a worker
// executable, it is started via a call to StartProcess. When a file is deleted,
// if a running process matches the deleted file, it is killed vua a call to
// StopProcess.
func (p *ProcessManager) WatchForProcesses(dir string) error {
	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(dir, c, notify.Create, notify.InDelete); err != nil {
		return err
	}
	defer notify.Stop(c)

	for ei := range c {
		p.logger.Debugf("notify event %v", ei)
		switch ei.Event() {
		case notify.Create:
			if strings.HasSuffix(ei.Path(), "worker") {
				p.logger.Debugf("found worker: %v", ei.Path())
				go p.StartProcess(ei.Path(), 1*time.Second)
			}
		case notify.InDelete:
			tx := p.db.Txn(false)
			obj, err := tx.First(tableNameProcess, indexNameFile, ei.Path())
			if err != nil {
				p.logger.Error(err)
				continue
			}
			process := obj.(*Process)
			if err := p.StopProcess(process); err != nil {
				p.logger.Error(err)
				continue
			}
		}
	}

	return nil
}
