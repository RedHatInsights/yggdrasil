package main

import (
	"flag"
	"fmt"
	"github.com/google/uuid"
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
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method, returning the metadata and
// data it received.
func echo(
	w *worker.Worker,
	rcvAddr string,
	rcvId string, responseTo string, metadata map[string]string, data []byte) error {

	// Emit D-Bus signal to indicate that the worker is still working.
	if err := w.EmitEvent(ipc.WorkerEventNameWorking, fmt.Sprintf("echoing %v", string(data))); err != nil {
		return fmt.Errorf("cannot call EmitEvent: %w", err)
	}

	// Sleep time between receiving the message and sending it
	if sleepTime > 0 {
		log.Infof("sleeping: %v", sleepTime)
		time.Sleep(sleepTime)
	}

	// Set "response_to" according to the id of the message we received
	echoResponseTo := rcvId
	// Create new id for the message we are going to send
	echoId := uuid.New().String()
	// Create a new echo address, when remote content is enabled
	echoAddr := ""
	if w.RemoteContent {
		if metadata["return_url"] != "" {
			echoAddr = metadata["return_url"]
		}
	} else {
		echoAddr = rcvAddr
	}

	log.Infof("echoing received message as (addr: %v, id: %v, responseTo: %s, metadata: %v, data: %v)",
		rcvAddr, echoId, echoResponseTo, metadata, string(data))
	responseCode, responseMetadata, responseData, err := w.Transmit(echoAddr, echoId, echoResponseTo, metadata, data)
	if err != nil {
		return fmt.Errorf("cannot call Transmit: %w", err)
	}

	// Log the responses received from the Dispatcher, if any.
	log.Infof("responseCode = %v", responseCode)
	log.Infof("responseMetadata = %#v", responseMetadata)
	log.Infof("responseData = %v", responseData)

	// Set the DispatchedAt D-Bus property to the current time.
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

	// Set up logging.
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("error: cannot parse log level: %v", err)
	}
	log.SetLevel(level)
	log.SetPrefix(fmt.Sprintf("[%v] ", os.Args[0]))
	if log.CurrentLevel() >= log.LevelDebug {
		log.SetFlags(log.LstdFlags | log.Llongfile)
	}

	// Show information about worker type.
	if remoteContent {
		log.Infof("connecting as a remote content worker")
	} else {
		log.Infof("connecting as a normal worker")
	}

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

	// Connect to the D-Bus bus and start listening for messages
	if err := w.Connect(quit); err != nil {
		log.Fatalf("error: cannot connect: %v", err)
	}

	log.Debug("finished")
}
