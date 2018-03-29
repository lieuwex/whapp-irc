package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type File struct {
	MessageID string
	Path      string
	URL       string
}

type FileServer struct {
	Host      string
	Port      string
	Directory string

	IDToPath map[string]*File
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

		IDToPath: make(map[string]*File),
	}, nil
}

func (fs *FileServer) Start() {
	http.Handle("/", http.FileServer(http.Dir(fs.Directory)))

	http.ListenAndServe(fs.Host+":"+fs.Port, nil)
}

func (fs *FileServer) MakeFile(messageID, filename string) *File {
	if filename == "" {
		return nil
	}

	url := fmt.Sprintf("http://localhost:%s/%s", fs.Port, filename)
	file := fmt.Sprintf("./%s/%s", fs.Directory, filename)

	return &File{
		MessageID: messageID,
		URL:       url,
		Path:      file,
	}
}

func (fs *FileServer) AddBlob(messageID string, filename string, bytes []byte) (*File, error) {
	f := fs.MakeFile(messageID, filename)
	if f == nil {
		return nil, fmt.Errorf("filename can't be empty")
	}

	err := ioutil.WriteFile(f.Path, bytes, 0644)
	if err != nil {
		return nil, err
	}

	fs.IDToPath[messageID] = f
	return f, nil
}

func (fs *FileServer) Stop() {
	// TODO
}
