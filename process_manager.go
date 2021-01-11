package yggdrasil

import (
	"fmt"
	"os"
	"os/exec"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-memdb"
)

const (
	// SignalProcessSpawn is emitted when a process is spawned. The value
	// emitted on the channel is a yggdrasil.Process.
	SignalProcessSpawn = "process-spawn"

	// SignalProcessDie is emitted when a process dies. The value emitted on the
	// channel is a yggdrasil.Process.
	SignalProcessDie = "process-die"
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
func NewProcessManager() (*ProcessManager, error) {
	p := new(ProcessManager)
	p.logger = log.New(log.Writer(), fmt.Sprintf("%v[%T] ", log.Prefix(), p), log.Flags(), log.CurrentLevel())

	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"process": {
				Name: "process",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:    "id",
						Unique:  true,
						Indexer: &memdb.IntFieldIndex{Field: "pid"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}
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

	p.sig.emit(SignalProcessDie, process)
	p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
	p.logger.Tracef("emitted value: %#v", process)

	tx := p.db.Txn(true)
	obj, err := tx.First("process", "id", process.pid)
	if err != nil {
		p.logger.Error(err)
		return
	}
	tx.Delete("process", obj)
	tx.Commit()

	go p.StartProcess(process.file)
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
		p.sig.emit(SignalProcessDie, *process)
		p.logger.Debugf("emitted signal \"%v\"", SignalProcessDie)
		p.logger.Tracef("emitted value: %#v", *process)
	}

	if _, err := tx.DeleteAll("process", "id"); err != nil {
		return err
	}

	tx.Commit()

	return nil
}
