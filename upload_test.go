package yggdrasil

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUpload(t *testing.T) {
	tests := []struct {
		desc  string
		input struct {
			collector string
			metadata  *CanonicalFacts
		}
		want string
	}{
		{
			desc: "valid",
			input: struct {
				collector string
				metadata  *CanonicalFacts
			}{
				collector: "foo",
				metadata:  &CanonicalFacts{},
			},
			want: "dc99c9e6-c708-40a1-987a-e798c39cb3dc",
		},
		{
			desc: "implied collector",
			want: "dc99c9e6-c708-40a1-987a-e798c39cb3dc",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			// Set up a test HTTP server with a static handler function
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				w.Write([]byte(`{"request_id":"dc99c9e6-c708-40a1-987a-e798c39cb3dc"}`))
			}))
			defer server.Close()

			// Set up a basic client, configured to connect to the test server
			client, err := NewHTTPClientBasicAuth(server.URL, "", "", "")
			if err != nil {
				t.Fatal(err)
			}

			// Create an empty file to "upload"
			file, err := ioutil.TempFile("", "")
			if err != nil {
				t.Fatal(err)
			}
			file.Close()
			defer os.Remove(file.Name())

			got, err := Upload(client, file.Name(), test.input.collector, test.input.metadata)

			if err != nil {
				t.Fatalf("Upload(%+v) returned %#v", test.input, err)
			}
			if !cmp.Equal(got, test.want) {
				t.Errorf("Upload(%+v) = %#v, want %#v", test.input, got, test.want)
			}
		})
	}
}
