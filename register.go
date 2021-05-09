package main

import (
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// RegisterServer register this server in redis
func RegisterServer() {
	RedisDo("HSET", RKeyServers, Env.ServerID, time.Now().UTC().UnixNano())
	go keepServerStampUpdated()
	go cleanupOutdatedServers()
	log.Info("Server " + Env.ServerID + " registered")
}

func keepServerStampUpdated() {
	for {
		time.Sleep(1 * time.Second)
		RedisDo("HSET", RKeyServers, Env.ServerID, time.Now().UTC().UnixNano())
	}
}

func cleanupOutdatedServers() {
	for {
		time.Sleep(5 * time.Second)
		switch vals := RedisDo("HGETALL", RKeyServers).(type) {
		case []interface{}:
			currentKey := ""
			serversTimes := make(map[string]int64)
			for idx, vali := range vals {
				switch val := vali.(type) {
				case []byte:
					if idx%2 == 0 {
						currentKey = string(val)
					} else {
						t, err := strconv.ParseInt(string(val), 10, 64)
						if err == nil {
							serversTimes[currentKey] = t
						}
					}
				}
			}
			for serverID, t := range serversTimes {
				if t < time.Now().Add(-60*time.Second).UTC().UnixNano() {
					RedisDo("HDEL", RKeyServers, serverID)
					go cleanupOutdatedSessions(serverID)
					go cleanupOutdatedWorkers(serverID)
				}
			}
		}
	}
}

func cleanupOutdatedSessions(serverID string) {
	switch vals := RedisDo("HGETALL", RKeyServerSessions+serverID).(type) {
	case []interface{}:
		sessionID := ""
		mapSessionUID := make(map[string]string)
		for idx, vali := range vals {
			switch val := vali.(type) {
			case []byte:
				if idx%2 == 0 {
					sessionID = string(val)
				} else {
					mapSessionUID[sessionID] = string(val)
				}
			}
		}
		for sessionID, uidhex := range mapSessionUID {
			RedisDo("HDEL", RKeySessionsUsers, sessionID)
			RedisDo("HDEL", RKeySessionsServers, sessionID)
			RedisDo("HDEL", RKeyUserSessions+uidhex, sessionID)
			RedisDo("HDEL", RKeyUserExpirations+uidhex, sessionID)
		}
	}
	RedisDo("DEL", RKeyServerSessions+serverID)
	RedisDo("DEL", RKeyServerQueue+serverID)
}

func cleanupOutdatedWorkers(serverID string) {
	switch vals := RedisDo("SMEMBERS", RKeyWorkers).(type) {
	case []interface{}:
		workerID := ""
		workersIDs := make([]string, 0)
		log.Debug(vals)
		for _, vali := range vals {
			switch val := vali.(type) {
			case []byte:

				workerID = string(val)
				if strings.HasSuffix(workerID, serverID) {
					workersIDs = append(workersIDs, workerID)
				}
			}
		}
		for _, workerID := range workersIDs {
			RedisDo("DEL", RKeyStatFailed+workerID)
			RedisDo("DEL", RKeyStatProcessed+workerID)
			RedisDo("SREM", RKeyWorkers, workerID)
		}
	}
}
