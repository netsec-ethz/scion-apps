// Copyright 2020 ETH Zurich
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

// bwtestserver application
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/bwtester/README.md
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"
	"inet.af/netaddr"

	. "github.com/netsec-ethz/scion-apps/bwtester/bwtestlib"
	"github.com/netsec-ethz/scion-apps/pkg/pan"
)

const (
	resultExpiry = time.Minute
)

func main() {
	var listen pan.IPPortValue
	kingpin.Flag("listen", "Address to listen on").Default(":40002").SetValue(&listen)
	kingpin.Parse()

	err := runServer(listen.Get())
	Check(err)
}

func runServer(listen netaddr.IPPort) error {
	receivePacketBuffer := make([]byte, 2500)

	var currentBwtest string
	var currentBwtestFinish time.Time
	currentResult := make(chan BwtestResult)

	results := make(resultsMap)

	ccSelector := pan.NewDefaultReplySelector()
	ccConn, err := pan.ListenUDP(context.Background(), listen, ccSelector)
	if err != nil {
		return err
	}
	serverCCAddr := ccConn.LocalAddr().(pan.UDPAddr)
	for {
		// Handle client requests
		n, clientCCAddr, err := ccConn.ReadFrom(receivePacketBuffer)
		if err != nil {
			return err
		}
		request := receivePacketBuffer[:n]
		if n < 1 || (request[0] != 'N' && request[0] != 'R') {
			continue
		}

		// Check (non-blocking) for result from test running in background:
		select {
		case res := <-currentResult:
			results.insert(currentBwtest, res)
			currentBwtest = ""
			currentBwtestFinish = time.Time{}
		default:
		}

		clientCCAddrStr := clientCCAddr.String()
		fmt.Println("Received request:", string(request[0]), clientCCAddrStr)

		if request[0] == 'N' {
			// New bwtest request
			if len(currentBwtest) != 0 {
				fmt.Println("A bwtest is already ongoing", currentBwtest)
				if clientCCAddrStr == currentBwtest {
					// The request is from the same client for which the current test is already ongoing
					// If the response packet was dropped, then the client would send another request
					// We simply send another response packet, indicating success
					fmt.Println("clientCCAddrStr == currentBwtest")
					writeResponseN(ccConn, clientCCAddr, 0)
				} else {
					// A bwtest is currently ongoing, so send back remaining duration
					writeResponseN(ccConn, clientCCAddr, retryWaitTime(currentBwtestFinish))
				}
				continue
			}

			clientBwp, serverBwp, err := decodeRequestN(request)
			if err != nil {
				continue
			}
			path := ccSelector.Path(clientCCAddr.(pan.UDPAddr))
			finishTime, err := startBwtestBackground(serverCCAddr, clientCCAddr.(pan.UDPAddr), path,
				clientBwp, serverBwp, currentResult)
			if err != nil {
				// Ask the client to try again in 1 second
				writeResponseN(ccConn, clientCCAddr, 1)
				continue
			}
			currentBwtest = clientCCAddrStr
			currentBwtestFinish = finishTime
			// Send back success
			writeResponseN(ccConn, clientCCAddr, 0)
		} else if request[0] == 'R' {
			if clientCCAddrStr == currentBwtest {
				// test is still ongoing, send back remaining duration
				writeResponseR(ccConn, clientCCAddr, retryWaitTime(currentBwtestFinish), nil)
				continue
			}

			v, ok := results[clientCCAddrStr]
			if !ok || !bytes.Equal(v.PrgKey, request[1:]) {
				// There are no results for this client or incorrect PRG, return an error
				writeResponseR(ccConn, clientCCAddr, 127, nil)
				continue
			}
			writeResponseR(ccConn, clientCCAddr, 0, &v.BwtestResult)
		}
	}
}

// startBwtestBackground starts a bandwidth test, in the background.
// Returns the expected finish time of the test, or any error during the setup.
func startBwtestBackground(serverCCAddr pan.UDPAddr, clientCCAddr pan.UDPAddr,
	path *pan.Path, clientBwp, serverBwp BwtestParameters, res chan<- BwtestResult) (time.Time, error) {

	// Data Connection addresses:
	clientDCAddr := clientCCAddr
	clientDCAddr.Port = clientBwp.Port
	serverDCAddr := netaddr.IPPortFrom(serverCCAddr.IP, serverBwp.Port)

	// Open Data Connection
	dcSelector := initializedReplySelector(clientDCAddr, path)
	dcConn, err := listenConnected(serverDCAddr, clientDCAddr, dcSelector)
	if err != nil {
		return time.Time{}, err
	}

	now := time.Now()
	finishTimeSend := now.Add(serverBwp.BwtestDuration + GracePeriodSend)
	finishTimeReceive := now.Add(clientBwp.BwtestDuration + StragglerWaitPeriod)
	finishTime := finishTimeReceive
	if finishTime.Before(finishTimeSend) {
		finishTime = finishTimeSend
	}
	if err := dcConn.SetReadDeadline(finishTimeReceive); err != nil {
		dcConn.Close()
		return time.Time{}, err
	}
	if err := dcConn.SetWriteDeadline(finishTimeSend); err != nil {
		dcConn.Close()
		return time.Time{}, err
	}

	sendDone := make(chan struct{})
	go func() {
		_ = HandleDCConnSend(serverBwp, dcConn)
		close(sendDone)
	}()
	go func() {
		r := HandleDCConnReceive(clientBwp, dcConn)
		<-sendDone
		dcConn.Close()
		res <- r
	}()
	return finishTime, nil
}

