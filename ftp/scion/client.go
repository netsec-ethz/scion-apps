package scion

import (
	"bufio"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"github.com/lucas-clemente/quic-go"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/netsec-ethz/scion-apps/pkg/appnet/appquic"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
)

func DialAddr(localAddr, remoteAddr string, openKeepAlive bool) (*Connection, *Connection, error) {

	local, err := ConvertAddress(localAddr)
	if err != nil {
		return nil, nil, err
	}

	remote, err := ConvertAddress(remoteAddr)
	if err != nil {
		return nil, nil, err
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"scionftp"},
	}

	session, err := appquic.Dial(remoteAddr, tlsConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to dial %s: %s", remote, err)
	}

	_, port, err := ParseCompleteAddress(session.LocalAddr().String())
	if err != nil {
		return nil, nil, err
	}

	local.port = port

	stream, err := AddStream(&session)
	if err != nil {
		return nil, nil, err
	}
	conn := NewAppQuicConnection(*stream, local, remote)

	var kConn *Connection = nil
	if openKeepAlive {
		kStream, err := AddStream(&session)
		if err != nil {
			return nil, nil, err
		}
		kConn = NewAppQuicConnection(*kStream, local, remote)
	}

	return conn, kConn, nil
}

func AddStream(session *quic.Session) (*quic.Stream, error) {
	stream, err := (*session).OpenStream()
	if err != nil {
		return nil, fmt.Errorf("unable to open stream: %s", err)
	}

	err = sendHandshake(stream)
	if err != nil {
		return nil, err
	}
	return &stream, nil
}

func sendHandshake(rw io.ReadWriter) error {

	msg := []byte{200}

	return binary.Write(rw, binary.BigEndian, msg)

	// log.Debug("Sent handshake", "msg", msg)
}

type PathSelector func([]*snet.Path) *snet.Path

func DefaultPathSelector(paths []*snet.Path) *snet.Path {
	return paths[0]
}

func NewStaticSelector() *StaticSelector {
	return &StaticSelector{}
}

type StaticSelector struct {
	sync.Mutex
	path *snet.Path
}

func (selector *StaticSelector) PathSelector(paths []*snet.Path) *snet.Path {
	selector.Lock()
	defer selector.Unlock()
	if selector.path == nil {
		selector.path = paths[0]
	}

	return selector.path
}

func (selector *StaticSelector) Reset() {
	selector.path = nil
}

// Copied from Pingpong sample application:
// https://github.com/scionproto/scion/blob/8291539e5b23a217cb367bce6da05b71d0fe1d82/go/examples/pingpong/pingpong.go#L419
func InteractivePathSelector(paths []*sciond.PathReplyEntry) *sciond.PathReplyEntry {
	if len(paths) == 1 {
		return paths[0]
	}

	var index uint64

	fmt.Printf("Available paths to\n")
	for i := range paths {
		fmt.Printf("[%2d] %s\n", i, paths[i].Path.String())
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Choose path: ")
		pathIndexStr, _ := reader.ReadString('\n')
		var err error
		index, err = strconv.ParseUint(pathIndexStr[:len(pathIndexStr)-1], 10, 64)
		if err == nil && int(index) < len(paths) {
			break
		}
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Invalid path index, valid indices range: [0, %v]\n",
			len(paths))
	}

	return paths[index]
}

type Rotator struct {
	max, current int
	paths        []*snet.Path
	sync.Mutex
}

func NewRotator(max int) *Rotator {
	return &Rotator{max: max}
}

func (r *Rotator) Reset(max int) {
	r.max = max
	r.current = 0
	r.paths = nil
}

func (r *Rotator) GetNumberOfUsedPaths() int {
	if r.current > len(r.paths) {
		return len(r.paths)
	} else {
		return r.current
	}
}

func (r *Rotator) PathSelector(paths []*snet.Path) *snet.Path {
	r.Lock()
	defer r.Unlock()

	if r.paths == nil {
		max := r.max
		if max > len(paths) {
			max = len(paths)
		}
		r.paths = paths[0:max]
	}

	idx := r.current % len(r.paths)
	r.current++
	return r.paths[idx]
}
