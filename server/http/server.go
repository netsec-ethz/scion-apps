package shttp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/chaehni/scion-http/utils/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
)

type Server struct {
	AddrString String
	Addr       snet.Addr
}

func (srv *Server) ListenAndServe() {

	srv.initSCIONConnection()

	li, err := squic.ListenSCION(nil, srv.Addr)
	defer li.Close()

	if err != nil {
		log.Fatal("Failed to listen on %v: %v", srv.Addr, err)
	}

	for {
		// wait for new sessions
		sess, err := li.Accept()
		defer sess.Close()

		if err != nil {
			log.Printf(err.Error())
			continue
		}

		// get stream of session
		stream, err := sess.AcceptStream()
		if err != nil {
			log.Printf("Failed to accept incoming stream (%s)", err)
			continue
		}

		// Handle connection
		go handle(stream)
	}
}

// TODO: create dummy type that wraps session and adds missing methods to make it a net.Conn
func handle(conn net.Conn) {
	defer conn.Close()

	// read request
	request(conn)

	// respond to reques
	respond(conn)

}

func (srv *Server) initSCIONConnection(serverAddress, tlsCertFile, tlsKeyFile string) (*snet.Addr, error) {

	log.Println("Initializing SCION connection")

	Srv.Addr, err = snet.AddrFromString(srv.AddrString)
	if err != nil {
		return nil, err
	}

	err = snet.Init(srv.Addr.IA, utils.GetSciondAddr(srv.Addr), utils.GetDispatcherAddr(srv.Addr))
	if err != nil {
		return srv.Addr, err
	}

	err = squic.Init(tlsKeyFile, tlsCertFile)
	if err != nil {
		return Srv.Addr, err
	}

	return Srv.Addr, nil

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

func respond(conn net.Conn){
	body := `<!DOCTYPE html><html lang="en"><head><meta
		charset="UTF-8"><title>sample server</title></head>
		<body><strong>Hello World></strong></body></html>`
	
	fmt.Fprint(conn, "HTTP/1.1 200 OK\r\n")
	fmt.Fprintf(conn "Content-Length: %d\r\n", len(body))
	fmt.Fprint(conn, "Content-Type: text/html\r\n")
	fmt.Fprint(conn, "\r\n")
	fmt.Fprint(conn, body)
}
