package storage

import (
	"net/url"
	"time"

	"github.com/dgraph-io/badger"
)

// Badger storage
type Badger struct {
	Conn *badger.DB
}

// Connect will start redis connection
func (r Badger) Connect(dsn url.URL) (StorageHandler, error) {
	database := dsn.Opaque // for relative path, dsn => badger:./data
	if database == "" {
		database = dsn.Path // for absolute path, dsn => badger:/var/file.io/data
	}

	db, err := badger.Open(badger.DefaultOptions(database))
	if err != nil {
		return nil, err
	}

	return &Badger{db}, nil
}

// Set set bytes file to redis using unique id as key
func (r *Badger) Set(key string, value []byte, expired time.Duration) error {
	err := r.Conn.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), value).WithTTL(expired)
		err := txn.SetEntry(e)
		return err
	})

	return err
}

// Get get bytes from redis and write bytes as response (file)
func (r *Badger) Get(key string) ([]byte, error) {

	var valCopy []byte

	err := r.Conn.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		err = item.Value(func(val []byte) error {
			// This func with val would only be called if item.Value encounters no error.

			valCopy = append([]byte{}, val...)
			return nil
		})

		return nil
	})

	return valCopy, err
}

// Del uwu
func (r *Badger) Del(key string) {
	_ = r.Conn.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}
