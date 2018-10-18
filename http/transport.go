package shttp

import (
	"errors"
	"fmt"
	"net/http"

	"golang_org/x/net/http/httpguts"

	"github.com/scionproto/scion/go/lib/snet"
)

// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

type Transport struct {
	Dns map[string]*snet.Addr // map from services to SCION addresses
}

// roundTrip implements a RoundTripper over HTTP
func (t *Transport) roundTrip(req *http.Request) (*http.Response, error) {

	ctx := req.Context()

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

	}

	pconn, err := t.getConn(treq, cm)
	resp, err := pconn.roundTrip(treq)

}

// getConn dials and creates a new persistConn to the target as specified in the connectMethod
// This includes setting up SCION/QUIC. If this doesn't return an error, persistConn is readty
// to written requests to
func (t *Transport) getConn(treq *transportRequest, cm connectMethod) (*persistConn, error) {

}
