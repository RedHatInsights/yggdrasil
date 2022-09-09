//go:build go1.16
// +build go1.16

package transport_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/yggdrasil/internal/transport"
)

func startServer() *http.Server {
	getResponse := func(w http.ResponseWriter) {
		data := map[string]string{
			"status": "OK",
		}
		resBytes, _ := json.Marshal(data)
		fmt.Fprintf(w, "%s", resBytes)
	}
	port, err := getFreePort()
	if err != nil {
		log.Fatal("cannot get free port")
	}

	http.HandleFunc("/yggdrasil/test/401/out", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		getResponse(w)
	})

	http.HandleFunc("/yggdrasil/test/500/out", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		getResponse(w)
	})

	http.HandleFunc("/yggdrasil/test/200/out", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		getResponse(w)
	})

	srv := &http.Server{
		Addr: fmt.Sprintf("localhost:%d", port),
	}

	go func() {
		// The error will be always not nil because we're going to shutdown the
		// server.
		_ = srv.ListenAndServe()
	}()
	return srv
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func TestSend(t *testing.T) {
	srv := startServer()
	defer func() {
		err := srv.Shutdown(context.Background())
		if err != nil {
			log.Fatal("cannot stop server: ", err)
		}
	}()
	server := srv.Addr
	freePort, err := getFreePort()
	if err != nil {
		t.Error("cannot get free port")
	}

	tests := []struct {
		description string
		server      string
		client      string
		err         bool
		response    bool
		statusCode  int
		body        json.RawMessage
	}{
		{
			description: "Invalid server",
			server:      fmt.Sprintf("localhost:%d", freePort),
			client:      "200",
			err:         true,
			response:    false,
		},
		{
			description: "200OK works as expected",
			server:      server,
			client:      "200",
			err:         false,
			response:    true,
			statusCode:  200,
			body:        []byte(`{"status":"OK"}`),
		},
		{
			description: "401 works as expected",
			server:      server,
			client:      "401",
			err:         true,
			response:    true,
			statusCode:  401,
			body:        []byte(`{"status":"OK"}`),
		},
		{
			description: "500 works as expected",
			server:      server,
			client:      "500",
			err:         true,
			response:    true,
			statusCode:  500,
			body:        []byte(`{"status":"OK"}`),
		},
	}

	cb := func([]byte, string) {}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			httpTransport, err := transport.NewHTTPTransport(test.client, test.server, nil, "testUA", time.Second, cb)
			if err != nil {
				t.Error("Cannot create new transport")
			}

			res, err := httpTransport.SendData([]byte("test"), "test")

			errorResult := err != nil
			if !cmp.Equal(errorResult, test.err) {
				t.Errorf("Error should match %#v != %#v, err:%v", errorResult, test.err, err)
			}
			if !test.response {
				if res != nil {
					t.Error("It has response when it shouldn't")
				}
				return
			}

			if len(res) == 0 {
				t.Error("Send should return a valid response")
			}

			var parsedResponse transport.HTTPResponse
			err = json.Unmarshal(res, &parsedResponse)
			if err != nil {
				t.Errorf("Cannot unmarshal response, err = %v", err)
			}

			if !cmp.Equal(parsedResponse.StatusCode, test.statusCode) {
				t.Errorf("Response statuscode is not the same %#v != %#v", parsedResponse.StatusCode, test.statusCode)
			}

			if !cmp.Equal(parsedResponse.Body, test.body) {
				t.Errorf("Response body is not the same %s != %s", parsedResponse.Body, test.body)
			}

			if len(parsedResponse.Metadata) == 0 {
				t.Error("Metadata shouldn't be empty")
			}
		})
	}
}
