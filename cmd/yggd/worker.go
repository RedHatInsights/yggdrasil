package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/pelletier/go-toml"
	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
)

type workerConfig struct {
	Exec string `toml:"exec"`
}

func loadWorkerConfig(file string) (*workerConfig, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var config workerConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot load config: %w", err)
	}

	return &config, nil
}

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
				config, err := loadWorkerConfig(e.Path())
				if err != nil {
					log.Errorf("cannot load worker config: %v", err)
				}
				go startProcess(config.Exec, env, 0, died)
			}
		case notify.InDelete, notify.InMovedFrom:
			config, err := loadWorkerConfig(e.Path())
			if err != nil {
				log.Errorf("cannot load worker config: %v", err)
				continue
			}
			workerName := filepath.Base(config.Exec)
			pidFilePath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers", workerName+".pid")

			if err := killWorker(pidFilePath); err != nil {
				log.Errorf("cannot kill worker: %v", err)
				continue
			}
		}
	}
}
