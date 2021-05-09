package main

import (
	"context"
	"time"

	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"vitess.io/vitess/go/pools"
)

// These constants to be used as keys.
const (
	RKeyWorkers         = "ych:workers"
	RKeyStatFailed      = "ych:stat:failed:"
	RKeyStatProcessed   = "ych:stat:processed:"
	RKeyServers         = "ych:servers"
	RKeyServerQueue     = "ych:queue:"
	RKeyServerSessions  = "ych:server_sessions:"
	RKeySessionsUsers   = "ych:sessions_users"
	RKeySessionsServers = "ych:sessions_servers"
	RKeyUserSessions    = "ych:user_sessions:"
	RKeyUserExpirations = "ych:user_expirations:"
	RKeyQueue           = "ych:queue:"
	RKeyNS              = "ych:"
)

// // RedisResourceConnT to have as handler
// type RedisResourceConnT struct {
// 	Connection redis.Conn
// }

// // Close to stop internals
// func (r RedisResourceConnT) Close() {
// 	r.Connection.Close()
// }

// InitRedisPool initialization
func InitRedisPool() error {
	Env.RedisPool = newRedisPool(Conf.Redis, 1, 10, 5*time.Minute)
	// Env.RedisPool = pools.NewResourcePool(func() (pools.Resource, error) {
	// 	connString := Conf.Redis
	// 	var dialOptions []redis.DialOption
	// 	dialOptions = append(dialOptions, redis.DialClientName("backws"))
	// 	c, err := redis.Dial("tcp", connString, dialOptions...)
	// 	if err != nil {
	// 		log.Print(err)
	// 	}
	// 	return RedisResourceConnT{Connection: c}, err
	// }, 1, 10 /*max connections*/, 5*time.Minute, 0)

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

func redisConnFromURI(connString string) (*RedisConn, error) {

	var dialOptions []redis.DialOption
	dialOptions = append(dialOptions, redis.DialClientName("backws"))
	conn, err := redis.Dial("tcp", connString, dialOptions...)
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
