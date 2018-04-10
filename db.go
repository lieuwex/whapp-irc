package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

type User struct {
	Nickname     string            `json:"nickname"`
	LocalStorage map[string]string `json:"localStorage"`
}

type Database struct {
	Folder string
	lock   sync.Mutex
}

func MakeDatabase(folder string) (*Database, error) {
	return &Database{
		Folder: folder,
	}, nil
}

func (db *Database) GetItem(id string) (item interface{}, found bool, err error) {
	if id == "" {
		return nil, false, errors.New("id can't be empty")
	}

	dir, file := path.Split(id)

	db.lock.Lock()
	defer db.lock.Unlock()

	bytes, err := ioutil.ReadFile(path.Join(db.Folder, dir, file+".json"))
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
		return errors.New("id can't be empty")
	}

	dir, file := path.Split(id)

	db.lock.Lock()
	defer db.lock.Unlock()

	bytes, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path.Join(db.Folder, dir, file+".json"), bytes, 0777)
}
