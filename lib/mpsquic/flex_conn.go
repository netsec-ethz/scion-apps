// Copyright 2017 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mpsquic

import (
	"net"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/snet"
)

var _ net.Conn = (*SCIONFlexConn)(nil)
var _ net.PacketConn = (*SCIONFlexConn)(nil)
var _ snet.Conn = (*SCIONFlexConn)(nil)

type SCIONFlexConn struct {
	sconn snet.Conn

	raddr *snet.Addr

	raddrs []*snet.Addr // Backup raddrs, w path, includes raddr
}

func newSCIONFlexConn(sconn  snet.Conn, raddrs []*snet.Addr) *SCIONFlexConn {
	c := &SCIONFlexConn{
		sconn:         sconn,

		raddr:        raddrs[0],
		raddrs:       raddrs,
	}
	return c
}

func (c *SCIONFlexConn) Read(b []byte) (int, error) {
	return c.sconn.Read(b)
}

func (c *SCIONFlexConn) ReadFrom(b []byte) (int, net.Addr, error) {
	return c.sconn.ReadFrom(b)
}

func (c *SCIONFlexConn) ReadFromSCION(b []byte) (int, *snet.Addr, error) {
	return c.sconn.ReadFromSCION(b)
}

func (c *SCIONFlexConn) Write(b []byte) (n int, err error) {
	return c.sconn.WriteToSCION(b, c.raddr)
}
func (c *SCIONFlexConn) WriteTo(b []byte, raddr net.Addr) (int, error) {
	_, ok := raddr.(*snet.Addr)
	if !ok {
		return 0, common.NewBasicError("Unable to write to non-SCION address", nil, "addr", raddr)
	}
	return c.WriteToSCION(b, c.raddr)
}

func (c *SCIONFlexConn) WriteToSCION(b []byte, raddr *snet.Addr) (int, error) {
	return c.sconn.WriteToSCION(b, c.raddr)
}

func (c *SCIONFlexConn) Close() error {
	return c.sconn.Close()
}

func (c *SCIONFlexConn) LocalAddr() net.Addr {
	return c.sconn.LocalAddr()
}

func (c *SCIONFlexConn) BindAddr() net.Addr {
	return c.sconn.BindAddr()
}

func (c *SCIONFlexConn) SVC() addr.HostSVC {
	return c.sconn.SVC()
}

func (c *SCIONFlexConn) RemoteAddr() net.Addr {
	return c.sconn.RemoteAddr()
}

func (c *SCIONFlexConn) SetDeadline(t time.Time) error {
	return c.sconn.SetDeadline(t)
}

func (c *SCIONFlexConn) SetReadDeadline(t time.Time) error {
	return c.sconn.SetReadDeadline(t)
}

func (c *SCIONFlexConn) SetWriteDeadline(t time.Time) error {
	return c.sconn.SetWriteDeadline(t)
}
