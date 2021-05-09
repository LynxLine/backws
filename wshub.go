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
	clients          map[*WebSocketClient]bool
	clientsBySession map[string]*WebSocketClient
	sessions         map[*WebSocketClient]string
	userSessions     map[string]map[string]bool
	sessionUID       map[string]string

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
		inmessages:       make(chan *WebSocketInpMessage),
		outmessages:      make(chan *WebSocketOutMessage),
		register:         make(chan *WebSocketClient),
		unregister:       make(chan *WebSocketClient),
		clients:          make(map[*WebSocketClient]bool),
		sessions:         make(map[*WebSocketClient]string),
		clientsBySession: make(map[string]*WebSocketClient),
		userSessions:     make(map[string]map[string]bool),
		sessionUID:       make(map[string]string),
	}
}

func (h *WebSocketsHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				if sessionID, ok := h.sessions[client]; ok {
					delete(h.clientsBySession, sessionID)
					delete(h.sessions, client)
					if uid, ok := h.sessionUID[sessionID]; ok {
						if _, ok := h.userSessions[uid]; ok {
							delete(h.userSessions[uid], sessionID)
						}
						delete(h.sessionUID, uid)
						RedisDo("HDEL", RKeyUserSessions+uid, sessionID)
						RedisDo("HDEL", RKeyUserExpirations+uid, sessionID)
					}
					RedisDo("HDEL", RKeySessionsUsers, sessionID)
					RedisDo("HDEL", RKeySessionsServers, sessionID)
				}
			}
		case msg := <-h.inmessages:
			log.Info("wsinp:", string(msg.data))

			base := WsReqT{}
			init := WsInitReqT{}

			buf := bytes.NewBuffer(msg.data)
			err := json.NewDecoder(buf).Decode(&base)
			if err != nil {
				log.Println(err)
				continue
			}

			if base.Type == "init" {
				buf = bytes.NewBuffer(msg.data)
				err = json.NewDecoder(buf).Decode(&init)
				if err != nil {
					log.Println(err)
					continue
				}

				log.Println("jwt:", init.Jwt)

				uid, exp, _ := VerifyJwt(init.Jwt, nil, Conf.JwtKey)
				log.Println("user:", uid, exp)

				sessionID := ""
				hasSession := false
				sessionID, hasSession = h.sessions[msg.client]
				if !hasSession {
					sessionID = uuid.New()
				} else {
					oldUid := h.sessionUID[sessionID]
					if _, hasUserSessions := h.userSessions[oldUid]; hasUserSessions {
						delete(h.userSessions[oldUid], sessionID)
						RedisDo("HDEL", RKeyUserSessions+oldUid, sessionID)
						RedisDo("HDEL", RKeyUserExpirations+oldUid, sessionID)
					}
					delete(h.sessionUID, oldUid)
				}
				if _, hasUserSessions := h.userSessions[uid]; !hasUserSessions {
					h.userSessions[uid] = make(map[string]bool)
				}
				h.userSessions[uid][sessionID] = true
				h.sessionUID[sessionID] = uid

				h.sessions[msg.client] = sessionID
				h.clientsBySession[sessionID] = msg.client

				RedisDo("HSET", RKeyUserSessions+uid, sessionID, Env.ServerID)
				RedisDo("HSET", RKeyUserExpirations+uid, sessionID, exp)
				RedisDo("HSET", RKeyServerSessions+Env.ServerID, sessionID, uid)
				RedisDo("HSET", RKeySessionsUsers, sessionID, uid)
				RedisDo("HSET", RKeySessionsServers, sessionID, Env.ServerID)
			}

		case msg := <-h.outmessages:
			//log.Info("wsout:", strings.TrimSpace(string(msg.data)))
			if client, ok := h.clientsBySession[msg.sessionID]; ok {
				select {
				case client.send <- msg.data:
				default:
					close(client.send)
					delete(h.clients, client)
					delete(h.sessions, client)
					delete(h.clientsBySession, msg.sessionID)
				}
			} else if msg.sessionID == "*" {
				for client := range h.clients {
					select {
					case client.send <- msg.data:
					default:
						close(client.send)
						delete(h.clients, client)
						delete(h.sessions, client)
						delete(h.clientsBySession, msg.sessionID)
					}
				}
			}
		}
	}
}
