package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type File struct {
	Hash string
	Path string
	URL  string
}

type FileServer struct {
	Host      string
	Port      string
	Directory string

	HashToPath map[string]*File

	httpServer *http.Server
}

func MakeFileServer(host, port, dir string) (*FileServer, error) {
	fs := &FileServer{
		Host:      host,
		Port:      port,
		Directory: dir,

		HashToPath: make(map[string]*File),
	}

	err := os.Mkdir("./"+dir, 0700)
	if err != nil {
		if !os.IsExist(err) {
			return nil, err
		}

		files, err := ioutil.ReadDir("./" + dir)
		if err != nil {
			return nil, err
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}

			fname := f.Name()
			dotIndex := strings.LastIndexByte(fname, '.')
			if dotIndex == -1 {
				continue
			}

			hash, err := b64urltob64(fname[:dotIndex])
			if err != nil {
				continue
			}
			ext := fname[dotIndex+1:]

			fs.HashToPath[hash] = fs.MakeFile(hash, ext)
		}
	}

	return fs, nil
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

func (fs *FileServer) MakeFile(hash, ext string) *File {
	if hash == "" {
		return nil
	}

	b32Hash, err := b64tob64url(hash)
	if err != nil {
		b32Hash = hash
	}

	url := fmt.Sprintf("http://%s:%s/%s.%s", fs.Host, fs.Port, b32Hash, ext)
	file := fmt.Sprintf("./%s/%s.%s", fs.Directory, b32Hash, ext)

	return &File{
		Hash: hash,
		URL:  url,
		Path: file,
	}
}

func (fs *FileServer) AddBlob(hash, ext string, bytes []byte) (*File, error) {
	if hash == "" || ext == "" || len(bytes) == 0 {
		return nil, fmt.Errorf("hash, ext, or bytes can't be empty")
	}

	f := fs.MakeFile(hash, ext)
	if f == nil {
		return nil, fmt.Errorf("error while creating file object")
	}

	err := ioutil.WriteFile(f.Path, bytes, 0644)
	if err != nil {
		return nil, err
	}

	if hash != "" {
		fs.HashToPath[hash] = f
	}
	return f, nil
}

func (fs *FileServer) RemoveFile(file *File) error {
	delete(fs.HashToPath, file.Hash)
	return os.Remove(file.Path)
}
