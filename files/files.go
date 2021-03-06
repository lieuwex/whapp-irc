package files

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
)

// File represents a file on the FileServer.
type File struct {
	Hash string
	Path string
	URL  string
}

// FileServer represents a (running) file server.
type FileServer struct {
	Host      string
	Port      string
	UseHTTPS  bool
	Directory string

	mutex      sync.RWMutex
	hashToPath map[string]File
}

// MakeFileServer returns a new FileServer in the given dir, using the given
// options. It first scans the dir for older files, and loads them in the
// database.
func MakeFileServer(host, port, dir string, useHTTPS bool) (*FileServer, error) {
	fs := &FileServer{
		Host:      host,
		Port:      port,
		UseHTTPS:  useHTTPS,
		Directory: dir,

		hashToPath: make(map[string]File),
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

		for _, diskFile := range files {
			fname := diskFile.Name()
			dotIndex := strings.LastIndexByte(fname, '.')

			if diskFile.IsDir() || fname[0] == '.' {
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

			f, err := fs.makeFile(hash, ext)
			if err != nil {
				return nil, err
			}
			fs.hashToPath[hash] = f
		}
	}

	return fs, nil
}

// Serve starts the current FileServer.
func (fs *FileServer) Serve() error {
	httpServer := &http.Server{
		Addr:    ":" + fs.Port,
		Handler: noDirListing(http.FileServer(http.Dir(fs.Directory))),
	}

	return httpServer.ListenAndServe()
}

func (fs *FileServer) getURL(fname string) string {
	protocol := "http"
	if fs.UseHTTPS {
		protocol = "https"
	}

	if fs.Port == "80" {
		return fmt.Sprintf("%s://%s/%s", protocol, fs.Host, fname)
	}

	return fmt.Sprintf("%s://%s:%s/%s", protocol, fs.Host, fs.Port, fname)
}

func (fs *FileServer) makeFile(hash, ext string) (File, error) {
	if hash == "" {
		return File{}, ErrHashEmpty
	}

	urlHash, err := b64tob64url(hash)
	if err != nil {
		urlHash = hash
	}

	fname := urlHash
	if ext != "" {
		fname += "." + ext
	}

	return File{
		Hash: hash,
		URL:  fs.getURL(fname),
		Path: fmt.Sprintf("./%s/%s", fs.Directory, fname),
	}, nil
}

// AddBlob adds the given bytes blob to the database, using the given hash and
// extension for the file name.
func (fs *FileServer) AddBlob(hash, ext string, bytes []byte) (File, error) {
	if hash == "" {
		return File{}, ErrHashEmpty
	} else if len(bytes) == 0 {
		return File{}, ErrBytesEmpty
	}

	f, err := fs.makeFile(hash, ext)
	if err != nil {
		return File{}, err
	}

	if err := ioutil.WriteFile(f.Path, bytes, 0644); err != nil {
		return File{}, err
	}

	fs.mutex.Lock()
	fs.hashToPath[hash] = f
	fs.mutex.Unlock()

	return f, nil
}

// RemoveFile removes the file from disk matching the given file struct.
func (fs *FileServer) RemoveFile(file File) error {
	if err := os.Remove(file.Path); err != nil {
		return err
	}

	fs.mutex.Lock()
	delete(fs.hashToPath, file.Hash)
	fs.mutex.Unlock()

	return nil
}

// GetFileByHash returns the File struct matching the given hash.
func (fs *FileServer) GetFileByHash(hash string) (file File, has bool) {
	fs.mutex.RLock()
	file, has = fs.hashToPath[hash]
	fs.mutex.RUnlock()
	return file, has
}
