package messagejournal

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/redhatinsights/yggdrasil"
)

var placeholderWorkerMessageEntry = yggdrasil.WorkerMessage{
	MessageID:  "test-id",
	Sent:       time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
	WorkerName: "test-worker",
	ResponseTo: "test-response",
	WorkerEvent: struct {
		EventName uint              "json:\"event_name\""
		EventData map[string]string "json:\"event_data\""
	}{
		5,
		map[string]string{"test": "test-event-data"},
	},
}

func TestOpen(t *testing.T) {
	tests := []struct {
		description string
		input       string
	}{
		{
			description: "create message journal",
			input:       "file::memory:?cache=shared",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			got, err := Open(test.input)

			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Errorf("message journal is null")
			}
		})
	}
}

func TestGetEntries(t *testing.T) {
	tests := []struct {
		description string
		entries     []yggdrasil.WorkerMessage
		input       Filter
		want        []map[string]string
		wantError   error
	}{
		{
			description: "get journal entries - unfiltered empty",
			entries:     []yggdrasil.WorkerMessage{},
			input: Filter{
				Persistent: true,
				MessageID:  "",
				Worker:     "",
				Since:      "",
				Until:      "",
			},
			wantError: &errorJournal{fmt.Errorf("no journal entries found")},
		},
		{
			description: "get journal entries - unfiltered results",
			entries: []yggdrasil.WorkerMessage{
				placeholderWorkerMessageEntry,
			},
			input: Filter{
				Persistent: true,
				MessageID:  "",
				Worker:     "",
				Since:      "",
				Until:      "",
			},
			want: []map[string]string{
				0: {
					"message_id":   "test-id",
					"response_to":  "test-response",
					"sent":         "2000-01-01 00:00:00 +0000 UTC",
					"worker_event": "STOPPED",
					"worker_data":  "{\"test\":\"test-event-data\"}",
					"worker_name":  "test-worker",
				},
			},
		},
		{
			description: "get journal entries - filtered empty",
			entries: []yggdrasil.WorkerMessage{
				placeholderWorkerMessageEntry,
			},
			input: Filter{
				Persistent: true,
				MessageID:  "test-invalid-filtered-message-id",
				Worker:     "",
				Since:      "",
				Until:      "",
			},
			wantError: &errorJournal{fmt.Errorf("no journal entries found")},
		},
		{
			description: "get journal entries - filtered results",
			entries: []yggdrasil.WorkerMessage{
				placeholderWorkerMessageEntry,
				{
					MessageID:  "test-filtered-message-id",
					Sent:       time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
					WorkerName: "test-worker",
					ResponseTo: "test-response",
					WorkerEvent: struct {
						EventName uint              "json:\"event_name\""
						EventData map[string]string "json:\"event_data\""
					}{
						5,
						map[string]string{"test": "test-event-data"},
					},
				},
			},
			input: Filter{
				Persistent: true,
				MessageID:  "test-filtered-message-id",
				Worker:     "",
				Since:      "",
				Until:      "",
			},
			want: []map[string]string{
				0: {
					"message_id":   "test-filtered-message-id",
					"response_to":  "test-response",
					"sent":         "2000-01-01 00:00:00 +0000 UTC",
					"worker_event": "STOPPED",
					"worker_data":  "{\"test\":\"test-event-data\"}",
					"worker_name":  "test-worker",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// Create a message journal to test with:
			journal, err := Open("file::memory:?cache=shared")
			if err != nil {
				t.Fatal(err)
			}

			// Add entries from test input data:
			for _, entry := range test.entries {
				if err := journal.AddEntry(entry); err != nil {
					t.Fatal(err)
				}
			}

			// Get entries from the message journal:
			got, err := journal.GetEntries(test.input)
			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !cmp.Equal(got, test.want) {
					t.Errorf("%#v != %#v", got, test.want)
				}
			}
		})
	}
}

func TestAddEntry(t *testing.T) {
	tests := []struct {
		description string
		input       yggdrasil.WorkerMessage
		wantError   error
	}{
		{
			description: "create journal entry",
			input:       placeholderWorkerMessageEntry,
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			messageJournal, err := Open("file::memory:?cache=shared")
			if err != nil {
				t.Fatal(err)
			}

			err = messageJournal.AddEntry(test.input)
			if test.wantError != nil {
				if !cmp.Equal(err, test.wantError, cmpopts.EquateErrors()) {
					t.Errorf("%#v != %#v", err, test.wantError)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestBuildDynamicGetEntriesQuery(t *testing.T) {
	tests := []struct {
		description string
		input       struct {
			filter        Filter
			initializedAt time.Time
		}
		want string
	}{
		{
			description: "build dynamic get entries sql query - unfiltered",
			input: struct {
				filter        Filter
				initializedAt time.Time
			}{
				filter: Filter{
					Persistent: true,
					MessageID:  "",
					Worker:     "",
					Since:      "",
					Until:      "",
				},
				initializedAt: time.Now(),
			},
			want: "SELECT * FROM journal " +
				"ORDER BY sent",
		},
		{
			description: "build dynamic get entries sql query - filtered",
			input: struct {
				filter        Filter
				initializedAt time.Time
			}{
				filter: Filter{
					Persistent: true,
					MessageID:  "filtered-id",
					Worker:     "filtered-worker",
					Since:      "01-01-1970",
					Until:      "01-01-2000",
				},
				initializedAt: time.Now(),
			},
			want: "SELECT * FROM journal " +
				"INTERSECT SELECT * FROM journal WHERE message_id='filtered-id' " +
				"INTERSECT SELECT * FROM journal WHERE worker_name='filtered-worker' " +
				"INTERSECT SELECT * FROM journal WHERE sent>='01-01-1970' " +
				"INTERSECT SELECT * FROM journal WHERE sent<='01-01-2000' " +
				"ORDER BY sent",
		},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			query, err := buildDynamicGetEntriesQuery(test.input.filter, test.input.initializedAt)
			if err != nil {
				t.Fatal(err)
			}
			if !cmp.Equal(query, test.want) {
				t.Errorf("%#v != %#v", query, test.want)
			}
		})
	}
}
