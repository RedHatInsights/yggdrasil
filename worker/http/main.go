package main

import (
	"encoding/json"
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

var (
	loops int
	sleep time.Duration
)

type request struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

// httpReq handles the incoming data received by the worker.
func httpReq(
	w *worker.Worker,
	addr string,
	messageID string,
	responseTo string,
	metadata map[string]string,
	data []byte,
) error {
	err := w.EmitEvent(
		ipc.WorkerEventNameWorking,
		messageID,
		responseTo,
		map[string]string{"message": fmt.Sprintf("working on request: %v", string(data))},
	)
	if err != nil {
		return fmt.Errorf("cannot emit event: %v", err)
	}

	var message request

	if err := json.Unmarshal(data, &message); err != nil {
		return fmt.Errorf("cannot unmarshal data: %v", err)
	}

	for i := 0; i < loops; i++ {
		responseCode, responseHeaders, responseBody, err := w.Request(
			message.Method,
			message.URL,
			message.Headers,
			message.Body,
		)
		if err != nil {
			return fmt.Errorf("call Request failed: %v", err)
		}

		log.Infof("responseCode = %v", responseCode)
		log.Infof("responseHeaders = %v", responseHeaders)
		log.Infof("responseBody = %v", responseBody)

		log.Infof("sleeping: %v", sleep)
		time.Sleep(sleep)
	}

	return nil
}

func main() {
	var logLevel string

	flag.IntVar(&loops, "loop", 1, "number of times to repeat sending the request")
	flag.DurationVar(&sleep, "sleep", 0, "duration to wait before sending the request")
	flag.StringVar(&logLevel, "log-level", "error", "set log level")
	flag.Parse()

	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("error: cannot parse log level: %v", err)
	}
	log.SetLevel(level)

	w, err := worker.NewWorker("http", nil, nil, httpReq, nil)
	if err != nil {
		log.Fatalf("error: cannot create worker: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	if err := w.Connect(quit); err != nil {
		log.Fatalf("error: cannot connect worker: %v", err)
	}
}
