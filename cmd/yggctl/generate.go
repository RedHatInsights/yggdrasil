package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
)

func generateDataMessage(messageType yggdrasil.MessageType, responseTo string, directive string, bytes []byte, metadata map[string]string, version int) (*yggdrasil.Data, error) {
	var data interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, fmt.Errorf("cannot unmarshal content: %v", err)
	}

	content, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal data: %v", err)
	}

	msg := yggdrasil.Data{
		Type:       messageType,
		MessageID:  uuid.New().String(),
		ResponseTo: responseTo,
		Version:    version,
		Sent:       time.Now(),
		Directive:  directive,
		Metadata:   metadata,
		Content:    content,
	}

	return &msg, nil
}

// generateControlMessage creates a control message of the appropriate type by
// switching on the value of messageType.
func generateControlMessage(messageType yggdrasil.MessageType, responseTo string, version int, bytes []byte) (*yggdrasil.Control, error) {
	switch messageType {
	case yggdrasil.MessageTypeCommand:
		msg, err := generateCommandMessage(messageType, responseTo, version, bytes)
		if err != nil {
			return nil, fmt.Errorf("cannot generate command message: %v", err)
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal command message: %v", err)
		}
		var ctrl yggdrasil.Control
		if err := json.Unmarshal(data, &ctrl); err != nil {
			return nil, fmt.Errorf("cannot unmarshal control message: %v", err)
		}
		return &ctrl, nil
	case yggdrasil.MessageTypeEvent:
		msg, err := generateEventMessage(messageType, responseTo, version, bytes)
		if err != nil {
			return nil, fmt.Errorf("cannot generate event message: %v", err)
		}
		data, err := json.Marshal(msg)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal command message: %v", err)
		}
		var ctrl yggdrasil.Control
		if err := json.Unmarshal(data, &ctrl); err != nil {
			return nil, fmt.Errorf("cannot unmarshal control message: %v", err)
		}
		return &ctrl, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %v", messageType)
	}
}

// generateCommandMessage unmarshals bytes into a command message.
func generateCommandMessage(messageType yggdrasil.MessageType, responseTo string, version int, bytes []byte) (*yggdrasil.Command, error) {
	msg := yggdrasil.Command{
		Type:       messageType,
		MessageID:  uuid.New().String(),
		ResponseTo: responseTo,
		Version:    version,
		Sent:       time.Now(),
	}

	if err := json.Unmarshal(bytes, &msg.Content); err != nil {
		return nil, fmt.Errorf("cannot unmarshal content: %v", err)
	}

	return &msg, nil
}

func generateEventMessage(messageType yggdrasil.MessageType, responseTo string, version int, bytes []byte) (*yggdrasil.Event, error) {
	msg := yggdrasil.Event{
		Type:       messageType,
		MessageID:  uuid.New().String(),
		ResponseTo: responseTo,
		Version:    version,
		Sent:       time.Now(),
		Content:    string(bytes),
	}

	return &msg, nil
}
