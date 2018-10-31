package shttp

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

var (
	crlf = []byte("\r\n")
)

type Server struct {
	AddrString  string
	Addr        *snet.Addr
	TLSCertFile string
	TLSKeyFile  string
	Mux         *http.ServeMux
}

// check https://golang.org/src/net/http/server.go l1486 for Life of a Write as implemented in net/http
type RespWriter struct {
	status        int
	contentLength int // initialize to -1

	conn net.Conn

	// sendHeader that will be sent by WriteHeader
	// cannot be changed after call to WriteHeader
	sendHeader http.Header
	// This is the header Handlers get access to via Header()
	// It might change even after a call to WriteHeader
	handlerHeader http.Header
	// denotes wheter the sendHeader has been (logically) written
	wroteHeader bool
	// denotes wheter a header finalizing CRLF has been sent
	// this is for the case where Write is never called (e.g. 204)
	headerFinalized bool
	// headers to exclude from writing on call to Write()
	// have been written already on call to WriteHeader()
	excludeHeaders map[string]bool
}

func (rw *RespWriter) Write(data []byte) (int, error) {
	return rw.write(len(data), data, "")
}

func (w *RespWriter) WriteString(data string) (n int, err error) {
	return w.write(len(data), nil, data)
}

func (rw *RespWriter) write(lenData int, dataB []byte, dataS string) (n int, err error) {

	// Either dataB or dataS is non-nil
	var data []byte
	if dataB != nil {
		data = dataB
	} else {
		data = []byte(dataS)
	}

	// set status to an implicit 200 OK if not already set
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}

	// no data to send, return
	if lenData == 0 {
		return 0, nil
	}

	// if Content-Length not set, set it to lenData
	if rw.contentLength == -1 {
		rw.contentLength = lenData
		rw.sendHeader.Set("Content-Length", strconv.Itoa(lenData))
	}
	// return error if lenData is > 0 and status does not allow body
	if lenData > 0 && !bodyAllowedForStatus(rw.status) {
		return 0, errors.New("shttp: request method or response status code does not allow body")
	}
	// if Content-Type not set, detect it from first 512 bytes
	if ct := getHeader(rw.sendHeader, "Content-Type"); ct == "" {
		rw.sendHeader.Set("Content-Type", http.DetectContentType(data))
	}

	// write headers inferred from data
	// exclude the headers already sent
	rw.sendHeader.WriteSubset(rw.conn, rw.excludeHeaders)
	rw.conn.Write(crlf)
	rw.headerFinalized = true

	// write data to conn
	n, err = rw.conn.Write(data)
	return n, err
}

func bodyAllowedForStatus(status int) bool {
	switch {
	case status >= 100 && status <= 199:
		return false
	case status == 204:
		return false
	case status == 304:
		return false
	}
	return true
}

// Header returns the header that will be sent by WriteHeader
// if sendHeader has been (logically) written, Header returns a copy of sendHeader
func (rw *RespWriter) Header() http.Header {

	if rw.wroteHeader {
		return cloneHeader(rw.sendHeader)
	}
	return rw.handlerHeader
}

func (rw *RespWriter) WriteHeader(code int) {

	if rw.wroteHeader {
		log.Print("shttp: multiple WriteHeader calls")
		return
	}

	if code < 100 || code > 599 {
		panic(fmt.Sprintf("invalid WriteHeader code %v", code))
		return
	}

	rw.wroteHeader = true
	rw.status = code
	rw.sendHeader = cloneHeader(rw.handlerHeader)

	if cl := getHeader(rw.sendHeader, "Content-Length"); cl != "" {
		v, err := strconv.ParseInt(cl, 10, 64)
		if err == nil && v >= 0 {
			rw.contentLength = int(v)
		} else {
			log.Printf("shttp: invalid Content-Length of %q", cl)
			rw.sendHeader.Del("Content-Length")
		}
	}

	// write status line and headers
	// Content-Length and Content-Type headers will be sent on call to Write(), if not set
	fmt.Fprintf(rw.conn, "HTTP/1.1 %d %s%s", rw.status, http.StatusText(rw.status), crlf)
	rw.sendHeader.Write(rw.conn)
	//rw.conn.Write(crlf)
	for k := range rw.sendHeader {
		rw.excludeHeaders[k] = true
	}
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func getHeader(h http.Header, key string) string {
	if v := h[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func (srv *Server) ListenAndServe() error {

	// Initialize the SCION/QUIC network connection
	if _, err := srv.initSCIONConnection(); err != nil {
		return err
	}

	li, err := squic.ListenSCION(nil, srv.Addr)
	defer li.Close()

	if err != nil {
		return fmt.Errorf("Failed to listen on %v: %v", srv.Addr, err)
	}

	for {
		// wait for new sessions
		sess, err := li.Accept()
		defer sess.Close(nil) // TODO: Why does it take an error?
		if err != nil {
			log.Printf("Failed to accept incoming connection: %v", err)
			continue
		}

		// get stream of session
		stream, err := sess.AcceptStream()
		if err != nil {
			log.Printf("Failed to accept incoming stream: %v", err)
			continue
		}

		// Handle connection
		go srv.handle(&quicconn.QuicConn{sess, stream})
	}
}

func (srv *Server) handle(conn net.Conn) {

	// read request
	req := request(conn)

	// respond to reques
	srv.respond(req, conn)
}

func (srv *Server) initSCIONConnection() (*snet.Addr, error) {

	log.Println("Initializing SCION connection")

	var err error
	srv.Addr, err = snet.AddrFromString(srv.AddrString)
	if err != nil {
		return nil, err
	}

	err = snet.Init(srv.Addr.IA, utils.GetSCIOND(), utils.GetDispatcher())
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize SCION network:", err)
	}

	log.Println("Initialized SCION network")

	err = squic.Init(srv.TLSKeyFile, srv.TLSCertFile)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize QUIC network:", err)
	}

	log.Println("Initialized SCION/QUIC network")

	return srv.Addr, nil

}

func request(conn net.Conn) *http.Request {

	br := bufio.NewReader(conn)

	req, err := http.ReadRequest(br)
	if err != nil {
		log.Println("failed to parse request")
	}
	return req
}

func (srv *Server) respond(req *http.Request, conn net.Conn) {
	rw := &RespWriter{
		contentLength:  -1,
		conn:           conn,
		sendHeader:     make(http.Header),
		handlerHeader:  make(http.Header),
		excludeHeaders: make(map[string]bool),
	}
	srv.Mux.ServeHTTP(rw, req)
	if !rw.headerFinalized {
		conn.Write(crlf)
	}
}
