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
	S *Server
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

func (b *BasicDirHandler) Open(name string, mode protocol.Mode) error {
	return nil
}

func (b *BasicDirHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creation is not supported")
}

func (b *BasicDirHandler) Stat(name string) (protocol.QID, error) {
	return protocol.QID{Version: 0, Type: protocol.QTDIR}, nil
}

func (b *BasicDirHandler) getDir(name string, length bool) ([]byte, error) {
	matches := b.S.MatchFiles(func(f *FileEntry) bool {
		return strings.HasPrefix(f.Name, name+"/") && strings.Count(name, "/") == strings.Count(f.Name, "/")-1
	})

	var bb bytes.Buffer

	for _, idx := range matches {
		match := &b.S.Files[idx]

		var b bytes.Buffer
		dir := protocol.Dir{}
		qid, err := match.Handler.Stat(match.Name)
		if err != nil {
			return []byte{}, err
		}
		qid.Path = uint64(idx)
		dir.QID = qid

		m := 0755
		if dir.QID.Type&protocol.QTDIR != 0 {
			m = m | protocol.DMDIR
		}
		dir.Mode = uint32(m)

		if length {
			l, err := match.Handler.Length(match.Name)
			if err != nil {
				return []byte{}, err
			}
			dir.Length = l
		}
		dir.Name = path.Base(match.Name)

		protocol.Marshaldir(&b, dir)
		bb.Write(b.Bytes())
	}

	return bb.Bytes(), nil
}

func (b *BasicDirHandler) Length(name string) (uint64, error) {
	contents, err := b.getDir(name, false)
	if err != nil {
		return 0, err
	}

	return uint64(len(contents)), nil
}

func (b *BasicDirHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Wstat is not supported")
}

func (b *BasicDirHandler) Remove(name string) error {
	return fmt.Errorf("Remove is not supported")
}

func (b *BasicDirHandler) Read(name string, offset int64, count int64) ([]byte, error) {
	content, err := b.getDir(name, true)
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

func (b *BasicDirHandler) Write(name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Write is not supported")
}
