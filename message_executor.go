package yggdrasil

import (
	"os"
	"os/exec"
)

type MessageExecutor interface {
	Run() error
}

type EchoMessageExecutor struct {
	Text string
}

func (e EchoMessageExecutor) Run() error {
	cmd := exec.Command("echo", e.Text)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}
