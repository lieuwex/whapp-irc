package database

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"whapp-irc/database/lockmap"
)

type Database struct {
	Folder  string
	lockMap *lockmap.LockMap
}

// MakeDatabase returns a new Database using the given folder on disk.
func MakeDatabase(folder string) (*Database, error) {
	if err := os.MkdirAll(folder, 0700); err != nil {
		return nil, err
	}

	return &Database{
		Folder:  folder,
		lockMap: lockmap.New(),
	}, nil
}

func (db *Database) getPath(id string) string {
	dir, file := filepath.Split(id)
	return filepath.Join(db.Folder, dir, file+".json")
}

// GetItem retrieves the item with the given id and, if found, stores the result
// in the value pointed to by output.
func (db *Database) GetItem(id string, output interface{}) (found bool, err error) {
	if id == "" {
		return false, ErrIDEmpty
	}

	readFile := func(id string) ([]byte, error) {
		unlock := db.lockMap.RLock(id)
		defer unlock()

		return ioutil.ReadFile(db.getPath(id))
	}

	bytes, err := readFile(id)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return true, err
	}

	return true, json.Unmarshal(bytes, output)
}

// SaveItem stores the given item with the given id in the database.
func (db *Database) SaveItem(id string, item interface{}) error {
	if id == "" {
		return ErrIDEmpty
	}

	bytes, err := json.Marshal(item)
	if err != nil {
		return err
	}

	unlock := db.lockMap.Lock(id)
	defer unlock()

	return ioutil.WriteFile(db.getPath(id), bytes, 0777)
}
