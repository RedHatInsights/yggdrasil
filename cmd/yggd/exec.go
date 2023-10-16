package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
)

func startProcess(file string, env []string, delay time.Duration, died chan int) {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		log.Warnf("cannot start worker: %v", err)
		return
	}

	cmd := exec.Command(file)
	cmd.Env = env

	if delay < 0 {
		log.Errorf("failed to start worker '%v' too many times", file)
		return
	}

	if delay > 0 {
		log.Tracef("delaying worker start for %v...", delay)
		time.Sleep(delay)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Errorf("cannot connect to stdout: %v", err)
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Errorf("cannot connect to stderr: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Errorf("cannot start worker: %v: %v", file, err)
		return
	}
	log.Debugf("started process: %v", cmd.Process.Pid)

	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := stdout.Read(buf)
			if n > 0 {
				log.Tracef("[%v] %v", file, strings.TrimRight(string(buf), "\n\x00"))
			}
			if err != nil {
				switch err {
				case io.EOF:
					log.Debugf("%v stdout reached EOF: %v", file, err)
					return
				default:
					log.Errorf("cannot read from stdout: %v", err)
					continue
				}
			}
		}
	}()

	go func() {
		for {
			buf := make([]byte, 4096)
			n, err := stderr.Read(buf)
			if n > 0 {
				log.Errorf("[%v] %v", file, strings.TrimRight(string(buf), "\n\x00"))
			}
			if err != nil {
				switch err {
				case io.EOF:
					log.Debugf("%v stderr reached EOF: %v", file, err)
					return
				default:
					log.Errorf("cannot read from stderr: %v", err)
					continue
				}
			}
		}
	}()

	pidDirPath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers")

	if err := os.MkdirAll(pidDirPath, 0755); err != nil {
		log.Errorf("cannot create directory: %v", err)
		return
	}

	if err := os.WriteFile(filepath.Join(pidDirPath, filepath.Base(file)+".pid"), []byte(fmt.Sprintf("%v", cmd.Process.Pid)), 0644); err != nil {
		log.Errorf("cannot write to file: %v", err)
		return
	}

	go watchProcess(cmd, delay, died)
}

func watchProcess(cmd *exec.Cmd, delay time.Duration, died chan int) {
	log.Debugf("watching process: %v", cmd.Process.Pid)

	state, err := cmd.Process.Wait()
	if err != nil {
		log.Errorf("process %v exited with error: %v", cmd.Process.Pid, err)
	}

	died <- state.Pid()

	if state.SystemTime() < time.Duration(1*time.Second) {
		delay += 5 * time.Second
	}
	if delay >= time.Duration(30*time.Second) {
		delay = -1
	}

	go startProcess(cmd.Path, cmd.Env, delay, died)
}

func killProcess(pid int) error {
	process, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("cannot find process with pid: %w", err)
	}
	if err := process.Kill(); err != nil {
		log.Errorf("cannot kill process: %v", err)
	} else {
		log.Infof("killed process %v", process.Pid)
	}
	return nil
}

func killWorker(pidFile string) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("cannot read contents of file: %w", err)
	}
	pid, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse file contents as int: %w", err)
	}

	if err := killProcess(int(pid)); err != nil {
		return fmt.Errorf("cannot kill process: %w", err)
	}

	if err := os.Remove(pidFile); err != nil {
		return fmt.Errorf("cannot remove file: %w", err)
	}
	return nil
}

func killWorkers() error {
	pidDirPath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers")
	if err := os.MkdirAll(pidDirPath, 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}
	fileInfos, err := os.ReadDir(pidDirPath)
	if err != nil {
		return fmt.Errorf("cannot read contents of directory: %w", err)
	}

	for _, info := range fileInfos {
		pidFilePath := filepath.Join(pidDirPath, info.Name())
		if err := killWorker(pidFilePath); err != nil {
			return fmt.Errorf("cannot kill worker: %w", err)
		}
	}

	return nil
}

func watchWorkerDir(dir string, env []string, died chan int) {
	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(dir, c, notify.InCloseWrite, notify.InDelete, notify.InMovedFrom, notify.InMovedTo); err != nil {
		log.Errorf("cannot start notify watchpoint: %v", err)
		return
	}
	defer notify.Stop(c)

	for e := range c {
		log.Debugf("received inotify event %v", e.Event())
		switch e.Event() {
		case notify.InCloseWrite, notify.InMovedTo:
			if strings.HasSuffix(e.Path(), "worker") {
				if ExcludeWorkers[filepath.Base(e.Path())] {
					continue
				}
				log.Tracef("new worker detected: %v", e.Path())
				go startProcess(e.Path(), env, 0, died)
			}
		case notify.InDelete, notify.InMovedFrom:
			workerName := filepath.Base(e.Path())
			pidFilePath := filepath.Join(
				yggdrasil.LocalstateDir,
				"run",
				yggdrasil.LongName,
				"workers",
				workerName+".pid",
			)

			if err := killWorker(pidFilePath); err != nil {
				log.Errorf("cannot kill worker: %v", err)
				continue
			}
		}
	}
}
