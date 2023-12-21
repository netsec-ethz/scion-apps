package pan

import (
	"context"
	"net"
	"net/netip"
)

/*
!
\brief a SCION UDP datagram socket
\details

	this is required by bindings to languages
	whose networking libraries main abstraction are berkley sockets,
	not connections as in Go. It is a prerequisite for i.e. P2P applications
	where you want to act as both client and server on the same address/socket
	utilizing QUIC implementations that support it i.e rust-quinn.
*/
type ScionSocket interface {
	Close() error
	Bind(context.Context, netip.AddrPort) error
	WriteToVia(b []byte, dst UDPAddr, path *Path) (int, error)
	ReadFromVia(b []byte) (int, UDPAddr, *Path, error)
	LocalAddr() net.Addr
	WriteTo(b []byte, dst UDPAddr) (int, error)
	ReadFrom(b []byte) (int, UDPAddr, error)

	// SetCombiSelector(CombiSelector)
	SetReplySelector(ReplySelector)
	SetSelector(sel func() Selector)
	SetPolicy(pol func() Policy)

	// setter for AS specific path policies
	// SetSelectorFor(remote IA, sel Selector)
	// SetPolicyFor(remote IA, pol Policy)
	// SetPolicedSelectorFor(remote IA, sel Selector, pol Policy)

	// SetReadDeadline()
	// SetWriteDeadline()
	// SetDeadline()
}

type scionSocket struct {
	local    UDPAddr
	conn     baseUDPConn
	selector CombiSelector
}

/*
!
\brief 	open a scion-socket that is bound to the specified local addr

TODO:
  - it should be possible to defer the binding of the socket
    from ctor to a later explicit Bind() call.
  - accept Selector, ReplySelector and Policy arguments here
    or a CombiSelector to customize the behaviour of the socket
*/
func NewScionSocket(ctx context.Context /*rsel ReplySelector, pol Policy, sel Selector,*/, local netip.AddrPort) (ScionSocket, error) {
	local, err := defaultLocalAddr(local)
	if err != nil {
		return nil, err
	}
	raw, slocal, err := openBaseUDPConn(ctx, local)
	if err != nil {
		return nil, err
	}

	// as of now the local parameter is only required
	// for the selector to determine if a remote is in the same AS as itself
	sel, e := NewDefaultCombiSelector(slocal)
	if e != nil {
		return nil, e
	}

	return &scionSocket{
		local:    slocal,
		conn:     baseUDPConn{raw: raw},
		selector: sel,
	}, nil
}

func (s *scionSocket) Close() error {
	if err := s.selector.Close(); err != nil {
		return err
	}
	if err := s.conn.Close(); err != nil {
		return err
	}
	return nil
}

func (s *scionSocket) SetReplySelector(rsel ReplySelector) {
	s.selector.SetReplySelector(rsel)
}
func (s *scionSocket) SetSelector(sel func() Selector) {
	s.selector.SetSelector(sel)
}
func (s *scionSocket) SetPolicy(pol func() Policy) {
	s.selector.SetPolicy(pol)
}

func (s *scionSocket) Bind(ctx context.Context, local netip.AddrPort) error {
	raw, slocal, err := openBaseUDPConn(ctx, local)
	if err != nil {
		return err
	}
	s.conn.raw = raw
	s.local = slocal

	// notify CombiSelector about address change
	s.selector.LocalAddrChanged(slocal)
	return nil

}

func (s *scionSocket) needPathTo(remote UDPAddr) bool {
	return s.local.IA != remote.IA
}

func (s *scionSocket) WriteToVia(b []byte, dst UDPAddr, path *Path) (int, error) {
	return s.conn.writeMsg(s.local, dst, path, b)
}

func (s *scionSocket) ReadFromVia(b []byte) (int, UDPAddr, *Path, error) {
	n, remote, fwPath, err := s.conn.readMsg(b)
	if err != nil {
		return n, UDPAddr{}, nil, err
	}
	path, err := reversePathFromForwardingPath(remote.IA, s.local.IA, fwPath)
	s.selector.Record(remote, path)
	return n, remote, path, err
}

func (s *scionSocket) LocalAddr() net.Addr {
	return s.local
}

func (s *scionSocket) WriteTo(b []byte, dst UDPAddr) (int, error) {
	var path *Path
	if s.needPathTo(dst) {
		path, _ = s.selector.Path(dst)
		if path == nil {
			return 0, errNoPathTo(dst.IA)
		}
	}
	return s.WriteToVia(b, dst, path)
}

func (s *scionSocket) ReadFrom(b []byte) (int, UDPAddr, error) {
	n, a, _, e := s.ReadFromVia(b)
	return n, a, e
}
