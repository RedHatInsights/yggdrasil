package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"github.com/redhatinsights/yggdrasil"
)

func benchmark(broker string, inboundTopic string, outboundTopic string, count int, messageDelay time.Duration) {
	var mu sync.RWMutex
	pendingMessages := map[string]*yggdrasil.Data{}

	var wg sync.WaitGroup

	// start goroutine that subscribes to an MQTT topic and receives
	// messages published there. For each message received, it
	// calculates the time taken to receive the message from when it
	// was sent.
	go func() {
		opts := mqtt.NewClientOptions()
		opts.AddBroker(broker)
		opts.SetClientID(uuid.New().String())
		opts.SetCleanSession(true)
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			client.Subscribe(outboundTopic, 1, func(c mqtt.Client, m mqtt.Message) {
				var msg yggdrasil.Data
				if err := json.Unmarshal(m.Payload(), &msg); err != nil {
					log.Errorf("cannot unmarshal payload: %v", err)
					return
				}
				mu.RLock()
				sentMessage, has := pendingMessages[msg.ResponseTo]
				mu.RUnlock()
				if has {
					fmt.Printf("%v: %v\n", sentMessage.MessageID, msg.Sent.Sub(sentMessage.Sent))
				}
				mu.Lock()
				delete(pendingMessages, msg.ResponseTo)
				mu.Unlock()
				wg.Done()
			})
		})
		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Fatalf("cannot connect to broker: %v", token.Error())
		}
	}()

	go func() {
		opts := mqtt.NewClientOptions()
		opts.AddBroker(broker)
		opts.SetClientID(uuid.New().String())
		opts.SetCleanSession(true)
		opts.SetOnConnectHandler(func(client mqtt.Client) {
			for i := 0; i < count; i++ {
				wg.Add(1)
				msg, err := generateDataMessage(yggdrasil.MessageTypeData, "", "echo", []byte(`{}`), nil, 1)
				if err != nil {
					log.Errorf("cannot generate data message: %v", err)
					continue
				}

				data, err := json.Marshal(msg)
				if err != nil {
					log.Errorf("cannot marshal json: %v", err)
					continue
				}

				mu.Lock()
				pendingMessages[msg.MessageID] = msg
				mu.Unlock()

				if token := client.Publish(inboundTopic, 1, false, data); token.Wait() && token.Error() != nil {
					log.Errorf("cannot publish message: %v", token.Error())
					continue
				}
				time.Sleep(messageDelay)
			}
		})
		client := mqtt.NewClient(opts)
		if token := client.Connect(); token.Wait() && token.Error() != nil {
			log.Fatalf("cannot connect to broker: %v", token.Error())
		}
	}()

	wg.Wait()
}
