package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/redhatinsights/yggdrasil/worker"
)

var sleepTime time.Duration
var abort chan string

func handler(w *worker.Worker, addr string, id string, responseTo string, cancelID string, metadata map[string]string, data []byte) error {
	abort = make(chan string)
	log.Tracef("handling message")
	if cancelID == "" {
		return echo(w, addr, id, responseTo, cancelID, metadata, data)
	} else {
		log.Tracef("sending abort execution to message: %v", cancelID)
		abort <- cancelID
	}
	return nil
}

// echo opens a new dbus connection and calls the
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method, returning the metadata and
// data it received.
func echo(w *worker.Worker, addr string, id string, responseTo string, cancelID string, metadata map[string]string, data []byte) error {
	log.Tracef("echoing")
	if sleepTime > 0 {
		log.Infof("sleeping: %v", sleepTime)
		time.Sleep(sleepTime)
	}

	select {
	case cID := <-abort:
		if cID == id {
			log.Tracef("cancelling echo message id: %v", id)
			return nil
		}
	default:
	}

	if err := w.EmitEvent(ipc.WorkerEventNameWorking, fmt.Sprintf("echoing %v", data)); err != nil {
		return fmt.Errorf("cannot call EmitEvent: %w", err)
	}

	responseCode, responseMetadata, responseData, err := w.Transmit(addr, id, responseTo, cancelID, metadata, data)
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

func events(event ipc.DispatcherEvent) {
	switch event {
	case ipc.DispatcherEventReceivedDisconnect:
		os.Exit(1)
	}
}

func main() {
	var (
		logLevel      string
		remoteContent bool
	)

	flag.StringVar(&logLevel, "log-level", "error", "set log level")
	flag.BoolVar(&remoteContent, "remote-content", false, "connect as a remote content worker")
	flag.DurationVar(&sleepTime, "sleep", 0, "sleep time in seconds before echoing the response")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("error: cannot parse log level: %v", err)
	}
	log.SetLevel(level)

	w, err := worker.NewWorker("echo", remoteContent, map[string]string{"DispatchedAt": "", "Version": "1"}, handler, events)
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
