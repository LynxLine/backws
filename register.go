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
			current_key := ""
			servers_times := make(map[string]int64)
			for idx, vali := range vals {
				switch val := vali.(type) {
				case []byte:
					if idx%2 == 0 {
						current_key = string(val)
					} else {
						t, err := strconv.ParseInt(string(val), 10, 64)
						if err == nil {
							servers_times[current_key] = t
						}
					}
				}
			}
			for server_id, t := range servers_times {
				if t < time.Now().Add(-60*time.Second).UTC().UnixNano() {
					RedisDo("HDEL", RKeyServers, server_id)
					go cleanupOutdatedSessions(server_id)
					go cleanupOutdatedWorkers(server_id)
				}
			}
		}
	}
}

func cleanupOutdatedSessions(server_id string) {
	switch vals := RedisDo("HGETALL", RKeyServerSessions+server_id).(type) {
	case []interface{}:
		session_id := ""
		session_to_uid := make(map[string]string)
		for idx, vali := range vals {
			switch val := vali.(type) {
			case []byte:
				if idx%2 == 0 {
					session_id = string(val)
				} else {
					session_to_uid[session_id] = string(val)
				}
			}
		}
		for session_id, uidhex := range session_to_uid {
			RedisDo("HDEL", RKeySessionsUsers, session_id)
			RedisDo("HDEL", RKeySessionsServers, session_id)
			RedisDo("HDEL", RKeyUserSessions+uidhex, session_id)
			RedisDo("HDEL", RKeyUserExpirations+uidhex, session_id)
		}
	}
	switch vals := RedisDo("HGETALL", RKeyServerGroups+server_id).(type) {
	case []interface{}:
		session_id := ""
		session_to_grps := make(map[string][]string)
		for idx, vali := range vals {
			switch val := vali.(type) {
			case []byte:
				if idx%2 == 0 {
					session_id = string(val)
				} else {
					text := string(val)
					grps := []string{}
					if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
						grps = strings.Split(text[1:len(text)-1], " ")
					}
					session_to_grps[session_id] = grps
				}
			}
		}
		for session_id, grps := range session_to_grps {
			for _, grphex := range grps {
				RedisDo("HDEL", RKeyGroupSessions+grphex, session_id)
				RedisDo("HDEL", RKeyGroupExpirations+grphex, session_id)
			}
		}
	}
	RedisDo("DEL", RKeyServerSessions+server_id)
	RedisDo("DEL", RKeyServerGroups+server_id)
	RedisDo("DEL", RKeyServerQueue+server_id)
}

func cleanupOutdatedWorkers(server_id string) {
	switch vals := RedisDo("SMEMBERS", RKeyWorkers).(type) {
	case []interface{}:
		worker_id := ""
		workers_ids := make([]string, 0)
		log.Debug(vals)
		for _, vali := range vals {
			switch val := vali.(type) {
			case []byte:
				worker_id = string(val)
				if strings.HasSuffix(worker_id, server_id) {
					workers_ids = append(workers_ids, worker_id)
				}
			}
		}
		for _, worker_id := range workers_ids {
			RedisDo("DEL", RKeyStatFailed+worker_id)
			RedisDo("DEL", RKeyStatProcessed+worker_id)
			RedisDo("SREM", RKeyWorkers, worker_id)
		}
	}
}
