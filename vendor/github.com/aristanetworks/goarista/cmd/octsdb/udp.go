// Copyright (C) 2017  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

import (
	"math/rand"
	"time"

	"github.com/aristanetworks/glog"
	kcp "github.com/xtaci/kcp-go"
)

type udpClient struct {
	addr    string
	conn    *kcp.UDPSession
	parity  int
	timeout time.Duration
}

func newUDPClient(addr string, parity int, timeout time.Duration) OpenTSDBConn {
	return &udpClient{
		addr:    addr,
		parity:  parity,
		timeout: timeout,
	}
}

func (c *udpClient) Put(d *DataPoint) error {
	var err error
	if c.conn == nil {
		// Prevent a bunch of clients all disconnecting and attempting to reconnect
		// at nearly the same time.
		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

		c.conn, err = kcp.DialWithOptions(c.addr, nil, 10, c.parity)
		if err != nil {
			return err
		}
		c.conn.SetNoDelay(1, 40, 1, 1) // Suggested by kcp-go to lower cpu usage
	}

	dStr := d.String()
	glog.V(3).Info(dStr)

	c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
	_, err = c.conn.Write([]byte(dStr))
	if err != nil {
		c.conn.Close()
		c.conn = nil
	}
	return err
}

type udpServer struct {
	lis    *kcp.Listener
	telnet *telnetClient
}

func newUDPServer(udpAddr, tsdbAddr string, parity int) (*udpServer, error) {
	lis, err := kcp.ListenWithOptions(udpAddr, nil, 10, parity)
	if err != nil {
		return nil, err
	}
	return &udpServer{
		lis:    lis,
		telnet: newTelnetClient(tsdbAddr).(*telnetClient),
	}, nil
}

func (c *udpServer) Run() error {
	for {
		conn, err := c.lis.AcceptKCP()
		if err != nil {
			return err
		}
		conn.SetNoDelay(1, 40, 1, 1) // Suggested by kcp-go to lower cpu usage
		if glog.V(3) {
			glog.Infof("New connection from %s", conn.RemoteAddr())
		}
		go func() {
			defer conn.Close()
			var buf [4096]byte
			for {
				n, err := conn.Read(buf[:])
				if err != nil {
					if n != 0 { // Not EOF
						glog.Error(err)
					}
					return
				}
				if glog.V(3) {
					glog.Info(string(buf[:n]))
				}
				err = c.telnet.PutBytes(buf[:n])
				if err != nil {
					glog.Error(err)
					return
				}
			}
		}()
	}
}
