package dynamic

import (
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/Harvey-OS/ninep/protocol"
)

// A basic directory handler interprets the file
//  entries to show its children. The handler
//  does not support the creation of any children files.
type BasicDirHandler struct {
	S      *Server
	Filter func(name string) bool
}

func (b *BasicDirHandler) WalkChild(name string, child string) (int, error) {
	if name == "" {
		name = "/"
	}
	idx := b.S.MatchFile(func(f *FileEntry) bool { return f.Name == path.Join(name, child) })
	if idx == -1 {
		return idx, fmt.Errorf("File not found: %v\n", child)
	}

	return idx, nil
}

func (b *BasicDirHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	return nil
}

func (b *BasicDirHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creation is not supported")
}

func (b *BasicDirHandler) Stat(name string) (protocol.Dir, error) {
	contents, err := b.getDir(name, -1)
	if err != nil {
		return protocol.Dir{}, err
	}

	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTDIR}, Length: uint64(len(contents))}, nil
}

func (b *BasicDirHandler) getDir(name string, max int64) ([]byte, error) {
	matches := b.S.MatchFiles(func(f *FileEntry) bool {
		ischild := strings.HasPrefix(f.Name, name+"/") && strings.Count(name, "/") == strings.Count(f.Name, "/")-1
		if !ischild {
			return false
		}

		if b.Filter != nil {
			return b.Filter(f.Name)
		}

		return true
	})

	var bb bytes.Buffer

	for _, idx := range matches {
		match := &b.S.files[idx]

		var b bytes.Buffer
		dir := protocol.Dir{}
		dir, err := match.Handler.Stat(match.Name)
		if err != nil {
			return []byte{}, err
		}
		dir.QID.Path = uint64(idx)

		m := 0755
		if dir.QID.Type&protocol.QTDIR != 0 {
			m = m | protocol.DMDIR
		}
		dir.Mode = uint32(m)
		dir.Name = path.Base(match.Name)

		protocol.Marshaldir(&b, dir)
		bb.Write(b.Bytes())
		if max != -1 && int64(bb.Len())+int64(b.Len()) > max {
			break
		}
	}

	return bb.Bytes(), nil
}

func (b *BasicDirHandler) Wstat(name string, dir protocol.Dir) error {
	return fmt.Errorf("Wstat is not supported")
}

func (b *BasicDirHandler) Remove(name string) error {
	return fmt.Errorf("Remove is not supported")
}

func (b *BasicDirHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	content, err := b.getDir(name, offset+count)
	if err != nil {
		return []byte{}, err
	}

	if offset >= int64(len(content)) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(len(content)) {
		return content[offset:], nil
	}

	return content[offset : offset+count], nil
}

func (b *BasicDirHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Write is not supported")
}

func (b *BasicDirHandler) Clunk(name string, fid protocol.FID) error {
	return nil
}
