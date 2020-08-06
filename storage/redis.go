package storage

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v7"
)

// Redis storage
type Redis struct {
	Conn *redis.Client
}

// Connect will start redis connection
func (r Redis) Connect(dsn url.URL) (StorageHandler, error) {
	db, err := strconv.Atoi(strings.TrimLeft(dsn.Path, "/"))
	if err != nil {
		return nil, errors.New("redis db must be integer: " + strings.TrimLeft(dsn.Path, "/"))
	}

	pass, _ := dsn.User.Password()
	conn := redis.NewClient(&redis.Options{
		Addr:     dsn.Host,
		Password: pass, // no password set
		DB:       db,   // use default DB
	})

	return &Redis{conn}, nil
}

// Set set bytes file to redis using unique id as key
func (r *Redis) Set(key string, value []byte, expired time.Duration) error {
	return r.Conn.Set(key, value, expired).Err()
}

// Get get bytes from redis and write bytes as response (file)
func (r *Redis) Get(key string) ([]byte, error) {
	d := r.Conn.Get(key)
	return d.Bytes()
}

// Del uwu
func (r *Redis) Del(key string) {
	r.Conn.Del(key)
}
