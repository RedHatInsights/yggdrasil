package main

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redhatinsights/yggdrasil"
)

type Input struct {
	messageType string
	responseTo  string
	directive   string
	cancelID    string
	content     []byte
	metadata    map[string]string
	version     int
}

func TestGenerateDataMessage(t *testing.T) {
	tests := []struct {
		description string
		input       Input
		want        *yggdrasil.Data
		wantError   error
	}{
		{
			description: "data JSON content",
			input: Input{
				messageType: "data",
				directive:   "dir",
				cancelID:    "message-id",
				content:     []byte(`{"field":"value"}`),
				metadata:    map[string]string{},
				version:     1,
			},
			want: &yggdrasil.Data{
				Type:      yggdrasil.MessageTypeData,
				Version:   1,
				Directive: "dir",
				CancelID:  "message-id",
				Metadata:  map[string]string{},
				Content:   []byte(`{"field":"value"}`),
			},
		},
		{
			description: "data string content",
			input: Input{
				messageType: "data",
				directive:   "dir",
				cancelID:    "",
				content:     []byte(`"hello world"`),
				metadata:    map[string]string{},
				version:     1,
			},
			want: &yggdrasil.Data{
				Type:      yggdrasil.MessageTypeData,
				Version:   1,
				Directive: "dir",
				CancelID:  "",
				Metadata:  map[string]string{},
				Content:   []byte(`"hello world"`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := generateDataMessage(yggdrasil.MessageType(test.input.messageType), test.input.responseTo, test.input.directive, test.input.content, test.input.metadata, test.input.version, test.input.cancelID)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want, cmpopts.IgnoreFields(yggdrasil.Data{}, "MessageID", "Sent")) {
					t.Errorf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}

func TestGenerateCommandMessage(t *testing.T) {
	tests := []struct {
		description string
		input       Input
		want        *yggdrasil.Command
		wantError   error
	}{
		{
			description: "command",
			input: Input{
				messageType: string(yggdrasil.MessageTypeCommand),
				content:     []byte(`{"command":"ping","arguments":{}}`),
				version:     1,
			},
			want: &yggdrasil.Command{
				Type:    yggdrasil.MessageTypeCommand,
				Version: 1,
				Content: struct {
					Command   yggdrasil.CommandName "json:\"command\""
					Arguments map[string]string     "json:\"arguments\""
				}{
					Command:   "ping",
					Arguments: map[string]string{},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := generateCommandMessage(yggdrasil.MessageType(test.input.messageType), test.input.responseTo, test.input.version, test.input.content)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want, cmpopts.IgnoreFields(yggdrasil.Command{}, "MessageID", "Sent")) {
					t.Errorf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}

func TestGenerateControlMessage(t *testing.T) {
	tests := []struct {
		description string
		input       Input
		want        *yggdrasil.Control
		wantError   error
	}{
		{
			input: Input{
				messageType: "event",
				content:     []byte(`pong`),
				version:     1,
			},
			want: &yggdrasil.Control{
				Type:    yggdrasil.MessageTypeEvent,
				Version: 1,
				Content: json.RawMessage(`"pong"`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := generateControlMessage(yggdrasil.MessageType(test.input.messageType), test.input.responseTo, test.input.version, test.input.content)

			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want, cmpopts.IgnoreFields(yggdrasil.Control{}, "MessageID", "Sent")) {
					t.Errorf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}
