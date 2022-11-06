package nextexport

import (
	"io/fs"
	"os"
	"path"
)

type minfs struct {
	name string
}

func (sf *minfs) Open(name string) (fs.File, error) {
	return os.Open(path.Join(sf.name, name))
}

func (sf *minfs) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(path.Join(sf.name, name))
}

func NewMinFs(name string) FS {
	return &minfs{name: name}
}
