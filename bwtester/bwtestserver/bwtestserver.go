// bwtestserver application
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/bwtester/README.md
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/kormat/fmt15"

	. "github.com/netsec-ethz/scion-apps/bwtester/bwtestlib"
	"github.com/netsec-ethz/scion-apps/pkg/appnet"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/snet"
)

var (
	resultsMap     map[string]*BwtestResult
	resultsMapLock sync.Mutex
	currentBwtest  string // Contains connection parameters, in case server's ack packet was lost
)

// Deletes the old entries in resultsMap
func purgeOldResults() {
	for {
		time.Sleep(time.Minute * time.Duration(5))
		resultsMapLock.Lock()
		// Erase entries that are older than 1 minute
		t := time.Now().Add(-time.Minute)
		for k, v := range resultsMap {
			if v.ExpectedFinishTime.Before(t) {
				delete(resultsMap, k)
			}
		}
		resultsMapLock.Unlock()
	}
}

func main() {
	resultsMap = make(map[string]*BwtestResult)
	go purgeOldResults()

	// Fetch arguments from command line
	serverPort := flag.Uint("p", 40002, "Port")
	id := flag.String("id", "bwtester", "Element ID")
	logDir := flag.String("log_dir", "./logs", "Log directory")

	flag.Parse()

	// Setup logging
	if _, err := os.Stat(*logDir); os.IsNotExist(err) {
		err := os.Mkdir(*logDir, 0744)
		if err != nil {
			LogFatal("Unable to create log dir", "err", err)
		}
	}
	log.Root().SetHandler(log.MultiHandler(
		log.LvlFilterHandler(log.LvlDebug,
			log.StreamHandler(os.Stderr, fmt15.Fmt15Format(fmt15.ColorMap))),
		log.LvlFilterHandler(log.LvlDebug,
			log.Must.FileHandler(fmt.Sprintf("%s/%s.log", *logDir, *id),
				fmt15.Fmt15Format(nil)))))

	err := runServer(uint16(*serverPort))
	if err != nil {
		LogFatal("Unable to start server", "err", err)
	}
}

func runServer(port uint16) error {

	conn, err := appnet.ListenPort(port)
	if err != nil {
		return err
	}

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	handleClients(conn, receivePacketBuffer, sendPacketBuffer)
	return nil
}

