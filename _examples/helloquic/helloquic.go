// Copyright 2021 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/lucas-clemente/quic-go"
	"inet.af/netaddr"

	"github.com/netsec-ethz/scion-apps/pkg/pan"
	"github.com/netsec-ethz/scion-apps/pkg/quicutil"
)

func main() {
	var err error
	// get local and remote addresses from program arguments:
	var listen pan.IPPortValue
	flag.Var(&listen, "listen", "[Server] local IP:port to listen on")
	remoteAddr := flag.String("remote", "", "[Client] Remote (i.e. the server's) SCION Address (e.g. 17-ffaa:1:1,[127.0.0.1]:12345)")
	count := flag.Uint("count", 1, "[Client] Number of messages to send")
	flag.Parse()

	if (listen.Get().Port() > 0) == (len(*remoteAddr) > 0) {
		check(fmt.Errorf("Either specify -port for server or -remote for client"))
	}

	if listen.Get().Port() > 0 {
		err = runServer(listen.Get())
		check(err)
	} else {
		err = runClient(*remoteAddr, int(*count))
		check(err)
	}
}

func runServer(listen netaddr.IPPort) error {
	tlsCfg := &tls.Config{
		Certificates: quicutil.MustGenerateSelfSignedCert(),
		NextProtos:   []string{"hello-quic"},
	}
	listener, err := pan.ListenQUIC(context.Background(), listen, nil, tlsCfg, nil)
	if err != nil {
		return err
	}
	defer listener.Close()
	fmt.Println(listener.Addr())

	for {
		session, err := listener.Accept(context.Background())
		if err != nil {
			return err
		}
		fmt.Println("New session", session.RemoteAddr())
		go func() {
			err := workSession(session)
			if err != nil && !errors.Is(err, &quic.ApplicationError{}) {
				fmt.Println("Error in session", session.RemoteAddr(), err)
			}
		}()
	}
}

func workSession(session quic.Session) error {
	for {
		stream, err := session.AcceptStream(context.Background())
		if err != nil {
			return err
		}
		defer stream.Close()
		data, err := ioutil.ReadAll(stream)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", data)
		_, err = stream.Write([]byte("gotcha: "))
		_, err = stream.Write(data)
		if err != nil {
			return err
		}
		stream.Close()
	}
}

func runClient(address string, count int) error {
	addr, err := pan.ResolveUDPAddr(address)
	if err != nil {
		return err
	}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"hello-quic"},
	}
	// Set Pinging Selector with active probing on two paths
	selector := &pan.PingingSelector{
		Interval: 2 * time.Second,
		Timeout:  time.Second,
	}
	selector.SetActive(2)
	session, err := pan.DialQUIC(context.Background(), netaddr.IPPort{}, addr, nil, selector, "", tlsCfg, nil)
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		stream, err := session.OpenStream()
		if err != nil {
			return err
		}
		_, err = stream.Write([]byte(fmt.Sprintf("hi dude, %d", i)))
		if err != nil {
			return err
		}
		stream.Close()
		reply, err := ioutil.ReadAll(stream)
		fmt.Printf("%s\n", reply)
	}
	session.CloseWithError(quic.ApplicationErrorCode(0), "")
	return nil
}

// Check just ensures the error is nil, or complains and quits
func check(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, "Fatal error:", e)
		os.Exit(1)
	}
}