// writeResponseN writes the response to an 'N' (new bandwidth test) request.
// The waitTime field is
//  - 0:   Ok, the test starts immediately
//  - N>0: please try again in N seconds
func writeResponseN(ccConn net.PacketConn, addr net.Addr, waitTime byte) {
	var response [2]byte
	response[0] = 'N'
	response[1] = waitTime
	_, _ = ccConn.WriteTo(response[:], addr)
}

// writeResponseN writes the response to an 'R' (fetch results) request.
// The code field is
//  - 0:   Ok, the rest of the response is the encoded result
//  - N>0: please try again in N seconds
//  - 127: error, go away (why 127? I guess we have 7-bit bytes or something...)
func writeResponseR(ccConn net.PacketConn, addr net.Addr, code byte, res *BwtestResult) {
	response := make([]byte, 2000)
	response[0] = 'R'
	response[1] = code
	n := 0
	if res != nil {
		n, _ = EncodeBwtestResult(*res, response[2:])
	}
	_, _ = ccConn.WriteTo(response[:2+n], addr)
}

// retryWaitTime gives back the "encoded" number of seconds for a client to wait until t.
// Clips to at least 1 second wait time, even if t is closer or in the past.
func retryWaitTime(t time.Time) byte {
	remTime := time.Until(t)
	// Ensure non-negative; should already have finished, but apparently hasn't.
	if remTime < 0 {
		remTime = 0
	}
	return byte(remTime/time.Second) + 1
}

// decodeRequestN decodes and checks the bandwidth test parameters contained in
// an 'N' (new bandwidth test) request.
func decodeRequestN(request []byte) (clientBwp, serverBwp BwtestParameters, err error) {
	clientBwp, n1, err := DecodeBwtestParameters(request[1:])
	if err != nil {
		err = fmt.Errorf("decoding client->server parameters: %w", err)
		return
	}
	if err = validateBwtestParameters(clientBwp); err != nil {
		err = fmt.Errorf("invalid client->server parameters: %w", err)
		return
	}
	serverBwp, n2, err := DecodeBwtestParameters(request[n1+1:])
	if err != nil {
		err = fmt.Errorf("decoding server->client parameters: %w", err)
		return
	}
	if err = validateBwtestParameters(serverBwp); err != nil {
		err = fmt.Errorf("invalid server->client parameters: %w", err)
		return
	}
	if len(request) != 1+n1+n2 {
		err = errors.New("packet size incorrect")
	}
	return
}

func validateBwtestParameters(bwp BwtestParameters) error {
	if bwp.BwtestDuration > MaxDuration {
		return fmt.Errorf("duration exceeds max: %s > %s", bwp.BwtestDuration, MaxDuration)
	}
	if bwp.PacketSize < MinPacketSize {
		return fmt.Errorf("packet size too small: %d < %d", bwp.PacketSize, MinPacketSize)
	}
	if bwp.PacketSize > MaxPacketSize {
		return fmt.Errorf("packet size exceeds max: %d > %d", bwp.PacketSize, MaxPacketSize)
	}
	if bwp.Port < MinPort {
		return fmt.Errorf("invalid port: %d", bwp.Port)
	}
	if len(bwp.PrgKey) != 16 {
		return fmt.Errorf("invalid key size: %d != 16", len(bwp.PrgKey))
	}
	return nil
}

type bwtestResultWithExpiry struct {
	BwtestResult
	Expiry time.Time
}

type resultsMap map[string]bwtestResultWithExpiry

func (r resultsMap) insert(client string, res BwtestResult) {
	r.purgeExpired()
	r[client] = bwtestResultWithExpiry{
		BwtestResult: res,
		Expiry:       time.Now().Add(resultExpiry),
	}
}

func (r resultsMap) purgeExpired() {
	now := time.Now()
	for k, v := range r {
		if v.Expiry.Before(now) {
			delete(r, k)
		}
	}
}

func listenConnected(local netaddr.IPPort, remote pan.UDPAddr, selector pan.ReplySelector) (net.Conn, error) {
	conn, err := pan.ListenUDP(context.Background(), local, selector)
	return connectedPacketConn{
		ListenConn: conn,
		remote:     remote,
	}, err
}

// connectedPacketConn connects a net.PacketConn to a fixed remote address,
// i.e. it uses the fixed remote address to wrap the ReadFrom/WriteTo of a
// net.PacketConn into Read/Write of a net.Conn.
type connectedPacketConn struct {
	pan.ListenConn
	remote pan.UDPAddr
}

func (c connectedPacketConn) Read(buf []byte) (int, error) {
	for {
		n, addr, err := c.ListenConn.ReadFrom(buf)
		if err != nil {
			return n, err
		}
		if c.remote != addr.(pan.UDPAddr) {
			continue
		}
		return n, err
	}
}

func (c connectedPacketConn) Write(buf []byte) (int, error) {
	return c.ListenConn.WriteTo(buf, c.remote)
}

func (c connectedPacketConn) RemoteAddr() net.Addr {
	return c.remote
}

// initializedReplySelector creates a pan.DefaultReplySelector, initialized with path for dst.
func initializedReplySelector(remote pan.UDPAddr, path *pan.Path) pan.ReplySelector {
	if path != nil && path.Destination != remote.IA {
		panic("path destination should match address")
	}
	selector := pan.NewDefaultReplySelector()
	selector.Record(remote, path)
	return selector
}
