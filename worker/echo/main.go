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

// echo opens a new dbus connection and calls the
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method, returning the
// metadata and data. New ID is generated for the message, and
// response_to is set to the ID of the message we received.
func echo(
	w *worker.Worker,
	addr string,
	rcvId string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) error {
	if err := w.EmitEvent(ipc.WorkerEventNameWorking, rcvId, fmt.Sprintf("echoing %v", data)); err != nil {
		return fmt.Errorf("cannot call EmitEvent: %w", err)
	}

	// Sleep time between receiving the message and sending it
	if sleepTime > 0 {
		log.Infof("sleeping: %v", sleepTime)
		time.Sleep(sleepTime)
	}

	responseCode, responseMetadata, responseData, err := w.Transmit(addr, id, responseTo, metadata, data)
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

	w, err := worker.NewWorker(
		"echo",
		remoteContent,
		map[string]string{"DispatchedAt": "", "Version": "1"},
		echo,
		events,
	)
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
