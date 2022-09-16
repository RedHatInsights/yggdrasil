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
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func TestTx(t *testing.T) {
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
		serverAddr  string
		clientID    string
		wantError   error
		want        struct {
			code     int
			metadata map[string]string
			data     []byte
		}
	}{
		{
			description: "Invalid server",
			serverAddr:  fmt.Sprintf("localhost:%d", freePort),
			clientID:    "200",
			wantError:   &url.Error{},
		},
		{
			description: "200OK works as expected",
			serverAddr:  server,
			clientID:    "200",
			want: struct {
				code     int
				metadata map[string]string
				data     []byte
			}{
				code: 200,
				metadata: map[string]string{
					"Content-Length": "15",
					"Content-Type":   "text/plain; charset=utf-8",
				},
				data: []byte(`{"status":"OK"}`),
			},
		},
		{
			description: "401 works as expected",
			serverAddr:  server,
			clientID:    "401",
			want: struct {
				code     int
				metadata map[string]string
				data     []byte
			}{
				code: 401,
				metadata: map[string]string{
					"Content-Length": "15",
					"Content-Type":   "text/plain; charset=utf-8",
				},
				data: []byte(`{"status":"OK"}`),
			},
		},
		{
			description: "500 works as expected",
			serverAddr:  server,
			clientID:    "500",
			want: struct {
				code     int
				metadata map[string]string
				data     []byte
			}{
				code: 500,
				metadata: map[string]string{
					"Content-Length": "15",
					"Content-Type":   "text/plain; charset=utf-8",
				},
				data: []byte(`{"status":"OK"}`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			httpTransport, err := transport.NewHTTPTransport(test.clientID, test.serverAddr, nil, "testUA", time.Second)
			if err != nil {
				t.Fatalf("cannot create new transport: %v", err)
			}

			responseCode, responseMetadata, responseData, err := httpTransport.Tx("test", nil, []byte("test"))

			if test.wantError != nil {
				if !cmp.Equal(test.wantError, cmpopts.AnyError, cmpopts.EquateErrors()) {
					t.Errorf("%v != %v", err, test.wantError)
				}
			} else {
				if !cmp.Equal(responseCode, test.want.code) {
					t.Errorf("%v != %v", responseCode, test.want.code)
				}

				if !cmp.Equal(responseMetadata, test.want.metadata, cmpopts.IgnoreMapEntries(func(key string, val string) bool { return key == "Date" })) {
					t.Errorf("%v", cmp.Diff(responseMetadata, test.want.metadata))
				}

				if !cmp.Equal(responseData, test.want.data) {
					t.Errorf("%v != %v", string(responseData), string(test.want.data))
				}
			}
		})
	}
}
