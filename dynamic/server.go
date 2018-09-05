package dynamic

import (
	"bytes"
	"flag"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/Harvey-OS/ninep/protocol"
)

var (
	debug = flag.Bool("debug", false, "Enable 9P debugging")
)

// A file handler defines the behaviour of one or more file entries
type FileHandler interface {
	WalkChild(name string, child string) (int, error)
	Open(name string, fid protocol.FID, mode protocol.Mode) error
	CreateChild(name string, child string) (int, error)
	Stat(name string) (protocol.Dir, error)
	Wstat(name string, dir protocol.Dir) error
	Remove(name string) error
	Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error)
	Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error)
	Clunk(name string, fid protocol.FID) error
}

// A file entry is a location in the filesystem tree with a handler
//  that handles the file operations for it. The server keeps track
//  of the QID and FID's of the entries.
type FileEntry struct {
	Name    string
	fids    []protocol.FID
	Handler FileHandler
	m       sync.Mutex
}

func NewFileEntry(name string, handler FileHandler) FileEntry {
	return FileEntry{Name: name, Handler: handler}
}

func (fe *FileEntry) addFid(fid protocol.FID) {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.fids = append(fe.fids, fid)
}

func (fe *FileEntry) removeFid(fid protocol.FID) {
	fe.m.Lock()
	defer fe.m.Unlock()

	for idx, f := range fe.fids {
		if f == fid {
			fe.fids = append(fe.fids[:idx], fe.fids[idx+1:]...)
			return
		}
	}
}

func (fe *FileEntry) hasFid(fid protocol.FID) bool {
	fe.m.Lock()
	defer fe.m.Unlock()

	for _, f := range fe.fids {
		if f == fid {
			return true
		}
	}

	return false
}

// A server
type Server struct {
	files  []FileEntry
	iounit int
	m      sync.Mutex
}

func (s *Server) Rversion(msize protocol.MaxSize, version string) (protocol.MaxSize, string, error) {
	if version != "9P2000" {
		return 0, "", fmt.Errorf("%v not supported; only 9P2000", version)
	}
	return msize, version, nil
}

func (s *Server) MatchFile(matcher func(f *FileEntry) bool) int {
	s.m.Lock()
	defer s.m.Unlock()

	for idx := range s.files {
		if matcher(&s.files[idx]) {
			return idx
		}
	}

	return -1
}

func (s *Server) MatchFiles(matcher func(f *FileEntry) bool) []int {
	s.m.Lock()
	defer s.m.Unlock()

	files := []int{}

	for idx := range s.files {
		if matcher(&s.files[idx]) {
			files = append(files, idx)
		}
	}

	return files
}

func (s *Server) AddFileEntry(name string, handler FileHandler) int {
	s.m.Lock()
	defer s.m.Unlock()
	newEntry := NewFileEntry(name, handler)

	for idx := range s.files {
		if s.files[idx].Name == newEntry.Name {
			//s.files[idx].Handler = newEntry.Handler
			return idx
		}
	}

	s.files = append(s.files, newEntry)
	return len(s.files) - 1

}

func (s *Server) HasChildren(name string) bool {
	s.m.Lock()
	defer s.m.Unlock()

	for idx := range s.files {
		if strings.HasPrefix(s.files[idx].Name, name+"/") {
			return true
		}
	}

	return false
}

func (s *Server) Rattach(fid protocol.FID, afid protocol.FID, uname string, aname string) (protocol.QID, error) {
	if afid != protocol.NOFID {
		return protocol.QID{}, fmt.Errorf("We don't do auth attach")
	}

	idx := s.MatchFile(func(f *FileEntry) bool { return f.Name == aname })
	if idx == -1 {
		return protocol.QID{}, fmt.Errorf("File not found: %v\n", aname)
	}

	// Register this new FID for this entry
	s.files[idx].addFid(fid)

	dir, err := s.files[idx].Handler.Stat(aname)
	if err != nil {
		return protocol.QID{}, err
	}

	// Handler doesn't specify the path, we can fill it in
	dir.QID.Path = uint64(idx)

	return dir.QID, nil
}

func (s *Server) Rflush(o protocol.Tag) error {
	return nil
}

func (s *Server) Rwalk(fid protocol.FID, newfid protocol.FID, paths []string) ([]protocol.QID, error) {
	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return []protocol.QID{}, fmt.Errorf("File not found")
	}

	parent := &s.files[idx]
	if len(paths) == 0 {
		parent.addFid(newfid)
		return []protocol.QID{}, nil
	}

	p := parent.Name
	if p == "" {
		p = "/"
	}
	q := make([]protocol.QID, len(paths))

	for idx = range paths {
		idx2, err := parent.Handler.WalkChild(parent.Name, paths[idx])
		if err != nil {
			return []protocol.QID{}, err
		}

		parent = &s.files[idx2]
		dir, err := parent.Handler.Stat(parent.Name)
		if err != nil {
			return []protocol.QID{}, err
		}

		q[idx] = dir.QID

		// Assign the new FID to the last file
		if idx == len(paths)-1 {
			parent.addFid(newfid)
		}
	}

	return q, nil
}

