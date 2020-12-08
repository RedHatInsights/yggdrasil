package yggdrasil

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"git.sr.ht/~spc/go-log"
)

// ProcessManager spawns workers and monitors the processes. If a worker dies,
// a new one is spawned.
type ProcessManager struct {
	workers      map[int]string
	lock         sync.RWMutex
	logger       *log.Logger
	workerDied   chan int
	workerReaped chan<- int
}

// NewProcessManager creates a new ProcessManager. If provided, the reaped
// channel will receive PIDs of worker processes that have been reaped from the
// manager's process map.
func NewProcessManager(reaped chan<- int) *ProcessManager {
	var m ProcessManager
	m.workers = make(map[int]string)
	m.logger = log.New(log.Writer(), fmt.Sprintf("%v[process_manager] ", log.Prefix()), log.Flags(), log.CurrentLevel())
	m.workerDied = make(chan int, 3)
	m.workerReaped = reaped

	return &m
}

// StartWorker executes file and asynchronously waits for the process to die.
func (m *ProcessManager) StartWorker(file string) (int, error) {
	m.logger.Debugf("starting worker: %v", file)
	cmd := exec.Command(file)
	if err := cmd.Start(); err != nil {
		return -1, err
	}
	m.logger.Tracef("worker started with pid: %v", cmd.Process.Pid)

	m.lock.Lock()
	m.workers[cmd.Process.Pid] = file
	m.lock.Unlock()

	go m.WatchWorker(cmd.Process.Pid)

	return cmd.Process.Pid, nil
}

// WatchWorker waits for the process with pid to exit.
func (m *ProcessManager) WatchWorker(pid int) {
	m.logger.Tracef("watching worker with pid: %v", pid)
	process, err := os.FindProcess(pid)
	if err != nil {
		m.logger.Debug(err)
	}
	state, err := process.Wait()
	if err != nil {
		m.logger.Debug(err)
	}
	m.logger.Tracef("worker died: %v", pid)
	m.workerDied <- state.Pid()
}

// ReapWorkers waits for pids to be reported as dead, and reaps them from the
// worker map.
func (m *ProcessManager) ReapWorkers() {
	for {
		pid := <-m.workerDied
		m.logger.Tracef("reaping worker with pid: %v", pid)
		m.lock.RLock()
		worker := m.workers[pid]
		m.lock.RUnlock()

		m.lock.Lock()
		delete(m.workers, pid)
		m.lock.Unlock()

		if m.workerReaped != nil {
			m.workerReaped <- pid
		}

		go m.StartWorker(worker)
	}
}
