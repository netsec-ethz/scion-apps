// Contains code from the

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Transport struct {
	Dns map[string]*snet.Addr // map from services to SCION addresses
}

// roundTrip implements a RoundTripper over HTTP
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	/* ctx := req.Context()

	// check request health
	if req.URL == nil {
		req.closeBody() // TODO: not exported, req.Body.Close() instead (what if body is nil?)
		return nil, errors.New("http: nil Request.Header")
	}
	if req.Header == nil {
		req.closeBody()
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
		req.closeBody()
		return nil, &http.badStringError{"unsupported protocol scheme", scheme}
	}
	if req.Method != "" && !validMethod(req.Method) {
		return nil, fmt.Errorf("snet/shttp: invalid method %q", req.Method)
	}
	if req.URL.Host == "" {
		req.closeBody()
		return nil, errors.New("shttp: no Host in request URL")

	} */

	/* str := `HTTP/2 200
		server: nginx/1.13.6
	date: Thu, 18 Oct 2018 16:25:36 GMT
	content-type: text/plain;charset=UTF-8
	vary: Accept-Encoding
	access-control-allow-origin: *
	x-frame-options: SAMEORIGIN
	x-xss-protection: 1; mode=block
	x-content-type-options: nosniff

	213.55.184.236`

		log.Println("a")
		sr := strings.NewReader(str) // returns strings.Reader which implements io.Reader
		log.Println("b")
		br := bufio.NewReader(sr) // returns bufio.Reader
		log.Println("c") */

	log.Println(req.URL.String())

	initSCIONConnection(req.URL.String())

	conn, err := Get(Dns[req.URL.String()])
	if err != nil {
		return nil, err
	}

	br, err := bufio.NewReader(conn)
	if err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}

	log.Println("d")

	return resp, nil

	/* pconn, err := t.getConn(treq, cm)
	resp, err := pconn.roundTrip(treq) */

}

// getConn dials and creates a new persistConn to the target as specified in the connectMethod
// This includes setting up SCION/QUIC. If this doesn't return an error, persistConn is readty
// to written requests to
/* func (t *Transport) getConn(treq *transportRequest, cm connectMethod) (*persistConn, error) {

} */

func Get(serverAddress string) (net.Conn, error) {

	// Initialize the SCION/QUIC network connection
	err := c.initSCIONConnection(serverAddress)
	if err != nil {
		return "", err
	}

	// Establish QUIC connection to server
	sess, err := squic.DialSCION(nil, cAddr, srvAddr)
	defer sess.Close(nil)
	if err != nil {
		return nil, fmt.Errorf("Error dialing SCION: %v", err)
	}

	stream, err := sess.OpenStreamSync()
	defer stream.Close()
	if err != nil {
		return nil, fmt.Errorf("Error opening stream: %v", err)
	}

	qc := &quicconn.QuicConn{sess, stream}

	fmt.Fprint(qc, "GET /hello_world.html HTTP/1.1\r\n")
	fmt.Fprint(qc, "Content-Type: text/html\r\n")
	fmt.Fprint(qc, "\r\n")

	return qc, nil

}

func initSCIONConnection(rAddr *snet.Addr) error {

	err = snet.Init(rAddr.IA, utils.GetSciondAddr(rAddr), utils.GetDispatcherAddr(rAddr))
	if err != nil {
		return err
	}

	log.Println("Initialized SCION network")

	return nil
}
