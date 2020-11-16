package yggdrasil

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

// Job wraps a playbook along with status and output information.
type Job struct {
	ID       string `json:"id"`
	Playbook string `json:"playbook"`
	Status   string `json:"status"`
	Stdout   string `json:"stdout"`
}

// PlaybookJobController implements JobController for an Ansible playbook.
type PlaybookJobController struct {
	job    Job
	client *HTTPClient
	url    string
}

// Start begins execution of the job.
func (j *PlaybookJobController) Start() error {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		return err
	}
	if _, err := f.WriteString(j.job.Playbook); err != nil {
		return err
	}
	defer func() {
		name := f.Name()
		f.Close()
		os.Remove(name)
		j.Finish()
	}()

	cmd := exec.Command("ansible-playbook", "--ssh-common-args=-oStrictHostKeyChecking=no", f.Name())
	cmd.Stderr = os.Stderr
	output, err := cmd.StdoutPipe()
	if err != nil {
		j.job.Status = "failed"
		j.job.Stdout = err.Error()
		return err
	}

	if err := cmd.Start(); err != nil {
		j.job.Status = "failed"
		j.job.Stdout = err.Error()
		return err
	}

	reader := bufio.NewReader(output)
	for {
		buf := make([]byte, 4096)
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			j.job.Status = "failed"
			j.job.Stdout += err.Error()
			break
		}
		j.job.Status = "running"
		j.job.Stdout += string(buf[:n])

		if err := j.Update("running", string(buf[:n])); err != nil {
			j.job.Status = "failed"
			j.job.Stdout = err.Error()
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		j.job.Status = "failed"
		j.job.Stdout = err.Error()
		return err
	}

	if j.job.Status != "" {
		j.job.Status = "succeeded"
	}

	return nil
}

// Update sends a "running" status update, along with a slice of stdout to the
// job service.
func (j *PlaybookJobController) Update(status, stdout string) error {
	update := struct {
		Status string `json:"status"`
		Stdout string `json:"stdout"`
	}{
		Status: status,
		Stdout: stdout,
	}
	data, err := json.Marshal(update)
	if err != nil {
		return err
	}

	resp, err := j.client.Patch(j.url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return &APIResponseError{
			Code: resp.StatusCode,
			body: string(data),
		}
	}
	return nil
}

// Finish completes the job by sending a "complete" status to the job service.
func (j *PlaybookJobController) Finish() error {
	data, err := json.Marshal(j.job)
	if err != nil {
		return err
	}

	resp, err := j.client.Put(j.url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusAccepted {
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		return &APIResponseError{
			Code: resp.StatusCode,
			body: string(data),
		}
	}
	return nil
}
