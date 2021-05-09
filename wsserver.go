package main

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// WebSocketRespT http handler
type WebSocketRespT struct {
}

func (h *WebSocketRespT) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Info("ws:", r.Method, r.URL)
	serveWs(Env.WsHub, w, r)
}
