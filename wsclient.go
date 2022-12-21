package main

import (
	"bytes"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const (
	writeWait      = 40 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 7) / 10
	maxMessageSize = 4096
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 300 * time.Second,
	CheckOrigin: func(r *http.Request) bool {
		return true // todo origin
	},
}

// WebSocketClient connection
type WebSocketClient struct {
	hub  *WebSocketsHub
	conn *websocket.Conn
	send chan []byte
}

// WebSocketInpMessage incomings
type WebSocketInpMessage struct {
	client *WebSocketClient
	data   []byte
}

// WebSocketOutMessage outcomings
type WebSocketOutMessage struct {
	sessionID string
	data      []byte
}

func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Errorf("error unexp: %v", err)
			} else {
				log.Errorf("error exp: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		msg := &WebSocketInpMessage{client: c, data: message}
		c.hub.inmessages <- msg
	}
}

func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Errorln("The hub is closing the channel. not ok from c.send")
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Errorln("NextWriter", err)
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				log.Errorln("w.Close()", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Errorln("write ping msg err:", err)
				return
			}
		}
	}
}

func serveWs(hub *WebSocketsHub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Errorln("upgrader.Upgrade, err:", err)
		return
	}
	client := &WebSocketClient{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}
