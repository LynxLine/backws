package main

import (
	"context"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"vitess.io/vitess/go/pools"
)

// These constants to be used as keys.
const (
	RKeyJwtKey           = "ych:jwtkey"
	RKeyWorkers          = "ych:workers"
	RKeyStatFailed       = "ych:stat:failed:"
	RKeyStatProcessed    = "ych:stat:processed:"
	RKeyServers          = "ych:servers"
	RKeyServerQueue      = "ych:queue:"
	RKeyServerGroups     = "ych:server_groups:"
	RKeyServerSessions   = "ych:server_sessions:"
	RKeySessionsUsers    = "ych:sessions_users"
	RKeySessionsServers  = "ych:sessions_servers"
	RKeyUserSessions     = "ych:user_sessions:"
	RKeyUserExpirations  = "ych:user_expirations:"
	RKeyGroupSessions    = "ych:group_sessions:"
	RKeyGroupExpirations = "ych:group_expirations:"
	RKeyQueue            = "ych:queue:"
	RKeyNS               = "ych:"
)

// InitRedisPool initialization
func InitRedisPool() error {
	Env.RedisPool = newRedisPool(Conf.Redis, 1, 10, 5*time.Minute)
	switch val := RedisDo("GET", RKeyJwtKey).(type) {
	case []byte:
		Env.JwtKey = string(val)
	}
	if Env.JwtKey == "" {
		uuid_obj, _ := uuid.NewUUID()
		Env.JwtKey = uuid_obj.String()
		RedisDo("SET", RKeyJwtKey, Env.JwtKey)
	}
	RegisterServer()
	return nil
}

type RedisConn struct {
	redis.Conn
}

func (r *RedisConn) Close() {
	_ = r.Conn.Close()
}

func newRedisFactory(uri string) pools.Factory {
	return func() (pools.Resource, error) {
		return redisConnFromURI(uri)
	}
}

func newRedisPool(uri string, capacity int, maxCapacity int, idleTimout time.Duration) *pools.ResourcePool {
	return pools.NewResourcePool(newRedisFactory(uri), capacity, maxCapacity, idleTimout)
}

func redisConnFromURI(conn_str string) (*RedisConn, error) {
	var dial_opts []redis.DialOption
	dial_opts = append(dial_opts, redis.DialClientName("backws"))
	conn, err := redis.Dial("tcp", conn_str, dial_opts...)
	if err != nil {
		return nil, err
	}
	return &RedisConn{Conn: conn}, nil
}

// RedisDo invoke a redis command
func RedisDo(cmd string, args ...interface{}) interface{} {
	ctx := context.TODO()
	res, err := Env.RedisPool.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer Env.RedisPool.Put(res)
	c := res.(*RedisConn).Conn
	val, err := c.Do(cmd, args...)
	if err != nil {
		log.Fatal(err)
	}
	return val
}
