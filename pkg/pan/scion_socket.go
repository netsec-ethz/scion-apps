package pan

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/scionproto/scion/pkg/daemon"
	"github.com/scionproto/scion/private/app"
	"github.com/scionproto/scion/private/app/flag"
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

the socket's task is to wrap any application layer data it gets passed into a valid Scion Header
and send it off the IP underlay.
Any received ScionPackets will be stripped off their underlay and ScionHeader and the payload
passed to the application.
Packets with data other than application payload i.e. SCMP messages will be handled by the socket
and are not propagated to the application.
*/
type ScionSocket interface {
	Close() error
	Bind(context.Context, netip.AddrPort) error
	WriteToVia(b []byte, dst UDPAddr, path *Path) (int, error)
	ReadFromVia(b []byte) (int, UDPAddr, *Path, error)
	LocalAddr() net.Addr
	WriteTo(b []byte, dst net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)

	// SetCombiSelector(CombiSelector)
	SetReplySelector(ReplySelector)
	SetSelector(sel func() Selector)
	SetPolicy(pol func() Policy)

	// setter for AS specific path policies
	// SetSelectorFor(remote IA, sel Selector)
	// SetPolicyFor(remote IA, pol Policy)
	// SetPolicedSelectorFor(remote IA, sel Selector, pol Policy)

	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
	SetDeadline(time.Time) error
}

type scionSocket struct {
	local_ia IA
	local    UDPAddr
	conn     baseUDPConn
	selector CombiSelector
}

func (s *scionSocket) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

func (s *scionSocket) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

func (s *scionSocket) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
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
		local_ia: slocal.IA,
		local:    slocal,
		conn:     baseUDPConn{raw: raw},
		selector: sel,
	}, nil
}

/*
returns a socket which is not bound to an address yet
Bind() has to be called later in order to receive packets
*/
func NewScionSocket2() (ScionSocket, error) {

	// as of now the local parameter is only required
	// for the selector to determine if a remote is in the same AS as itself
	sel, e := NewDefaultCombiSelector2()
	if e != nil {
		return nil, e
	}

	return &scionSocket{
		local_ia: GetLocalIA(),

		selector: sel,
	}, nil
}

var local_ia_init bool = false
var local_ia IA

func GetLocalIA() IA {
	if !local_ia_init {
		var envFlags flag.SCIONEnvironment
		var service daemon.Service

		if err := envFlags.LoadExternalVars(); err != nil {
			panic(fmt.Sprintf("pan initialization failed: %v", err))
		}

		// is this even necessary ?! or can the IA be read from the envFlags already?! docs dont tell it
		daemonAddr := envFlags.Daemon()
		service = daemon.NewService(daemonAddr)
		ctx, cancelF := context.WithTimeout(context.Background(), time.Second)
		defer cancelF()
		conn, err := service.Connect(ctx)
		if err != nil {
			panic(fmt.Sprintf("connecting to SCION Daemon: %v", err))
		}
		defer conn.Close()

		info, err := app.QueryASInfo(ctx, conn)
		if err != nil {
			panic(fmt.Sprintf("%v", err))
		}
		local_ia_init = true
		local_ia = IA(info.IA)

	}
	return local_ia
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
	local, err := defaultLocalAddr(local)
	if err != nil {
		return err
	}
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

func (s *scionSocket) WriteTo(b []byte, dst net.Addr) (int, error) {
	sdst, ok := dst.(UDPAddr)
	if !ok {
		return 0, errBadDstAddress
	}
	var path *Path
	if s.needPathTo(sdst) {
		path, _ = s.selector.Path(sdst)
		if path == nil {
			return 0, errNoPathTo(sdst.IA)
		}
	}
	return s.WriteToVia(b, sdst, path)
}

func (s *scionSocket) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, _, e := s.ReadFromVia(b)
	return n, a, e
}
