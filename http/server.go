package shttp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/chaehni/scion-http/quicconn"
	"github.com/chaehni/scion-http/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Server struct {
	AddrString  string
	Addr        *snet.Addr
	TLSCertFile string
	TLSKeyFile  string
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
		defer sess.Close(nil) // Why does it take an error?
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
		go handle(&quicconn.QuicConn{sess, stream})
	}
}

func handle(conn net.Conn) {
	//defer conn.Close()

	// read request
	request(conn)

	// respond to reques
	respond(conn)

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

func request(conn net.Conn) {
	i := 0
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		ln := scanner.Text()
		fmt.Println(ln)
		if i == 0 {
			// request line
			m := strings.Fields(ln)[0]
			fmt.Println("***METHOD", m)
		}
		if ln == "" {
			// headers are done
			break
		}
		i++
	}
}

func respond(conn net.Conn) {
	body := `<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>sample server</title></head><body><strong>Hello World></strong></body></html>`

	fmt.Fprint(conn, "HTTP/1.1 200 OK\r\n")
	fmt.Fprintf(conn, "Content-Length: %d\r\n", len(body))
	fmt.Fprint(conn, "Content-Type: text/html\r\n")
	fmt.Fprint(conn, "\r\n")
	fmt.Fprint(conn, body)
}
