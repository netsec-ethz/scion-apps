// This is the low-level Transport implementation of the RoundTripper interface for use with SCION/QUIC
// The high-level interface is in http/client.go

package shttp

import (
	"crypto/tls"
	"log"
	"net/http"
	"sync"

	"github.com/chaehni/scion-http/utils"
	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/h2quic"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

// Transport wraps a h2quic.RoundTripper and makes it compatible with SCION
type Transport struct {
	LAddr *snet.Addr
	DNS   map[string]*snet.Addr // map from services to SCION addresses

	rt *h2quic.RoundTripper

	dialOnce sync.Once
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	return t.RoundTripOpt(req, h2quic.RoundTripOpt{})
}

// RoundTripOpt is the same as RoundTrip but takes additional options
func (t *Transport) RoundTripOpt(req *http.Request, opt h2quic.RoundTripOpt) (*http.Response, error) {

	// initialize the SCION networking context once for all Transports
	initOnce.Do(func() {
		if snet.DefNetwork == nil {
			initErr = snet.Init(t.LAddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
		}
	})
	if initErr != nil {
		return nil, initErr
	}

	// set the dial function once for each Transport
	t.dialOnce.Do(func() {
		dial := func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error) {
			raddr, ok := t.DNS[req.URL.Host]
			if !ok {
				log.Fatal("shttp: Host not found in DNS map")
			}
			return squic.DialSCION(nil, t.LAddr, raddr)
		}
		t.rt = &h2quic.RoundTripper{
			Dial: dial,
		}
	})

	return t.rt.RoundTripOpt(req, opt)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *Transport) Close() error {
	return t.rt.Close()
}
