package internal

import (
	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
	"os"
	"path/filepath"
	"time"
)

func PublishConnectionStatus(t Transport, dispatchers map[string]map[string]string) {
	facts, err := yggdrasil.GetCanonicalFacts()
	if err != nil {
		log.Errorf("cannot get canonical facts: %v", err)
		return
	}

	tagsFilePath := filepath.Join(yggdrasil.SysconfDir, yggdrasil.LongName, "tags.toml")

	var tagMap map[string]string
	if _, err := os.Stat(tagsFilePath); !os.IsNotExist(err) {
		var err error
		tagMap, err = readTagsFile(tagsFilePath)
		if err != nil {
			log.Errorf("cannot load tags: %v", err)
		}
	}

	msg := yggdrasil.ConnectionStatus{
		Type:      yggdrasil.MessageTypeConnectionStatus,
		MessageID: uuid.New().String(),
		Version:   1,
		Sent:      time.Now(),
		Content: struct {
			CanonicalFacts yggdrasil.CanonicalFacts     "json:\"canonical_facts\""
			Dispatchers    map[string]map[string]string "json:\"dispatchers\""
			State          yggdrasil.ConnectionState    "json:\"state\""
			Tags           map[string]string            "json:\"tags,omitempty\""
		}{
			CanonicalFacts: *facts,
			Dispatchers:    dispatchers,
			State:          yggdrasil.ConnectionStateOnline,
			Tags:           tagMap,
		},
	}

	err = t.SendControl(msg)
	if err != nil {
		log.Error(err)
	}
	log.Debugf("published message %v to control topic", msg.MessageID)
	log.Tracef("message: %+v", msg)
}

func PublishReceivedData(transport Transport, c <-chan yggdrasil.Data) {
	for d := range c {
		err := transport.SendData(d)
		if err != nil {
			log.Error(err)
		}
	}
}
