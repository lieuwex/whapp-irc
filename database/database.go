package database

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"whapp-irc/database/lockmap"
)

type User struct {
	LocalStorage         map[string]string `json:"localStorage"`
	LastReceivedReceipts map[string]int64  `json:"lastReceivedReceipts"`
}

type Database struct {
	Folder  string
	lockMap *lockmap.LockMap
}

func MakeDatabase(folder string) (*Database, error) {
	if err := os.MkdirAll(folder, 0700); err != nil {
		return nil, err
	}

	return &Database{
		Folder:  folder,
		lockMap: lockmap.New(),
	}, nil
}

func (db *Database) GetItem(id string) (item interface{}, found bool, err error) {
	if id == "" {
		return nil, false, ErrIDEmpty
	}

	readFile := func(id string) ([]byte, error) {
		dir, file := filepath.Split(id)
		path := filepath.Join(db.Folder, dir, file+".json")

		unlock := db.lockMap.RLock(id)
		defer unlock()

		return ioutil.ReadFile(path)
	}

	bytes, err := readFile(id)
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

	bytes, err := json.Marshal(item)
	if err != nil {
		return err
	}

	unlock := db.lockMap.Lock(id)
	defer unlock()

	path := filepath.Join(db.Folder, dir, file+".json")
	return ioutil.WriteFile(path, bytes, 0777)
}
