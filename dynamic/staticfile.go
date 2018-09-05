package dynamic

import (
	"fmt"

	"github.com/Harvey-OS/ninep/protocol"
)

// Static file handler has a static contents that
//  is initiated at startup and cannot be modified.
//  This is useful for README files and other helpful
//  documentation for your filesystem.
type StaticFileHandler struct {
	Content []byte
}

func (f *StaticFileHandler) WalkChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Children are not supported")
}

func (f *StaticFileHandler) Open(name string, fid protocol.FID, mode protocol.Mode) error {
	return nil
}

func (f *StaticFileHandler) CreateChild(name string, child string) (int, error) {
	return -1, fmt.Errorf("Creation is not supported")
}

func (f *StaticFileHandler) Stat(name string) (protocol.Dir, error) {
	// There's only one version and it is always a file
	return protocol.Dir{QID: protocol.QID{Version: 0, Type: protocol.QTFILE}, Length: uint64(len(f.Content))}, nil
}

func (f *StaticFileHandler) Wstat(name string, qid protocol.Dir) error {
	return fmt.Errorf("Wstat is not supported")
}

func (f *StaticFileHandler) Remove(name string) error {
	return fmt.Errorf("Remove is not supported")
}

func (f *StaticFileHandler) Read(name string, fid protocol.FID, offset int64, count int64) ([]byte, error) {
	if offset >= int64(len(f.Content)) {
		return []byte{}, nil // TODO should an error be returned?
	}

	if offset+count >= int64(len(f.Content)) {
		return f.Content[offset:], nil
	}

	return f.Content[offset : offset+count], nil
}

func (f *StaticFileHandler) Write(name string, fid protocol.FID, offset int64, buf []byte) (int64, error) {
	return 0, fmt.Errorf("Write is not supported")
}

func (f *StaticFileHandler) Clunk(name string, fid protocol.FID) error {
	return nil
}
