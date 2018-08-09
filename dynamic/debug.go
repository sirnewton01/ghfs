// Copyright 2009 The Ninep Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dynamic

import (
	"bytes"
	"log"

	"github.com/Harvey-OS/ninep/protocol"
)

type debugServer struct {
	*Server
}

func (e *debugServer) Rversion(msize protocol.MaxSize, version string) (protocol.MaxSize, string, error) {
	log.Printf(">>> Tversion %v %v\n", msize, version)
	msize, version, err := e.Server.Rversion(msize, version)
	if err == nil {
		log.Printf("<<< Rversion %v %v\n", msize, version)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return msize, version, err
}

func (e *debugServer) Rattach(fid protocol.FID, afid protocol.FID, uname string, aname string) (protocol.QID, error) {
	log.Printf(">>> Tattach fid %v,  afid %v, uname %v, aname %v\n", fid, afid,
		uname, aname)
	qid, err := e.Server.Rattach(fid, afid, uname, aname)
	if err == nil {
		log.Printf("<<< Rattach %v\n", qid)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return qid, err
}

func (e *debugServer) Rflush(o protocol.Tag) error {
	log.Printf(">>> Tflush tag %v\n", o)
	err := e.Server.Rflush(o)
	if err == nil {
		log.Printf("<<< Rflush\n")
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return err
}

func (e *debugServer) Rwalk(fid protocol.FID, newfid protocol.FID, paths []string) ([]protocol.QID, error) {
	log.Printf(">>> Twalk fid %v, newfid %v, paths %v\n", fid, newfid, paths)
	qid, err := e.Server.Rwalk(fid, newfid, paths)
	if err == nil {
		log.Printf("<<< Rwalk %v\n", qid)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return qid, err
}

func (e *debugServer) Ropen(fid protocol.FID, mode protocol.Mode) (protocol.QID, protocol.MaxSize, error) {
	log.Printf(">>> Topen fid %v, mode %v\n", fid, mode)
	qid, iounit, err := e.Server.Ropen(fid, mode)
	if err == nil {
		log.Printf("<<< Ropen %v %v\n", qid, iounit)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return qid, iounit, err
}

func (e *debugServer) Rcreate(fid protocol.FID, name string, perm protocol.Perm, mode protocol.Mode) (protocol.QID, protocol.MaxSize, error) {
	log.Printf(">>> Tcreate fid %v, name %v, perm %v, mode %v\n", fid, name,
		perm, mode)
	qid, iounit, err := e.Server.Rcreate(fid, name, perm, mode)
	if err == nil {
		log.Printf("<<< Rcreate %v %v\n", qid, iounit)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return qid, iounit, err
}

func (e *debugServer) Rclunk(fid protocol.FID) error {
	log.Printf(">>> Tclunk fid %v\n", fid)
	err := e.Server.Rclunk(fid)
	if err == nil {
		log.Printf("<<< Rclunk\n")
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return err
}

func (e *debugServer) Rstat(fid protocol.FID) ([]byte, error) {
	log.Printf(">>> Tstat fid %v\n", fid)
	b, err := e.Server.Rstat(fid)
	if err == nil {
		dir, _ := protocol.Unmarshaldir(bytes.NewBuffer(b))
		log.Printf("<<< Rstat %v\n", dir)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return b, err
}

func (e *debugServer) Rwstat(fid protocol.FID, b []byte) error {
	dir, _ := protocol.Unmarshaldir(bytes.NewBuffer(b))
	log.Printf(">>> Twstat fid %v, %v\n", fid, dir)
	err := e.Server.Rwstat(fid, b)
	if err == nil {
		log.Printf("<<< Rwstat\n")
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return err
}

func (e *debugServer) Rremove(fid protocol.FID) error {
	log.Printf(">>> Tremove fid %v\n", fid)
	err := e.Server.Rremove(fid)
	if err == nil {
		log.Printf("<<< Rremove\n")
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return err
}

func (e *debugServer) Rread(fid protocol.FID, o protocol.Offset, c protocol.Count) ([]byte, error) {
	log.Printf(">>> Tread fid %v, off %v, count %v\n", fid, o, c)
	b, err := e.Server.Rread(fid, o, c)
	if err == nil {
		log.Printf("<<< Rread %v\n", len(b))
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return b, err
}

func (e *debugServer) Rwrite(fid protocol.FID, o protocol.Offset, b []byte) (protocol.Count, error) {
	log.Printf(">>> Twrite fid %v, off %v, count %v\n", fid, o, len(b))
	c, err := e.Server.Rwrite(fid, o, b)
	if err == nil {
		log.Printf("<<< Rwrite %v\n", c)
	} else {
		log.Printf("<<< Error %v\n", err)
	}
	return c, err
}
