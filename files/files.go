package files

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

type File struct {
	Hash string
	Path string
	URL  string
}

type FileServer struct {
	Host      string
	Port      string
	UseHTTPS  bool
	Directory string

	httpServer *http.Server

	mutex      sync.RWMutex
	hashToPath map[string]*File
}

func MakeFileServer(host, port, dir string, useHTTPS bool) (*FileServer, error) {
	fs := &FileServer{
		Host:      host,
		Port:      port,
		UseHTTPS:  useHTTPS,
		Directory: dir,

		hashToPath: make(map[string]*File),
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
			fname := f.Name()
			dotIndex := strings.LastIndexByte(fname, '.')

			if f.IsDir() || fname[0] == '.' {
				continue
			}

			var b64url string
			var ext string
			if dotIndex == -1 { // no extension
				b64url = fname
				ext = ""
			} else {
				b64url = fname[:dotIndex]
				ext = fname[dotIndex+1:]
			}

			hash, err := b64urltob64(b64url)
			if err != nil {
				continue
			}

			fs.hashToPath[hash] = fs.makeFile(hash, ext)
		}
	}

	return fs, nil
}

func (fs *FileServer) Start() error {
	fs.httpServer = &http.Server{
		Addr:    ":" + fs.Port,
		Handler: noDirListing(http.FileServer(http.Dir(fs.Directory))),
	}

	return fs.httpServer.ListenAndServe()
}

func (fs *FileServer) Stop() error {
	if err := fs.httpServer.Close(); err != nil {
		return err
	}

	fs.httpServer = nil
	return nil
}

func (fs *FileServer) makeFile(hash, ext string) *File {
	if hash == "" {
		return nil
	}

	urlHash, err := b64tob64url(hash)
	if err != nil {
		urlHash = hash
	}

	var fname string
	if ext != "" {
		fname = urlHash + "." + ext
	} else {
		fname = urlHash
	}

	protocol := "http"
	if fs.UseHTTPS {
		protocol = "https"
	}
	var url string
	if fs.Port == "80" {
		url = fmt.Sprintf("%s://%s/%s", protocol, fs.Host, fname)
	} else {
		url = fmt.Sprintf("%s://%s:%s/%s", protocol, fs.Host, fs.Port, fname)
	}

	path := fmt.Sprintf("./%s/%s", fs.Directory, fname)

	return &File{
		Hash: hash,
		URL:  url,
		Path: path,
	}
}

func (fs *FileServer) AddBlob(hash, ext string, bytes []byte) (*File, error) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if hash == "" || len(bytes) == 0 {
		return nil, fmt.Errorf("hash or bytes can't be empty")
	}

	f := fs.makeFile(hash, ext)
	if f == nil {
		return nil, fmt.Errorf("error while creating file object")
	}

	err := ioutil.WriteFile(f.Path, bytes, 0644)
	if err != nil {
		return nil, err
	}

	fs.hashToPath[hash] = f
	return f, nil
}

func (fs *FileServer) RemoveFile(file *File) error {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if err := os.Remove(file.Path); err != nil {
		return err
	}

	delete(fs.hashToPath, file.Hash)
	return nil
}

func (fs *FileServer) GetFileByHash(hash string) (file *File, has bool) {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	file, has = fs.hashToPath[hash]
	return file, has
}
