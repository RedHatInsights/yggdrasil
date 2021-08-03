package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
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
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			log.Tracef("[%v] %v", file, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("cannot read from stdout: %v", err)
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Errorf("[%v] %v", file, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("cannot read from stderr: %v", err)
		}
	}()

	pidDirPath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers")

	if err := os.MkdirAll(pidDirPath, 0755); err != nil {
		log.Errorf("cannot create directory: %v", err)
		return
	}

	if err := ioutil.WriteFile(filepath.Join(pidDirPath, filepath.Base(file)+".pid"), []byte(fmt.Sprintf("%v", cmd.Process.Pid)), 0644); err != nil {
		log.Errorf("cannot write to file: %v", err)
		return
	}

	go waitProcess(cmd, delay, died)
}

func waitProcess(cmd *exec.Cmd, delay time.Duration, died chan int) {
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
