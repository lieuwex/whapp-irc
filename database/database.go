package database

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

type User struct {
	Nickname             string            `json:"nickname"`
	LocalStorage         map[string]string `json:"localStorage"`
	LastReceivedReceipts map[string]int64  `json:"lastReceivedReceipts"`
}

type Database struct {
	Folder string
	lock   sync.Mutex
}

func MakeDatabase(folder string) (*Database, error) {
	if err := os.MkdirAll(folder, 0700); err != nil {
		return nil, err
	}

	return &Database{
		Folder: folder,
	}, nil
}

func (db *Database) GetItem(id string) (item interface{}, found bool, err error) {
	if id == "" {
		return nil, false, ErrIDEmpty
	}

	dir, file := filepath.Split(id)

	db.lock.Lock()
	defer db.lock.Unlock()

	bytes, err := ioutil.ReadFile(filepath.Join(db.Folder, dir, file+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}

		return nil, false, err
	}

	var res interface{}

	if err := json.Unmarshal(bytes, &res); err != nil {
		return nil, false, err
	}

	return res, true, nil
}

func (db *Database) SaveItem(id string, item interface{}) error {
	if id == "" {
		return ErrIDEmpty
	}

	dir, file := filepath.Split(id)

	db.lock.Lock()
	defer db.lock.Unlock()

	bytes, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(db.Folder, dir, file+".json"), bytes, 0777)
}
