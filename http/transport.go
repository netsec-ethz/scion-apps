// Contains code from the

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"bufio"
	"errors"
	"fmt"
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

// RoundTrip makes a single HTTP roundtrip
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	//ctx := req.Context()

	// check request health
	if req.URL == nil {
		closeBody(req)
		return nil, errors.New("http: nil Request.Header")
	}
	if req.Header == nil {
		closeBody(req)
		return nil, errors.New("http: nil Request.Header")
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
		return nil, fmt.Errorf("unsupported protocol scheme %v", scheme)
	}
	if req.Method != "" && !validMethod(req.Method) {
		return nil, fmt.Errorf("snet/shttp: invalid method %q", req.Method)
	}
	if req.URL.Host == "" {
		closeBody(req)
		return nil, errors.New("shttp: no Host in request URL")

	}
	fmt.Println("a")
	log.Println(req.URL.String())

	conn, err := dial(t.LAddr, t.DNS[req.URL.String()])
	if err != nil {
		return nil, err
	}
	defer conn.Close()

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
	//defer sess.Close(nil)
	if err != nil {
		return nil, fmt.Errorf("Error dialing SCION: %v", err)
	}

	stream, err := sess.OpenStreamSync()
	//defer stream.Close()
	if err != nil {
		return nil, fmt.Errorf("Error opening stream: %v", err)
	}

	qc := &quicconn.QuicConn{sess, stream}

	return qc, nil

}

func initSCION(lAddr *snet.Addr) error {

	return snet.Init(lAddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
}
