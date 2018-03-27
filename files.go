package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type FileServer struct {
	Host      string
	Port      string
	Directory string
}

func MakeFileServer(host, port, dir string) (*FileServer, error) {
	err := os.Mkdir("./"+dir, 0700)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}

	return &FileServer{
		Host:      host,
		Port:      port,
		Directory: dir,
	}, nil
}

func (fs *FileServer) Start() {
	http.Handle("/", http.FileServer(http.Dir(fs.Directory)))

	http.ListenAndServe(fs.Host+":"+fs.Port, nil)
}

func (fs *FileServer) AddBlob(filename string, bytes []byte) (string, error) {
	path := "./" + fs.Directory + "/" + filename

	err := ioutil.WriteFile(path, bytes, 0644)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://localhost:%s/%s", fs.Port, filename), nil
}

func (fs *FileServer) Stop() {
	// TODO
}
