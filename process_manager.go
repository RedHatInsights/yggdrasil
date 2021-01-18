package yggdrasil

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	pid  int
	file string
}

// ProcessManager spawns processes and monitors them for unexpected exits.
// If a managed process unexpectedly exits, it is spawned again.
type ProcessManager struct {
	logger *log.Logger
	sig    signalEmitter
	db     *memdb.MemDB
}

// NewProcessManager creates a new ProcessManager.
func NewProcessManager(db *memdb.MemDB) (*ProcessManager, error) {
	p := new(ProcessManager)
	p.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), p), log.Flags(), log.CurrentLevel())

	p.db = db

	return p, nil
}

// Connect assigns a channel in the signal table under name for the caller to
// receive event updates.
func (p *ProcessManager) Connect(name string) <-chan interface{} {
	return p.sig.connect(name, 1)
}

// StartProcess executes file and asynchronously waits for the process to die.
func (p *ProcessManager) StartProcess(file string) {
	p.logger.Debugf("StartProcess(%v)", file)
	cmd := exec.Command(file)
	if err := cmd.Start(); err != nil {
		p.logger.Error(err)
	}

	process := Process{
		pid:  cmd.Process.Pid,
		file: file,
	}

	p.sig.emit(SignalProcessSpawn, process)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessSpawn)
	p.logger.Tracef("emitted value: %#v", process)

	tx := p.db.Txn(true)
	tx.Insert("process", &process)
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

	go p.StartProcess(process.file)
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
			p.StartProcess(filepath.Join(dir, info.Name()))
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
