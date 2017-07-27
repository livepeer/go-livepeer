// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"bytes"
	"net"

	"github.com/aristanetworks/glog"
)

type telnetClient struct {
	addr string
	conn net.Conn
}

func newTelnetClient(addr string) OpenTSDBConn {
	return &telnetClient{
		addr: addr,
	}
}

func readErrors(conn net.Conn) {
	var buf [4096]byte
	for {
		// TODO: We should add a buffer to read line-by-line properly instead
		// of using a fixed-size buffer and splitting on newlines manually.
		n, err := conn.Read(buf[:])
		if n == 0 {
			return
		} else if n > 0 {
			for _, line := range bytes.Split(buf[:n], []byte{'\n'}) {
				if s := string(line); s != "" {
					glog.Info("tsd replied: ", s)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (c *telnetClient) Put(d *DataPoint) error {
	return c.PutBytes([]byte(d.String()))
}

func (c *telnetClient) PutBytes(d []byte) error {
	var err error
	if c.conn == nil {
		c.conn, err = net.Dial("tcp", c.addr)
		if err != nil {
			return err
		}
		go readErrors(c.conn)
	}
	_, err = c.conn.Write(d)
	if err != nil {
		c.conn.Close()
		c.conn = nil
	}
	return err
}
