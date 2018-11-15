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

	mutex sync.Mutex
}

// RoundTrip does a single round trip; retreiving a response for a given request
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	return t.RoundTripOpt(req, h2quic.RoundTripOpt{})
}

// RoundTripOpt is the same as RoundTrip but takes additional options
func (t *Transport) RoundTripOpt(req *http.Request, opt h2quic.RoundTripOpt) (*http.Response, error) {

	t.mutex.Lock()
	// initialize the SCION network connection
	if snet.DefNetwork == nil {
		err := snet.Init(t.LAddr.IA, utils.GetSCIOND(), utils.GetDispatcher())
		if err != nil {
			return nil, err
		}
	}

	if t.rt == nil {
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
	}
	t.mutex.Unlock()

	return t.rt.RoundTripOpt(req, opt)
}

// Close closes the QUIC connections that this RoundTripper has used
func (t *Transport) Close() error {
	return t.rt.Close()
}
