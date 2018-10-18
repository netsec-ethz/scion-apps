package shttp

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Client struct {
	AddrString string
	Addr       *snet.Addr
	Transport http.RoundTripper //TODO RoundTripper
	CheckRedirect func(*http.Request, via []*http.Request)
	Jar http.CookieJar
	Timeout    time.Duration
}


func (c *Client) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.do(req)
}

var testHookClientDoResult func(retres *http.Response, reterr error)

func (c *Client) do(req *http.Request) (retres *http.Response, reterr error) {
	if testHookClientDoResult != nil {
		defer func() { testHookClientDoResult(retres, reterr) }()
	}
	if req.URL == nil {
		req.closeBody()
		return nil, &url.Error{
			Op:  urlErrorOp(req.Method),
			Err: errors.New("http: nil Request.URL"),
		}
	}

	var (
		deadline      = c.deadline() //TODO
		reqs          []*http.Request
		resp          *http.Response
		copyHeaders   = c.makeHeadersCopier(req) //TODO
		reqBodyClosed = false                    // have we closed the current req.Body?

		// Redirect behavior:
		redirectMethod string
		includeBody    bool
	)

	uerr := func(err error) error {
		// the body may have been closed already by c.send()
		if !reqBodyClosed {
			req.closeBody()
		}

		var urlStr string
		if resp != nil && resp.Request != nil {
			urlStr = http.stripPassword(resp.Request.URL)
		} else {
			urlStr = http.stripPassword(req.URL)
		}

		return &url.Error{
			Op:  urlErrorOp(reqs[0].Method),
			URL: urlStr,
			Err: err,
		}
	}

	for {
		// For all but the first request, create the next
		// request hop and replace req.
		if len(reqs) > 0 {
			loc := resp.Header.Get("Location")
			if loc == "" {
				resp.closeBody()
				return nil, uerr(fmt.Errorf("%d response missing Location header", resp.StatusCode))
			}
			u, err := req.URL.Parse(loc)
			if err != nil {
				resp.closeBody()
				return nil, uerr(fmt.Errorf("failed to parse Location header %q: %v", loc, err))
			}

			host := ""
			if req.Host != "" && req.Host != req.URL.Host {
				// If the caller specified a custom Host header and the
				// redirect location is relative, preserve the Host header
				// through the redirect. See issue #22233.
				if u, _ := url.Parse(loc); u != nil && !u.IsAbs() {
					host = req.Host
				}
			}
			ireq := reqs[0]
			req = &http.Request{
				Method:   redirectMethod,
				Response: resp,
				URL:      u,
				Header:   make(Header),
				Host:     host,
				Cancel:   ireq.Cancel,
				ctx:      ireq.ctx,
			}

			if includeBody && ireq.GetBody != nil {
				req.Body, err = ireq.GetBody()
				if err != nil {
					resp.closeBody()
					return nil, uerr(err)
				}
				req.ContentLength = ireq.ContentLength

			}

			// Copy original headers before setting the Referer,
			// in case the user set Referer on their first request.
			// If they really want to override, they can do it in
			// their CheckRedirect func.
			copyHeaders(req) //TODO

			// Add the Referer header from the most recent
			// request URL to the new one, if it's not https->http:
			if ref := http.refererForURL(reqs[len(reqs)-1].URL, req.URL); ref != "" {
				req.Header.Set("Referer", ref)
			}
			err = c.checkRedirect(req, reqs) //TODO

			// Sentinel error to let users select the
			// previous response, without closing its
			// body. See Issue 10069.
			if err == ErrUseLastResponse {
				return resp, nil
			}

			// Close the previous response's body. But
			// read at least some of the body so if it's
			// small the underlying TCP connection will be
			// re-used. No need to check for errors: if it
			// fails, the Transport won't reuse it anyway.
			const maxBodySlurpSize = 2 << 10

			if resp.ContentLength == -1 || resp.ContentLength <= maxBodySlurpSize {
				io.CopyN(ioutil.Discard, resp.Body, maxBodySlurpSize)
			}

			resp.Body.Close()
			if err != nil {

				// Special case for Go 1 compatibility: return both the response
				// and an error if the CheckRedirect function failed.
				// See https://golang.org/issue/3795
				// The resp.Body has already been closed.
				ue := uerr(err)
				ue.(*url.Error).URL = loc
				return resp, ue
			}
		}

		reqs = append(reqs, req)
		var err error
		var didTimeout func() bool
		if resp, didTimeout, err = c.send(req, deadline); err != nil { //TODO

			// c.send() always closes req.Body
			reqBodyClosed = true
			if !deadline.IsZero() && didTimeout() {
				err = &httpError{
					// TODO: early in cycle: s/Client.Timeout exceeded/timeout or context cancelation/
					err:     err.Error() + " (Client.Timeout exceeded while awaiting headers)",
					timeout: true,
				}
			}
			return nil, uerr(err)
		}

		var shouldRedirect bool
		redirectMethod, shouldRedirect, includeBody = http.redirectBehavior(req.Method, resp, reqs[0])
		if !shouldRedirect {
			return resp, nil
		}
		req.closeBody()
	}
}

