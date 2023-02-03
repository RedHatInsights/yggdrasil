package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/redhatinsights/yggdrasil/worker"
)

// echo opens a new dbus connection and calls the
// com.redhat.yggdrasil.Dispatcher1.Transmit method, returning the metadata and
// data it received.
func echo(w *worker.Worker, addr string, id string, metadata map[string]string, data []byte) error {
	responseCode, responseMetadata, responseData, err := w.Transmit(addr, id, metadata, data)
	if err != nil {
		return fmt.Errorf("cannot call Transmit: %w", err)
	}

	// Log the responses received from the Dispatcher, if any.
	log.Infof("responseCode = %v", responseCode)
	log.Infof("responseMetadata = %#v", responseMetadata)
	log.Infof("responseData = %v", responseData)

	if err := w.SetFeature("DispatchedAt", time.Now().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("cannot set feature: %w", err)
	}

	return nil
}

func main() {
	// Get the log level specified by yggd via the YGG_LOG_LEVEL environment
	// variable.
	if logLevel, has := os.LookupEnv("YGG_LOG_LEVEL"); has {
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			log.Fatalf("error: cannot parse log level: %v", err)
		}
		log.SetLevel(level)
	}

	w, err := worker.NewWorker("echo", false, map[string]string{"DispatchedAt": "", "Version": "1"}, echo)
	if err != nil {
		log.Fatalf("error: cannot create worker: %v", err)
	}

	// Set up a channel to receive the TERM or INT signal over and clean up
	// before quitting.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if err := w.Connect(quit); err != nil {
		log.Fatalf("error: cannot connect: %v", err)
	}
}
