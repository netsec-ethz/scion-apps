package shttp

import (
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"testing"
)

type stubConn struct {
	net.Conn
	buf   []byte
	index int
}

func (s *stubConn) Write(b []byte) (n int, err error) {
	copy(s.buf[s.index:], b)
	s.index += len(b)
	return len(b), nil
}

func (s *stubConn) readByte() byte {
	// this function assumes that eof() check was done before
	b := s.buf[0]
	s.buf = s.buf[1:]
	return b
}

func (s *stubConn) eof() bool {
	return s.buf[0] == 0
}

func (s *stubConn) Read(b []byte) (n int, err error) {
	if s.eof() {
		err = io.EOF
		return
	}

	if c := cap(b); c > 0 {
		for n < c {
			b[n] = s.readByte()
			n++
			if s.eof() {
				break
			}
		}
	}
	return
}

func TestWriteHeaderOnlyOnce(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}
	rw.WriteHeader(200)
	rw.WriteHeader(404)

	if rw.status != 200 {
		t.Errorf("Status-Code changed after call to WriteHeader")
	}

}

func TestCanAddHeaders(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.Header().Add("Custom-Header", "value")
	rw.WriteHeader(200)
	if rw.Header().Get("Custom-Header") != "value" {
		t.Errorf("Failed to add header")
	}
}

func TestCannotAddHeadersAfterWriteHeader(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 10000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.WriteHeader(200)

	// this should add to copy of sendHeader -> no aliasing
	rw.Header().Add("Custom-Header", "value")
	if rw.Header().Get("Custom-Header") != "" {
		t.Errorf("Header added after call to WriteHeader")
	}
}

func TestNotLeakAlias(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	// get a handle of header
	h := rw.Header()

	// this should assign a copy of handlerHeader to sendHeader -> no aliasing
	rw.WriteHeader(200)

	// try to add header via handle
	h.Add("Custom-Header", "value")
	if rw.Header().Get("Custom-Header") != "" {
		t.Errorf("Possible to write header after call to WriteHeader via alias handle")
	}
}

func TestInferContentLength(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	buf := make([]byte, 256)
	rand.Read(buf)
	rw.Write(buf)

	if rw.Header().Get("Content-Length") != "256" {
		t.Errorf("Content-Length header does not match length of body")
	}
}

func TestNotOverwriteContentLength(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.Header().Set("Content-Length", "0")
	buf := make([]byte, 256)
	rand.Read(buf)
	rw.Write(buf)

	if rw.Header().Get("Content-Length") != "0" {
		t.Errorf("Content-Length header was overwritten by server")
	}
}

func TestNotOverwriteContentType(t *testing.T) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           &stubConn{buf: make([]byte, 1000)},
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.Header().Set("Content-Type", "value")
	buf := make([]byte, 256)
	rand.Read(buf)
	rw.Write(buf)

	if rw.Header().Get("Content-Type") != "value" {
		t.Errorf("Content-Type header was overwritten by server")
	}
}

func TestTerminateHeaderCRLFWithBody(t *testing.T) {

	stub := &stubConn{buf: make([]byte, 1000)}
	rw := &RespWriter{
		contentLength:  -1,
		conn:           stub,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.Write([]byte("This is a sample payload"))

	s, _ := ioutil.ReadAll(stub)
	if string(s) != "HTTP/1.1 200 OK\r\n"+
		"Content-Length: 24\r\n"+
		"Content-Type: text/plain; charset=utf-8\r\n\r\n"+
		"This is a sample payload" {
		t.Error("Wrong wire format")
	}
}

func TestTerminateHeaderCRLFWithoutBody(t *testing.T) {

	stub := &stubConn{buf: make([]byte, 1000)}
	rw := &RespWriter{
		contentLength:  -1,
		conn:           stub,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}

	rw.WriteHeader(200)

	s, _ := ioutil.ReadAll(stub)
	if string(s) != "HTTP/1.1 200 OK\r\n" {
		t.Error("Wrong wire format")
	}
}
