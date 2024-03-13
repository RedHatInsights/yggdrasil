package main

import (
	"io"
	"os"
	"testing"
)

func TestStartProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			err := startProcess(test.input.file, test.input.args, test.input.env, nil)
			if err != nil {
				t.Fatalf("unable to start process: %v", err)
			}
		})
	}
}

func TestStopProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"3"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			err := startProcess(
				test.input.file,
				test.input.args,
				test.input.env,
				func(pid int, stdout, stderr io.ReadCloser) {
					if err := stopProcess(pid); err != nil {
						t.Fatalf("unable to stop process: %v", err)
					}
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWaitProcess(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			file string
			args []string
			env  []string
		}
	}{
		{
			input: struct {
				file string
				args []string
				env  []string
			}{
				file: "/usr/bin/sleep",
				args: []string{"3"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {

			err := startProcess(
				test.input.file,
				test.input.args,
				test.input.env,
				func(startPid int, stdout, stderr io.ReadCloser) {
					if err := waitProcess(startPid, func(stopPid int, state *os.ProcessState) {
						if startPid != stopPid {
							t.Fatalf("%v != %v", startPid, stopPid)
						}
						if !state.Exited() {
							t.Fatalf("unexpected process exit state")
						}
					}); err != nil {
						t.Fatalf("unable to wait for process with pid %v: %v", startPid, err)
					}
				},
			)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
