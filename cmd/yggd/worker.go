package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/pelletier/go-toml"
	"github.com/redhatinsights/yggdrasil"
	"github.com/rjeczalik/notify"
	"golang.org/x/net/http/httpproxy"
)

type workerStartedFunc func(pid int)
type workerStoppedFunc func(pid int)

// workerConfig holds information necessary to start and manage a worker
// process.
type workerConfig struct {
	// Env is a slice of "KEY=VALUE" strings that get inserted into the worker's
	// environment when it is started.
	Env []string `toml:"env"`

	// delay is the current time to delay the worker if it is in a crash backoff
	// loop.
	delay time.Duration

	// program is the absolute path to the worker executable program.
	program string
}

// loadWorkerConfig reads the contents of file and parses it into a workerConfig
// value.
func loadWorkerConfig(file string) (*workerConfig, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %w", err)
	}

	var config workerConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot load config: %w", err)
	}
	log.Debugf("loaded worker config: %v", config)

	return &config, nil
}

// startWorker starts a worker represented by the given config. If the started
// function parameter is not nil, it is invoked when the worker process is
// successfully started. If the worker is successfully started, the process is
// waited upon until it stops. If the stopped function parameter is not nil, it
// is invoked when the worker process is stopped.
func startWorker(config workerConfig, started workerStartedFunc, stopped workerStoppedFunc) error {
	if config.program == "" {
		return fmt.Errorf("cannot start worker without program: %v", config)
	}

	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"YGG_SOCKET_ADDR=unix:" + SocketAddr,
		"YGG_CONFIG_DIR=" + filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName),
		"YGG_LOG_LEVEL=" + log.CurrentLevel().String(),
		"YGG_CLIENT_ID=" + ClientID,
	}

	proxy := httpproxy.FromEnvironment()
	if proxy.HTTPProxy != "" {
		env = append(env, "HTTP_PROXY="+proxy.HTTPProxy)
		env = append(env, "http_proxy="+proxy.HTTPProxy)
	}
	if proxy.HTTPSProxy != "" {
		env = append(env, "HTTPS_PROXY="+proxy.HTTPSProxy)
		env = append(env, "https_proxy="+proxy.HTTPSProxy)
	}
	if proxy.NoProxy != "" {
		env = append(env, "NO_PROXY="+proxy.NoProxy)
		env = append(env, "no_proxy="+proxy.NoProxy)
	}

	for _, val := range config.Env {
		if validEnvVar(val) {
			env = append(env, val)
		}
	}

	if config.delay < 0 {
		return fmt.Errorf("failed to start worker %v too many times", config.program)
	}

	if config.delay > 0 {
		log.Tracef("delaying worker start for %v...", config.delay)
		time.Sleep(config.delay)
	}

	log.Debugf("starting worker %v with environment: %v", config.program, env)
	err := startProcess(config.program, nil, env, func(pid int, stdout, stderr io.ReadCloser) {
		log.Infof("started worker: %v", config.program)

		pipe := func(r io.ReadCloser) {
			for {
				buf := make([]byte, 4096)
				n, err := r.Read(buf)
				if n > 0 {
					log.Tracef("[%v] %v", config.program, strings.TrimRight(string(buf), "\n\x00"))
				}
				if err != nil {
					switch err {
					case io.EOF:
						log.Debugf("%v reached EOF: %v", config.program, err)
						return
					default:
						log.Errorf("cannot read from reader: %v", err)
						continue
					}
				}
			}
		}

		go pipe(stdout)
		go pipe(stderr)

		pidDirPath := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers")

		if err := os.MkdirAll(pidDirPath, 0755); err != nil {
			log.Errorf("cannot create directory: %v", err)
			return
		}

		if err := os.WriteFile(filepath.Join(pidDirPath, filepath.Base(config.program)+".pid"), []byte(fmt.Sprintf("%v", pid)), 0644); err != nil {
			log.Errorf("cannot write to file: %v", err)
			return
		}

		if started != nil {
			go started(pid)
		}

		err := waitProcess(pid, func(pid int, state *os.ProcessState) {
			log.Infof("stopped worker: %v", config.program)
			if state.SystemTime() < 1*time.Second {
				config.delay += 5 * time.Second
			}

			if config.delay >= 30*time.Second {
				config.delay = -1
			}

			if stopped != nil {
				go stopped(pid)
			}

			if err := startWorker(config, started, stopped); err != nil {
				log.Errorf("cannot restart worker: %v", err)
				return
			}
		})
		if err != nil {
			log.Errorf("process exited with an error: %v", err)
			return
		}
	})
	if err != nil {
		return fmt.Errorf("cannot start worker: %v", err)
	}

	return nil
}

