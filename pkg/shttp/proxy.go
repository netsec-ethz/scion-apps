package shttp

import (
	"crypto/tls"
	"fmt"
	"net/http/httputil"
	"net/url"
)

// Proxies the incoming HTTP/1.1 request to the configured remote
// creating a new SCION HTTP/3 request
func NewSingleSCIONHostReverseProxy(remote string, cliTLSCfg *tls.Config) (*httputil.ReverseProxy, error) {
	// Enforce HTTPS, otherwise it cannot be parsed to URL
	sURL := MangleSCIONAddrURL(fmt.Sprintf("https://%s", remote))
	targetURL, err := url.Parse(sURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.Transport = NewRoundTripper(nil, cliTLSCfg, nil) // XXX policy!
	return proxy, nil
}