func (s *Server) Ropen(fid protocol.FID, mode protocol.Mode) (protocol.QID, protocol.MaxSize, error) {

	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return protocol.QID{}, 0, fmt.Errorf("File not found")
	}

	f := s.files[idx]
	dir, err := f.Handler.Stat(f.Name)

	if err != nil {
		return protocol.QID{}, 0, err
	}
	dir.QID.Path = uint64(idx)

	err = f.Handler.Open(f.Name, fid, mode)
	if err != nil {
		return protocol.QID{}, 0, err
	}

	return dir.QID, protocol.MaxSize(s.iounit), nil
}

func (s *Server) Rcreate(fid protocol.FID, name string, perm protocol.Perm, mode protocol.Mode) (protocol.QID, protocol.MaxSize, error) {
	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return protocol.QID{}, 0, fmt.Errorf("File not found")
	}

	parent := s.files[idx]
	idx, err := parent.Handler.CreateChild(parent.Name, name)
	if err != nil {
		return protocol.QID{}, 0, err
	}

	child := s.files[idx]
	dir, err := child.Handler.Stat(child.Name)
	if err != nil {
		return protocol.QID{}, 0, err
	}
	dir.QID.Path = uint64(idx)
	return dir.QID, protocol.MaxSize(s.iounit), nil
}

func (s *Server) Rclunk(fid protocol.FID) error {
	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return fmt.Errorf("File not found")
	}

	f := &s.files[idx]
	err := f.Handler.Clunk(f.Name, fid)
	if err != nil {
		return err
	}
	f.removeFid(fid)

	return nil
}

func (s *Server) Rstat(fid protocol.FID) ([]byte, error) {
	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return []byte{}, fmt.Errorf("File not found")
	}

	f := s.files[idx]
	dir, err := f.Handler.Stat(f.Name)
	if err != nil {
		return []byte{}, fmt.Errorf("File not found")
	}
	dir.QID.Path = uint64(idx)

	dir.Mode = 0755
	if dir.QID.Type&protocol.QTDIR != 0 {
		dir.Mode = dir.Mode | protocol.DMDIR
	}

	dir.Name = path.Base(f.Name)
	if f.Name == "" {
		dir.Name = "/"
	}

	var b bytes.Buffer
	protocol.Marshaldir(&b, dir)
	return b.Bytes(), nil
}

func (s *Server) Rwstat(fid protocol.FID, b []byte) error {
	buf := bytes.NewBuffer(b)
	dir, err := protocol.Unmarshaldir(buf)
	if err != nil {
		return err
	}

	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return fmt.Errorf("File not found")
	}

	f := s.files[idx]
	return f.Handler.Wstat(f.Name, dir)
}

func (s *Server) Rremove(fid protocol.FID) error {
	/*idx := s.MatchFile(func (f *FileEntry) bool { return f.hasFid(fid) })
	  if idx == -1 {
	          return fmt.Errorf("File not found")
	  }

	  f := s.files[idx]
	  return f.Handler.Remove(f.Name)*/

	return fmt.Errorf("Remove is not supported since it would invalidate the existing QID's")
}

func (s *Server) Rread(fid protocol.FID, o protocol.Offset, c protocol.Count) ([]byte, error) {
	if int(c) == 0 {
		return []byte{}, nil
	}

	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return []byte{}, fmt.Errorf("File not found")
	}

	f := s.files[idx]
	return f.Handler.Read(f.Name, fid, int64(o), int64(c))
}

func (s *Server) Rwrite(fid protocol.FID, o protocol.Offset, b []byte) (protocol.Count, error) {
	idx := s.MatchFile(func(f *FileEntry) bool { return f.hasFid(fid) })
	if idx == -1 {
		return 0, fmt.Errorf("File not found")
	}

	f := s.files[idx]
	c, err := f.Handler.Write(f.Name, fid, int64(o), b)
	return protocol.Count(c), err
}

type ServerOpt func(*protocol.Server) error

func NewServer(files []FileEntry, opts ...protocol.ServerOpt) (*protocol.Server, *Server, error) {
	f := &Server{}
	f.files = files
	f.files = append([]FileEntry{NewFileEntry("", &BasicDirHandler{f, nil})}, f.files...)

	var d protocol.NineServer = f
	if *debug {
		d = &debugServer{f}
	}
	s, err := protocol.NewServer(d, opts...)
	if err != nil {
		return nil, nil, err
	}
	f.iounit = 8192
	return s, f, nil
}
