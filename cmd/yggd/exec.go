package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// startProcess executes file, setting up the environment using the provided
// env values. If started is not nil, it is invoked on a goroutine after the
// process has been started.
func startProcess(file string, args []string, env []string, started func(pid int, stdout io.ReadCloser, stderr io.ReadCloser)) error {
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("cannot find file: %v", err)
	}

	cmd := exec.Command(file, args...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot connect to stdout: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("cannot connect to stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot start process: %v: %v", file, err)
	}

	if started != nil {
		go started(cmd.Process.Pid, stdout, stderr)
	}

	return nil
}

// waitProcess finds a process with the given pid and waits for it to exit.
// If died is not nil, it is invoked on a goroutine when the process exits.
func waitProcess(pid int, died func(pid int, state *os.ProcessState)) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process with pid: %v", err)
	}

	state, err := process.Wait()
	if err != nil {
		return fmt.Errorf("process %v exited with error: %v", process.Pid, err)
	}

	if died != nil {
		go died(process.Pid, state)
	}

	return nil
}

// stopProcess finds a process with the given pid and kills it.
func stopProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process with pid: %v", err)
	}

	if err := process.Kill(); err != nil {
		return fmt.Errorf("cannot stop process: %v", err)
	}

	return nil
}
