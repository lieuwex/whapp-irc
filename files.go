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

	httpServer *http.Server
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

func (fs *FileServer) Start() error {
	fs.httpServer = &http.Server{
		Addr:    ":" + fs.Port,
		Handler: http.FileServer(http.Dir(fs.Directory)),
	}

	return fs.httpServer.ListenAndServe()
}

func (fs *FileServer) Stop() error {
	err := fs.httpServer.Close()
	if err != nil {
		return err
	}

	fs.httpServer = nil
	return nil
}

func (fs *FileServer) MakeFile(messageID, filename string) *File {
	if filename == "" {
		return nil
	}

	url := fmt.Sprintf("http://%s:%s/%s", fs.Host, fs.Port, filename)
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

	if messageID != "" {
		fs.IDToPath[messageID] = f
	}
	return f, nil
}

func (fs *FileServer) RemoveFile(file *File) error {
	delete(fs.IDToPath, file.MessageID)
	return os.Remove(file.Path)
}
