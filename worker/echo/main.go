package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"git.sr.ht/~spc/go-log"

	"github.com/redhatinsights/yggdrasil/internal/sync"
	"github.com/redhatinsights/yggdrasil/ipc"
	"github.com/redhatinsights/yggdrasil/worker"
)

var sleepTime time.Duration
var loopIt int

// syncMapCancelChan is a map of channels that maps message ID and its current
// work. This allows for the cancellation of a message that has not finished.
var syncMapCancelChan sync.RWMutexMap[chan struct{}]

// echo handles the echo message and sets the channel that will manage the
// cancel message. It runs a loop and a sleep according to the loop and
// sleep parameters, then calls the echo function to transmit the
// message. If there is a cancellation message during the loop or the sleep
// time, it will cancel the transmission of the message and finish the work.
func echo(
	w *worker.Worker,
	addr string,
	rcvId string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) error {
	if err := w.EmitEvent(
		ipc.WorkerEventNameWorking,
		rcvId,
		responseTo,
		map[string]string{"message": fmt.Sprintf("echoing %v", data)},
	); err != nil {
		return fmt.Errorf("cannot call EmitEvent: %w", err)
	}

	// Setting the channel to handle cancellation
	syncMapCancelChan.Set(rcvId, make(chan struct{}))

	// Loop the echoes
	for i := 0; i < loopIt; i++ {
		// Sleep time between receiving the message and sending it
		if sleepTime > 0 {
			log.Infof("sleeping: %v", sleepTime)
			time.Sleep(sleepTime)
		}
		// Cancel message if it has been sent a cancel message
		// during sleep time or during the loop
		cancelChan, _ := syncMapCancelChan.Get(rcvId)
		select {
		case <-cancelChan:
			// if the channel is close delete the channel from map
			// it will not be longer used.
			log.Tracef("canceled echo message id: %v", rcvId)
			log.Tracef("deleting channel from map")
			syncMapCancelChan.Del(rcvId)
			return nil
		default:
			if err := sendEchoMessage(w, addr, rcvId, responseTo, metadata, data, i); err != nil {
				return err
			}
		}

	}

	log.Tracef("deleting channel from map")
	syncMapCancelChan.Del(rcvId)
	return nil
}

// sendEchoMessage opens a new dbus connection and calls the
// com.redhat.Yggdrasil1.Dispatcher1.Transmit method, returning the
// metadata and data. New ID is generated for the message, and
// response_to is set to the ID of the message we received.
func sendEchoMessage(
	w *worker.Worker,
	addr string,
	rcvId string,
	responseTo string,
	metadata map[string]string,
	data []byte,
	count int,
) error {
	// Set "response_to" according to the rcvId of the message we received
	echoResponseTo := rcvId
	// Create new echoId for the message we are going to send
	echoId := uuid.New().String()

	responseCode, responseMetadata, responseData, err := w.Transmit(
		addr,
		echoId,
		echoResponseTo,
		metadata,
		data,
	)
	if err != nil {
		return fmt.Errorf("cannot call Transmit: %w", err)
	}

	// Log the responses received from the Dispatcher, if any.
	log.Infof("responseCode = %v", responseCode)
	log.Infof("responseMetadata = %#v", responseMetadata)
	log.Infof("responseData = %v", responseData)
	log.Infof("message %v of %v", count+1, loopIt)

	if err := w.SetFeature("DispatchedAt", time.Now().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("cannot set feature: %w", err)
	}
	return nil
}

// cancelEcho receives a cancel message id via com.redhat.Yggdrasil1.Worker1.Cancel method
// closes the channel associated with that message to cancel its current run
func cancelEcho(w *worker.Worker, addr string, id string, cancelID string) error {
	log.Infof("cancelling message with id %v", cancelID)
	if cancelChan, exists := syncMapCancelChan.Get(cancelID); exists {
		close(cancelChan)
	} else {
		return fmt.Errorf("message with given id: %v does not exist and cannot be canceled", cancelID)
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
	flag.IntVar(&loopIt, "loop", 1, "number of loop echoes before finish echoing.")
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
		cancelEcho,
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
