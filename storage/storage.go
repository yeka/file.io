package storage

import (
	"errors"
	"net/url"
	"time"
)

type ConnectFn func(dsn url.URL) (StorageHandler, error)

type StorageHandler interface {
	Set(key string, value []byte, expired time.Duration) error
	Get(key string) ([]byte, error)
	Del(key string)
}

var knownStorage = map[string]ConnectFn{
	"redis":  Redis{}.Connect,
	"badger": Badger{}.Connect,
}

// Connect to specified `database`, return error if given `database` are invalid or unknown or
func Connect(database string) (StorageHandler, error) {
	dsn, err := url.Parse(database)
	if err != nil {
		return nil, errors.New("unable to parse database URI: " + err.Error())
	}

	connect, ok := knownStorage[dsn.Scheme]
	if !ok {
		return nil, errors.New("unknown storage: " + dsn.Scheme)
	}

	return connect(*dsn)
}
