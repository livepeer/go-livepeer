// Copyright (C) 2016  Arista Networks, Inc.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the COPYING file.

package main

type textDumper struct{}

func newTextDumper() OpenTSDBConn {
	return textDumper{}
}

func (t textDumper) Put(d *DataPoint) error {
	print(d.String())
	return nil
}
