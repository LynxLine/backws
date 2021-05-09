package main

import (
	"encoding/base64"

	"github.com/benmanns/goworker"
	log "github.com/sirupsen/logrus"
)

func initQueueClient(webSocketsHub *WebSocketsHub) {
	settings := goworker.WorkerSettings{
		URI:            "redis://" + Conf.Redis,
		Connections:    10,
		Queues:         []string{Env.ServerID},
		UseNumber:      true,
		ExitOnComplete: false,
		Concurrency:    5,
		Namespace:      RKeyNS,
		IntervalFloat:  0.2,
	}
	goworker.SetSettings(settings)
	goworker.Register("Events", func(queue string, args ...interface{}) error {
		switch sessionID := args[0].(type) {
		case string:
			switch messageB64 := args[1].(type) {
			case string:
				data, err := base64.StdEncoding.DecodeString(messageB64)
				if err != nil {
					log.Fatal(err)
					return err
				}
				msg := &WebSocketOutMessage{
					sessionID,
					data,
				}
				webSocketsHub.outmessages <- msg
			}
		}
		return nil
	})
}

func startQueueProcessing() {
	log.Info("Start queue processing")
	if err := goworker.Work(); err != nil {
		log.Fatal(err)
	}
	goworker.Close()
}
