package main

import (
	"bytes"
	"encoding/json"

	uuid "github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// WebSocketsHub maintains the set of active clients
type WebSocketsHub struct {
	// Registered clients.
	clients        map[*WebSocketClient]bool
	session2client map[string]*WebSocketClient
	client2session map[*WebSocketClient]string
	userSessions   map[string]map[string]bool
	sessionUID     map[string]string
	sessionGID     map[string][]string

	// Inbound messages from the clients.
	inmessages chan *WebSocketInpMessage

	// outbound messages from the clients.
	outmessages chan *WebSocketOutMessage

	// Register requests from the clients.
	register chan *WebSocketClient

	// Unregister requests from clients.
	unregister chan *WebSocketClient
}

// WsReqT as base
type WsReqT struct {
	Type string `json:"type"`
}

// WsInitReqT for registering
type WsInitReqT struct {
	Type string `json:"type"`
	Jwt  string `json:"jwt"`
}

func newHub() *WebSocketsHub {
	return &WebSocketsHub{
		inmessages:     make(chan *WebSocketInpMessage),
		outmessages:    make(chan *WebSocketOutMessage),
		register:       make(chan *WebSocketClient),
		unregister:     make(chan *WebSocketClient),
		clients:        make(map[*WebSocketClient]bool),
		client2session: make(map[*WebSocketClient]string),
		session2client: make(map[string]*WebSocketClient),
		sessionUID:     make(map[string]string),
		sessionGID:     make(map[string][]string),
	}
}

func (h *WebSocketsHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, has := h.clients[client]; has {
				delete(h.clients, client)
				close(client.send)
				//log.Println("ses unreg close")
				if session_id, has := h.client2session[client]; has {
					delete(h.session2client, session_id)
					delete(h.client2session, client)
					if uidhex, has := h.sessionUID[session_id]; has {
						delete(h.sessionUID, session_id)
						RedisDo("HDEL", RKeyUserSessions+uidhex, session_id)
						RedisDo("HDEL", RKeyUserExpirations+uidhex, session_id)
					}
					if grps, has := h.sessionGID[session_id]; has {
						delete(h.sessionGID, session_id)
						for _, grphex := range grps {
							RedisDo("HDEL", RKeyGroupSessions+grphex, session_id)
							RedisDo("HDEL", RKeyGroupExpirations+grphex, session_id)
						}
					}
					RedisDo("HDEL", RKeySessionsUsers, session_id)
					RedisDo("HDEL", RKeySessionsServers, session_id)
				}
			}
		case msg := <-h.inmessages:
			//log.Info("wsinp:", string(msg.data))

			base := WsReqT{}
			init := WsInitReqT{}

			buf := bytes.NewBuffer(msg.data)
			err := json.NewDecoder(buf).Decode(&base)
			if err != nil {
				log.Errorln("inmessage json decode:", err)
				continue
			}

			if base.Type == "init" {
				buf = bytes.NewBuffer(msg.data)
				err = json.NewDecoder(buf).Decode(&init)
				if err != nil {
					log.Errorln("inmessage init json decode:", err)
					continue
				}

				//log.Println("jwt:", init.Jwt)

				uidhex, grps, exp, _ := VerifyJwt(init.Jwt, nil, Env.JwtKey)
				//log.Println("user:", uid, exp)

				session_id := ""
				has_session := false
				session_id, has_session = h.client2session[msg.client]
				if !has_session {
					session_id = uuid.New()
				} else {
					if old_uidhex, has := h.sessionUID[session_id]; has {
						RedisDo("HDEL", RKeyUserSessions+old_uidhex, session_id)
						RedisDo("HDEL", RKeyUserExpirations+old_uidhex, session_id)
						delete(h.sessionUID, session_id)
					}
					if old_grps, has := h.sessionGID[session_id]; has {
						for _, old_grphex := range old_grps {
							RedisDo("HDEL", RKeyGroupSessions+old_grphex, session_id)
							RedisDo("HDEL", RKeyGroupExpirations+old_grphex, session_id)
						}
						delete(h.sessionGID, session_id)
					}
				}
				h.sessionUID[session_id] = uidhex
				h.sessionGID[session_id] = grps

				h.client2session[msg.client] = session_id
				h.session2client[session_id] = msg.client

				RedisDo("HSET", RKeyUserSessions+uidhex, session_id, Env.ServerID)
				RedisDo("HSET", RKeyUserExpirations+uidhex, session_id, exp)
				for _, grphex := range grps {
					RedisDo("HSET", RKeyGroupSessions+grphex, session_id, Env.ServerID)
					RedisDo("HSET", RKeyGroupExpirations+grphex, session_id, exp)
				}
				RedisDo("HSET", RKeyServerGroups+Env.ServerID, session_id, grps)
				RedisDo("HSET", RKeyServerSessions+Env.ServerID, session_id, uidhex)
				RedisDo("HSET", RKeySessionsUsers, session_id, uidhex)
				RedisDo("HSET", RKeySessionsServers, session_id, Env.ServerID)
			}

		case msg := <-h.outmessages:
			//log.Info("wsout:", strings.TrimSpace(string(msg.data)))
			if client, ok := h.session2client[msg.sessionID]; ok {
				select {
				case client.send <- msg.data:
				default:
					close(client.send)
					//log.Println("ses def close")
					delete(h.clients, client)
					delete(h.client2session, client)
					delete(h.session2client, msg.sessionID)
				}
			} else if msg.sessionID == "*" {
				for client := range h.clients {
					select {
					case client.send <- msg.data:
					default:
						close(client.send)
						//log.Println("all def close")
						delete(h.clients, client)
						delete(h.client2session, client)
						delete(h.session2client, msg.sessionID)
					}
				}
			}
		}
	}
}
