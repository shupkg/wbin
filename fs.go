package wbin

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Fs map[string]*File

func (fs Fs) Open(name string) (http.File, error) {
	file, find := fs[name]
	if !find {
		return nil, os.ErrNotExist
	}
	return &httpFile{fs: fs, file: file, rd: bytes.NewReader(file.Prepare())}, nil
}

func (fs Fs) ReadBytes(name string) ([]byte, error) {
	hf, err := fs.Open(name)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(hf)
}

var _ http.FileSystem = (Fs)(nil)

type File struct {
	Path        string
	FileName    string
	FileSize    int64
	FileModTime int64
	FileIsDir   bool

	Data string

	v []byte
	o sync.Once
	l sync.Mutex
}

func (f *File) Prepare() []byte {
	f.o.Do(func() {
		if !f.FileIsDir {
			f.l.Lock()
			gr, _ := gzip.NewReader(base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(f.Data)))
			f.v, _ = ioutil.ReadAll(gr)
			f.l.Unlock()
		}
	})
	return f.v
}

func (f *File) Bytes() []byte {
	return f.Prepare()
}

func (f *File) Reset() {
	f.l.Lock()
	f.v = f.v[:0]
	f.o = sync.Once{}
	f.l.Unlock()
}

func (f *File) Name() string {
	return f.FileName
}

func (f *File) Size() int64 {
	return f.FileSize
}

func (f *File) Mode() os.FileMode {
	return 0444
}

func (f *File) ModTime() time.Time {
	return time.Unix(f.FileModTime, 0)
}

func (f *File) IsDir() bool {
	return f.FileIsDir
}

func (f *File) Sys() interface{} {
	return f
}

var _ os.FileInfo = (*File)(nil)

type httpFile struct {
	fs   Fs
	file *File
	rd   *bytes.Reader
	c    fileChildren
	o    sync.Once
}

func (h *httpFile) Read(p []byte) (n int, err error) {
	return h.rd.Read(p)
}

func (h *httpFile) Seek(offset int64, whence int) (int64, error) {
	return h.rd.Seek(offset, whence)
}

func (h *httpFile) Close() error {
	return nil
}

func (h *httpFile) Readdir(count int) (result []os.FileInfo, err error) {
	if !h.file.IsDir() {
		return
	}
	h.o.Do(func() {
		for path, fi := range h.fs {
			if strings.HasPrefix(path, h.file.FileName) {
				h.c = append(h.c, fi)
			}
		}
		sort.Sort(h.c)
	})

	l := count
	if count > h.c.Len() {
		l = h.c.Len()
	}

	result = make([]os.FileInfo, l)
	copy(result, h.c[:l])
	return
}

func (h *httpFile) Stat() (os.FileInfo, error) {
	return h.file, nil
}

var _ http.File = (*httpFile)(nil)

type fileChildren []os.FileInfo

func (f fileChildren) Len() int {
	return len(f)
}

func (f fileChildren) Less(i, j int) bool {
	return f[i].Name() < f[j].Name()
}

func (f fileChildren) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

var _ sort.Interface = (fileChildren)(nil)
