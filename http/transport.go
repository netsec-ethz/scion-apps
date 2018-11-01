// Contains code from the

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
	"golang.org/x/net/http/httpguts"
)

var (
	networkInitialized bool
)

type Transport struct {
	LAddr *snet.Addr
	DNS   map[string]*snet.Addr // map from services to SCION addresses
}

// Body wraps io.Readcloser together with a connection
// Like this we can override the Close() method to also close connection
// after client consume a response body
type Body struct {
	io.ReadCloser
	conn net.Conn
}

func (b *Body) Close() error {
	b.conn.Close()
	return b.ReadCloser.Close()
}

// RoundTrip makes a single HTTP roundtrip
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	//ctx := req.Context()

	// check request health
	if req.URL == nil {
		closeBody(req)
		return nil, errors.New("shttp: nil Request.Header")
	}
	if req.Header == nil {
		closeBody(req)
		return nil, errors.New("shttp: nil Request.Header")
	}
	scheme := req.URL.Scheme
	isHTTP := scheme == "http" || scheme == "https"
	if isHTTP {
		for k, vv := range req.Header {
			if !httpguts.ValidHeaderFieldName(k) {
				return nil, fmt.Errorf("snet/shttp: invalid header field name %q", k)
			}
			for _, v := range vv {
				if !httpguts.ValidHeaderFieldValue(v) {
					return nil, fmt.Errorf("snet/shttp: invalid header field value %q for key %v", v, k)
				}
			}
		}
	}
	if !isHTTP {
		closeBody(req)
		return nil, fmt.Errorf("shttp: unsupported protocol scheme %v", scheme)
	}
	if req.Method != "" && !validMethod(req.Method) {
		return nil, fmt.Errorf("shttp: invalid method %q", req.Method)
	}
	if req.URL.Host == "" {
		closeBody(req)
		return nil, errors.New("shttp: no Host in request URL")

	}

	addr, ok := t.DNS[req.URL.Host]
	if !ok {
		log.Fatal("shttp: Host not found in DNS map")
	}

	conn, err := dial(t.LAddr, addr)
	if err != nil {
		return nil, err
	}

	// write request to conn
	err = req.Write(conn)
	if err != nil {
		return nil, err
	}

	br := bufio.NewReader(conn)

	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}

	// Replace response body with custom Body type that closes connection
	// when client closes response body is closed
	resp.Body = &Body{resp.Body, conn}

	return resp, nil

}

func validMethod(method string) bool {

	m := map[string]bool{
		"OPTIONS": true,
		"GET":     true,
		"HEAD":    true,
		"POST":    true,
		"PUT":     true,
		"DELETE":  true,
		"TRACE":   true,
		"CONNECT": true,
	}

	_, ok := m[method]
	return ok
}

func closeBody(r *http.Request) {
	if r.Body != nil {
		r.Body.Close()
	}
}

func dial(lAddr, rAddr *snet.Addr) (net.Conn, error) {

	// Initialize the SCION/QUIC network connection
	if !networkInitialized {
		err := initSCION(lAddr)
		if err != nil {
			return nil, err
		}
		networkInitialized = true
	}

	// Establish QUIC connection to server
	sess, err := squic.DialSCION(nil, lAddr, rAddr)
	if err != nil {
		return nil, fmt.Errorf("Error dialing SCION: %v", err)
	}

	stream, err := sess.OpenStreamSync()
	if err != nil {
		return nil, fmt.Errorf("Error opening stream: %v", err)
	}

	qc := &quicconn.QuicConn{sess, stream}

	return qc, nil

}

func initSCION(lAddr *snet.Addr) error {

	if snet.DefNetwork != nil {
		return nil
	}
	return snet.Init(lAddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
}