func handleClients(CCConn snet.Conn, receivePacketBuffer []byte, sendPacketBuffer []byte) {

	for {
		// Handle client requests
		n, fromAddr, err := CCConn.ReadFrom(receivePacketBuffer)
		clientCCAddr := fromAddr.(*snet.UDPAddr)
		if err != nil {
			// Todo: check error in detail, but for now simply continue
			continue
		}
		if n < 1 {
			continue
		}

		t := time.Now()
		// Check if a current test is ongoing, and if it completed
		if len(currentBwtest) > 0 {
			v, ok := resultsMap[currentBwtest]
			if !ok {
				// This can only happen if client aborted and never picked up results
				// then information got removed by purgeOldResults goroutine
				currentBwtest = ""
			} else if t.After(v.ExpectedFinishTime) {
				// The bwtest should be finished by now, check if results are written
				if v.NumPacketsReceived >= 0 {
					// Indeed, the bwtest has completed
					currentBwtest = ""
				}
			}
		}
		clientCCAddrStr := clientCCAddr.String()
		fmt.Println("Received request:", clientCCAddrStr)

		if receivePacketBuffer[0] == 'N' {
			// New bwtest request
			if len(currentBwtest) != 0 {
				fmt.Println("A bwtest is already ongoing")
				if clientCCAddrStr == currentBwtest {
					// The request is from the same client for which the current test is already ongoing
					// If the response packet was dropped, then the client would send another request
					// We simply send another response packet, indicating success
					fmt.Println("error, clientCCAddrStr == currentBwtest")
					sendPacketBuffer[0] = 'N'
					sendPacketBuffer[1] = byte(0)
					_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
					// Ignore error
					continue
				}

				// The request is from a different client
				// A bwtest is currently ongoing, so send back remaining duration
				resultsMapLock.Lock()
				v, ok := resultsMap[currentBwtest]
				if !ok {
					// This should never happen
					resultsMapLock.Unlock()
					continue
				}
				resultsMapLock.Unlock()

				// Compute for how much longer the current test is running
				remTime := t.Sub(v.ExpectedFinishTime)
				sendPacketBuffer[0] = 'N'
				sendPacketBuffer[1] = byte(remTime/time.Second) + 1
				_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
				// Ignore error
				continue
			}

			// This is a new request
			clientBwp, n1, err := DecodeBwtestParameters(receivePacketBuffer[1:])
			if err != nil {
				fmt.Println("Decoding error")
				// Decoding error, continue
				continue
			}
			serverBwp, n2, err := DecodeBwtestParameters(receivePacketBuffer[n1+1:])
			if err != nil {
				fmt.Println("Decoding error")
				// Decoding error, continue
				continue
			}
			if n != 1+n1+n2 {
				fmt.Println("Error, packet size incorrect")
				// Do not send a response packet for malformed request
				continue
			}

			// Address of client Data Connection (DC)
			clientDCAddr := clientCCAddr.Copy()
			clientDCAddr.Host.Port = int(clientBwp.Port)

			// Address of server Data Connection (DC)
			serverCCAddr := CCConn.LocalAddr().(*net.UDPAddr)
			serverDCAddr := &net.UDPAddr{IP: serverCCAddr.IP, Port: int(serverBwp.Port)}

			// Open Data Connection
			DCConn, err := appnet.DefNetwork().Dial(
				context.TODO(), "udp", serverDCAddr, clientDCAddr, addr.SvcNone)
			if err != nil {
				// An error happened, ask the client to try again in 1 second
				sendPacketBuffer[0] = 'N'
				sendPacketBuffer[1] = byte(1)
				_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
				// Ignore error
				continue
			}

			// Nothing needs to be added to account for network delay, since sending starts right away
			expFinishTimeSend := t.Add(serverBwp.BwtestDuration + GracePeriodSend)
			expFinishTimeReceive := t.Add(clientBwp.BwtestDuration + StragglerWaitPeriod)
			// We use resultsMapLock also for the bres variable
			bres := BwtestResult{
				NumPacketsReceived: -1,
				CorrectlyReceived:  -1,
				IPAvar:             -1,
				IPAmin:             -1,
				IPAavg:             -1,
				IPAmax:             -1,
				PrgKey:             clientBwp.PrgKey,
				ExpectedFinishTime: expFinishTimeReceive,
			}
			if expFinishTimeReceive.Before(expFinishTimeSend) {
				// The receiver will close the DC connection, so it will wait long enough until the
				// sender is also done
				bres.ExpectedFinishTime = expFinishTimeSend
			}
			resultsMapLock.Lock()
			resultsMap[clientCCAddrStr] = &bres
			resultsMapLock.Unlock()

			// go HandleDCConnReceive(clientBwp, DCConn, resChan)
			go HandleDCConnReceive(clientBwp, DCConn, &bres, &resultsMapLock, nil)
			go HandleDCConnSend(serverBwp, DCConn)

			// Send back success
			sendPacketBuffer[0] = 'N'
			sendPacketBuffer[1] = byte(0)
			_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
			// Ignore error
			// Everything succeeded, now set variable that bwtest is ongoing
			currentBwtest = clientCCAddrStr
		} else if receivePacketBuffer[0] == 'R' {
			// This is a request for the results
			sendPacketBuffer[0] = 'R'
			// Make sure that the client is known and that the results are ready
			v, ok := resultsMap[clientCCAddrStr]
			if !ok {
				// There are no results for this client, return an error
				sendPacketBuffer[1] = byte(127)
				_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
				continue
			}
			// Make sure the PRG key is correct
			if n != 1+len(v.PrgKey) || !bytes.Equal(v.PrgKey, receivePacketBuffer[1:1+len(v.PrgKey)]) {
				// Error, the sent PRG is incorrect
				sendPacketBuffer[1] = byte(127)
				_, _ = CCConn.WriteTo(sendPacketBuffer[:2], clientCCAddr)
				continue
			}
			// Note: it would be better to have the resultsMap key consist only of the PRG key,
			// so that a repeated bwtest from the same client with the same port gets a
			// different resultsMap entry. However, in practice, a client would not run concurrent
			// bwtests, as long as the results are fetched before a new bwtest is initiated, this
			// code will work fine.
			if v.NumPacketsReceived == -1 {
				// The results are not yet ready
				if t.After(v.ExpectedFinishTime) {
					// The results should be ready, but are not yet written into the data
					// structure, so let's let client wait for 1 second
					sendPacketBuffer[1] = byte(1)
				} else {
					sendPacketBuffer[1] = byte(v.ExpectedFinishTime.Sub(t)/time.Second) + 1
				}
				_, _ = CCConn.WriteTo(sendPacketBuffer[:n], clientCCAddr)
				continue
			}
			sendPacketBuffer[1] = byte(0)
			n = EncodeBwtestResult(v, sendPacketBuffer[2:])
			_, _ = CCConn.WriteTo(sendPacketBuffer[:n+2], clientCCAddr)
		}
	}
}