func (c *Client) send(req *Request, deadline time.Time) (resp *Response, didTimeout func() bool, err error) {

	if c.Jar != nil {
		for _, cookie := range c.Jar.Cookies(req.URL) {
			req.AddCookie(cookie)
		}
	}

	resp, didTimeout, err = http.send(req, c.transport(), deadline) //TODO c.transport
	if err != nil {
		return nil, didTimeout, err
	}

	if c.Jar != nil {
		if rc := resp.Cookies(); len(rc) > 0 {
			c.Jar.SetCookies(req.URL, rc)
		}
	}

	return resp, nil, nil

}

func (c *Client) transport() RoundTripper {
	if c.Transport != nil {
		return c.Transport
	}
	return snet.DefaultTransport //TODO
}

func (c *Client) Get(serverAddress string) (string, error) {

	// Initialize the SCION/QUIC network connection
	srvAddr, cAddr, err := c.initSCIONConnection(serverAddress)
	if err != nil {
		return "", err
	}

	// Establish QUIC connection to server
	sess, err := squic.DialSCION(nil, cAddr, srvAddr)
	defer sess.Close(nil)
	if err != nil {
		return "", fmt.Errorf("Error dialing SCION: %v", err)
	}

	stream, err := sess.OpenStreamSync()
	defer stream.Close()
	if err != nil {
		return "", fmt.Errorf("Error opening stream: %v", err)
	}

	qc := &quicconn.QuicConn{sess, stream}

	fmt.Fprint(qc, "GET /hello_world.html HTTP/1.1\r\n")
	fmt.Fprint(qc, "Content-Type: text/html\r\n")
	fmt.Fprint(qc, "\r\n")

	buf, _ := ioutil.ReadAll(qc)
	return string(buf), nil

}

func (c *Client) initSCIONConnection(serverAddress string) (*snet.Addr, *snet.Addr, error) {

	log.Println("Initializing SCION connection")

	srvAddr, err := snet.AddrFromString(serverAddress)
	if err != nil {
		return nil, nil, err
	}

	c.Addr, err = snet.AddrFromString(c.AddrString)
	if err != nil {
		return nil, nil, err
	}

	err = snet.Init(c.Addr.IA, utils.GetSciondAddr(c.Addr), utils.GetDispatcherAddr(c.Addr))
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to initialize SCION network:", err)
	}

	log.Println("Initialized SCION network")

	return srvAddr, c.Addr, nil

}

func (c *Client) deadline() time.Time {
	if c.Timeout > 0 {
		return time.Now().Add(c.Timeout)
	}
	return time.Time{}
}

func (c *Client) checkRedirect(req *Request, via []*Request) error {
	fn := c.CheckRedirect
	if fn == nil {
		fn = http.defaultCheckRedirect
	}
	return fn(req, via)
}
