package transport

import (
	"encoding/json"
	"testing"

	"github.com/redhatinsights/yggdrasil"
	"github.com/redhatinsights/yggdrasil/internal/constants"
)

// TestNewLastWillPayload_structure verifies that newLastWillPayload returns
// valid JSON with the expected connection-status-offline structure (CCT-1572).
func TestNewLastWillPayload_structure(t *testing.T) {
	payload, err := newLastWillPayload()
	if err != nil {
		t.Fatalf("newLastWillPayload() error: %v", err)
	}

	var msg yggdrasil.ConnectionStatus
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("payload is not valid ConnectionStatus JSON: %v", err)
	}

	if msg.Type != yggdrasil.MessageTypeConnectionStatus {
		t.Errorf("type = %q, want %q", msg.Type, yggdrasil.MessageTypeConnectionStatus)
	}
	if msg.MessageID == "" {
		t.Error("message_id must be non-empty")
	}
	if msg.Version != 1 {
		t.Errorf("version = %d, want 1", msg.Version)
	}
	if msg.Content.State != yggdrasil.ConnectionStateOffline {
		t.Errorf("content.state = %q, want %q", msg.Content.State, yggdrasil.ConnectionStateOffline)
	}
	// client_version must match constants.Version
	if msg.Content.ClientVersion != constants.Version {
		t.Errorf(
			"content.client_version = %q, want %q",
			msg.Content.ClientVersion,
			constants.Version,
		)
	}
}

// TestNewLastWillPayload_distinctPerCall verifies that each call to
// newLastWillPayload produces a distinct message_id (UUID) and a non-zero
// sent time. Uniqueness is asserted on message_id; timestamp is only checked
// for presence (CCT-1572).
func TestNewLastWillPayload_distinctPerCall(t *testing.T) {
	const numCalls = 10
	messageIDs := make(map[string]struct{}, numCalls)

	for i := range numCalls {
		payload, err := newLastWillPayload()
		if err != nil {
			t.Fatalf("newLastWillPayload() error: %v", err)
		}

		var msg yggdrasil.ConnectionStatus
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("payload is not valid JSON: %v", err)
		}

		if _, seen := messageIDs[msg.MessageID]; seen {
			t.Errorf("duplicate message_id %q on call %d", msg.MessageID, i+1)
		}
		messageIDs[msg.MessageID] = struct{}{}

		if msg.Sent.IsZero() {
			t.Errorf("sent timestamp must be non-zero on call %d", i+1)
		}
	}
}
