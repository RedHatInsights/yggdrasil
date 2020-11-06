package yggdrasil

import (
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

type MessageExecutor interface {
	Run() (string, error)
}

type EchoMessageExecutor struct {
	Text string
}

func (e EchoMessageExecutor) Run() (string, error) {
	cmd := exec.Command("echo")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if _, err := io.WriteString(stdin, e.Text); err != nil {
		return "", err
	}
	stdin.Close()

	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(data), err
}

// PythonMessageExecutor runs python code by piping the code to the python
// interpreter's stdin and pipes the stdout back.
type PythonMessageExecutor struct {
	Code string
}

func (e PythonMessageExecutor) Run() (string, error) {
	cmd := exec.Command("/usr/libexec/platform-python")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	if _, err := io.WriteString(stdin, e.Code); err != nil {
		return "", err
	}
	stdin.Close()

	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(data), err
}

// AnsibleMessageExecutor runs ansible with the given hostname and module and
// pipes the stdout back.
type AnsibleMessageExecutor struct {
	Module   string
	Hostname string
}

func (e AnsibleMessageExecutor) Run() (string, error) {
	cmd := exec.Command("ansible", e.Hostname, "-m", e.Module)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	data, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return string(data), err
}