// stopWorker attempts to stop a worker with the given name.
func stopWorker(name string) error {
	pidFile := filepath.Join(
		yggdrasil.LocalstateDir,
		"run",
		yggdrasil.LongName,
		"workers",
		name+".pid",
	)

	data, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("cannot read file: %w", err)
	}
	pid, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return fmt.Errorf("cannot parse data: %w", err)
	}

	if err := stopProcess(int(pid)); err != nil {
		return fmt.Errorf("cannot stop worker: %w", err)
	}
	if err := os.Remove(pidFile); err != nil {
		return fmt.Errorf("cannot remove file: %w", err)
	}
	return nil
}

// startWorkers reads the contents of the worker executable directory and starts
// any valid workers found.
func startWorkers(started workerStartedFunc, stopped workerStoppedFunc) error {
	workerExecDir := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
	workerConfigDir := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "workers")

	infos, err := os.ReadDir(workerExecDir)
	if err != nil {
		return fmt.Errorf("cannot read worker directory: %v", err)
	}

	for _, info := range infos {
		if validWorker(info.Name()) {
			var config *workerConfig

			configFile := filepath.Join(workerConfigDir, info.Name()+".toml")
			if fileExists(configFile) {
				var err error

				config, err = loadWorkerConfig(configFile)
				if err != nil {
					return fmt.Errorf("cannot read worker config file: %w", err)
				}
			} else {
				config = &workerConfig{}
			}
			config.program = filepath.Join(workerExecDir, info.Name())

			if err := startWorker(*config, started, stopped); err != nil {
				return fmt.Errorf("cannot start worker %v: %w", info.Name(), err)
			}
		}
	}

	return nil
}

// stopWorkers reads all PID files from the local state directory and attempts
// to stop the worker process.
func stopWorkers() error {
	dir := filepath.Join(yggdrasil.LocalstateDir, "run", yggdrasil.LongName, "workers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create directory: %w", err)
	}
	infos, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("cannot read contents of directory: %w", err)
	}

	for _, info := range infos {
		if strings.HasSuffix(info.Name(), ".pid") {
			if err := stopWorker(strings.TrimSuffix(info.Name(), ".pid")); err != nil {
				return fmt.Errorf("cannot stop worker: %w", err)
			}
		}
	}

	return nil
}

// watchWorkerDir watches the worker exec directory for file modifications and
// restarts workers when appropriate.
func watchWorkerDir(died chan int) {
	workerExecDir := filepath.Join(yggdrasil.LibexecDir, yggdrasil.LongName)
	workerConfigDir := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "workers")
	c := make(chan notify.EventInfo, 1)

	if err := notify.Watch(workerExecDir, c, notify.InCloseWrite, notify.InDelete, notify.InMovedFrom, notify.InMovedTo); err != nil {
		log.Errorf("cannot start notify watchpoint: %v", err)
		return
	}
	defer notify.Stop(c)

	for e := range c {
		log.Debugf("received inotify event %v", e.Event())
		name := filepath.Base(e.Path())
		if !validWorker(name) {
			log.Warnf("invalid worker detected: %v", name)
			continue
		}
		switch e.Event() {
		case notify.InCloseWrite, notify.InMovedTo:
			var config *workerConfig

			configFile := filepath.Join(workerConfigDir, name+".toml")
			if fileExists(configFile) {
				var err error

				config, err = loadWorkerConfig(configFile)
				if err != nil {
					log.Errorf("cannot read worker config file: %v", err)
					continue
				}
			} else {
				config = &workerConfig{}
			}
			config.program = filepath.Join(workerExecDir, name)

			if err := startWorker(*config, nil, func(pid int) {
				died <- pid
			}); err != nil {
				log.Errorf("cannot start worker %v: %v", config.program, err)
				continue
			}
		case notify.InDelete, notify.InMovedFrom:
			if err := stopWorker(name); err != nil {
				log.Errorf("cannot stop worker: %v", err)
				continue
			}
		}
	}
}

func validEnvVar(val string) bool {
	for _, pattern := range []string{"PATH=.*", "YGG_.*=.*"} {
		r := regexp.MustCompile(pattern)
		if r.Match([]byte(val)) {
			log.Warnf("invalid environment variable detected: %v", val)
			return false
		}
	}

	return true
}

func validWorker(name string) bool {
	return strings.HasSuffix(name, "worker") && !ExcludeWorkers[name]
}
