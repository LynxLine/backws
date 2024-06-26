package main

import (
	"net/http"
	"os"
	"strconv"

	uuid "github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v2"
	"vitess.io/vitess/go/pools"
)

// ConfigT as db conf, etc
type ConfigT struct {
	Port  int    `yaml:"port"`
	Redis string `yaml:"redis"`
}

// EnvT as db, etc
type EnvT struct {
	RedisPool *pools.ResourcePool
	WsHub     *WebSocketsHub
	JwtKey    string
	ServerID  string
}

// Conf var as storage
var Conf ConfigT

// Env var as storage
var Env EnvT

func startHTTP() {
	http.Handle("/", &WebSocketRespT{})

	addr := ":" + strconv.Itoa(Conf.Port)
	log.Warnln("about to start server addr:", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	buf, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatalln("Cannot read config.yml err:", err)
		return
	}

	err = yaml.Unmarshal(buf, &Conf)
	if err != nil {
		log.Fatalln("config err:", err)
		return
	}

	Env.ServerID = uuid.New()
	InitRedisPool()

	Env.WsHub = newHub()
	go Env.WsHub.run()

	initQueueClient(Env.WsHub)

	go startHTTP()
	startQueueProcessing()
}
