package yggdrasil

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestHandleDataMessage(t *testing.T) {
	tests := []struct {
		description string

		input     []byte
		want      interface{}
		wantError error
	}{
		{
			input: []byte(`{"type":"data","message_id": "a6a7d866-7de0-409a-84e0-3c56c4171bb7","version": 1,"sent": "2021-01-12T15:30:08+00:00","directive": "echo","content": "Hello world!"}`),
			want: Data{
				Type:      MessageTypeData,
				MessageID: "a6a7d866-7de0-409a-84e0-3c56c4171bb7",
				Version:   1,
				Sent:      time.Date(2021, time.January, 12, 15, 30, 8, 0, time.UTC),
				Directive: "echo",
				Content:   json.RawMessage("Hello world!"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			db, err := NewDatastore()
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMessageRouter(db, []string{}, "", "")
			if err != nil {
				t.Fatal(err)
			}
			go func(c <-chan interface{}) {
				got := <-c

				if test.wantError != nil {
					if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
						t.Errorf("%#v != %#v", err, test.wantError)
					}
				} else {
					if err != nil {
						t.Error(err)
					}
					if !cmp.Equal(got, test.want) {
						t.Errorf("%#v != %#v", got, test.want)
					}
				}
			}(m.Connect(SignalDataRecv))

			m.handleDataMessage(test.input)
		})
	}
}
