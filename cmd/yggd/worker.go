package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
)

func killWorker(pidFile string) error {
	data, err := ioutil.ReadFile(pidFile)
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
	fileInfos, err := ioutil.ReadDir(pidDirPath)
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
				log.Tracef("new worker detected: %v", e.Path())
				go startProcess(e.Path(), env, 0, died)
			}
		case notify.InDelete, notify.InMovedFrom:
			workerName := filepath.Base(e.Path())
			pidFilePath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers", workerName+".pid")

			if err := killWorker(pidFilePath); err != nil {
				log.Errorf("cannot kill worker: %v", err)
				continue
			}
		}
	}
}
