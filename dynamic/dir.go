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
	DirEntryMiss      func(s *Server, name string, child string) (FileHandler, error)
	ChildrenRequested func(s *Server, name string) error
}

func (b BasicDirHandler) WalkChild(s *Server, name string, child string) (int, error) {
	if name == "" {
		name = "/"
	}
	idx := s.MatchFile(func(f *FileEntry) bool { return f.name == path.Join(name, child) })
	if idx == -1 && b.DirEntryMiss != nil {
		newHandler, err := b.DirEntryMiss(s, name, child)
		if err != nil {
			return -1, err
		}
		newEntry := NewFileEntry(path.Join(name, child), newHandler)
		newIdx := s.AddFileEntry(newEntry)
		idx = newIdx
	}

	if idx == -1 {
		return idx, fmt.Errorf("File not found: %v\n", child)
	}

	return idx, nil
}

func (b BasicDirHandler) Open(name string, mode protocol.Mode) error {
	return nil
}

func (b BasicDirHandler) CreateChild(s *Server, name string, child string) (int, error) {
	return -1, fmt.Errorf("Creation is not supported")
}

func (b BasicDirHandler) Stat(name string) (protocol.QID, error) {
	return protocol.QID{Version: 0, Type: protocol.QTDIR}, nil
}

func (b BasicDirHandler) getDir(s *Server, name string, length bool) ([]byte, error) {
	if b.ChildrenRequested != nil {
		err := b.ChildrenRequested(s, name)
		if err != nil {
			return []byte{}, err
		}
	}

	matches := s.MatchFiles(func(f *FileEntry) bool {
		return strings.HasPrefix(f.name, name+"/") && strings.Count(name, "/") == strings.Count(f.name, "/")-1
	})

	var bb bytes.Buffer

	for _, idx := range matches {
		match := &s.files[idx]

		var b bytes.Buffer
		dir := protocol.Dir{}
		qid, err := match.handler.Stat(match.name)
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
			l, err := match.handler.Length(s, match.name)
			if err != nil {
				return []byte{}, err
			}
			dir.Length = l
		}
		dir.Name = path.Base(match.name)

		protocol.Marshaldir(&b, dir)
		bb.Write(b.Bytes())
	}

	return bb.Bytes(), nil
}

func (b BasicDirHandler) Length(s *Server, name string) (uint64, error) {
	contents, err := b.getDir(s, name, false)
	if err != nil {
		return 0, err
	}

	return uint64(len(contents)), nil
}

func (b BasicDirHandler) Wstat(name string, qid protocol.QID, length uint64) error {
	return fmt.Errorf("Wstat is not supported")
}

func (b BasicDirHandler) Remove(s *Server, name string) error {
	return fmt.Errorf("Remove is not supported")
}

func (b BasicDirHandler) Read(s *Server, name string, offset int64, count int64) ([]byte, error) {
	content, err := b.getDir(s, name, true)
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

func (b BasicDirHandler) Write(s *Server, name string, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Write is not supported")
}
