// imagefetcher application
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/camerapp/README.md
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/netsec-ethz/scion-apps/lib/scionutil"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

const (
	maxRetries   int           = 4
	maxWaitDelay time.Duration = 3 * time.Second

	// Number of blocks that are simultaneously requested
	maxNumBlocksRequested               = 5
	blockSize             uint32        = 1000
	rttTimeoutMult        time.Duration = 3
	consecReqWaitTime     time.Duration = 500 * time.Microsecond
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func printUsage() {
	fmt.Println("imagefetcher -c ClientSCIONAddress -s ServerSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1011,[192.33.93.166]:42002")
	fmt.Println("ClientSCIONAddress can be omitted, the application then binds to localhost")
}

func fetchFileInfo(udpConnection snet.Conn) (string, uint32, time.Duration, error) {
	numRetries := 0
	packetBuffer := make([]byte, 2500)

	for numRetries < maxRetries {
		numRetries++
		// Send LIST command ("L") to server
		t0 := time.Now()
		_, err := udpConnection.Write([]byte("L"))
		check(err)

		// Read response
		err = udpConnection.SetReadDeadline(time.Now().Add(maxWaitDelay))
		check(err)
		n, _, err := udpConnection.ReadFrom(packetBuffer)
		if err != nil {
			// Read error, most likely Timeout
			continue
			// Uncomment and remove "continue" on previous line once the new version of snet is part of the SCIONLab branch
			// if operr, ok := err.(*snet.OpError); ok {
			// 	// This is an OpError, could be SCMP or Timeout, in both cases continue
			// 	if operr.Timeout() {
			// 		continue
			// 	}
			// 	if operr.SCMP() != nil {
			// 		continue
			// 	}
			// }
			// If it's not an snet Timeout or SCMP error, then it's something more serious and fail
			// check(err)
		}
		t1 := time.Now()
		rttApprox := t1.Sub(t0)

		if n < 2 {
			continue
		}
		if packetBuffer[0] != 'L' {
			continue
		}
		fileNameLen := int(packetBuffer[1])
		if 2+fileNameLen+4 != n {
			continue
		}
		fileName := string(packetBuffer[2 : fileNameLen+2])
		fileSize := binary.LittleEndian.Uint32(packetBuffer[fileNameLen+2:])

		// Remove deadline
		var tzero time.Time // initialized to "zero" time
		err = udpConnection.SetReadDeadline(tzero)
		check(err)
		return fileName, fileSize, rttApprox, nil
	}
	return "", 0, 0, fmt.Errorf("Error: could not obtain file information")
}

func blockFetcher(fetchBlockChan chan uint32, udpConnection snet.Conn, fileName string, fileSize uint32) {
	packetBuffer := make([]byte, 512)
	packetBuffer[0] = 'G'
	packetBuffer[1] = byte(len(fileName))
	copy(packetBuffer[2:], []byte(fileName))
	sendLen := 2 + len(fileName) + 8
	for i := range fetchBlockChan {
		binary.LittleEndian.PutUint32(packetBuffer[sendLen-8:], i)
		readLength := blockSize
		if i+readLength > fileSize {
			// Final block, read remaining amount
			readLength = fileSize - i
		}
		binary.LittleEndian.PutUint32(packetBuffer[sendLen-4:], i+readLength)
		_, err := udpConnection.Write(packetBuffer[:sendLen])
		check(err)
	}
}

func blockReceiver(receivedBlockChan chan uint32, udpConnection snet.Conn, fileBuffer []byte, fileSize uint32) {
	packetBuffer := make([]byte, 2500)
	for {
		n, _, err := udpConnection.ReadFrom(packetBuffer)
		if err != nil {
			continue
			// Uncomment and remove "continue" on previous line once the new version of snet is part of the SCIONLab branch
			// if operr, ok := err.(*snet.OpError); ok {
			// 	// This is an OpError, could be SCMP, in which case continue
			// 	if operr.SCMP() != nil {
			// 		continue
			// 	}
			// }
			// If it's not an snet SCMP error, then it's something more serious and fail
			// check(err)
		}
		if n < 10 {
			continue
		}
		if packetBuffer[0] != 'G' {
			continue
		}
		startByte := binary.LittleEndian.Uint32(packetBuffer[1:])
		endByte := binary.LittleEndian.Uint32(packetBuffer[5:])
		readLength := blockSize
		if startByte+readLength > fileSize {
			// Final block, read remaining amount
			readLength = fileSize - startByte
		}
		if uint32(n) != 9+readLength {
			continue
		}
		if endByte != startByte+readLength {
			continue
		}
		copy(fileBuffer[startByte:], packetBuffer[9:n])
		receivedBlockChan <- startByte
	}
}

func main() {
	startTime := time.Now()

	var (
		clientAddress  string
		serverAddress  string
		sciondPath     string
		sciondFromIA   bool
		dispatcherPath string
		outputFilePath string

		err    error
		local  *snet.Addr
		remote *snet.Addr

		udpConnection snet.Conn
	)

	flag.StringVar(&clientAddress, "c", "", "Client SCION Address")
	flag.StringVar(&serverAddress, "s", "", "Server SCION Address")
	flag.StringVar(&sciondPath, "sciond", "", "Path to sciond socket")
	flag.BoolVar(&sciondFromIA, "sciondFromIA", false, "SCIOND socket path from IA address:ISD-AS")
	flag.StringVar(&dispatcherPath, "dispatcher", "/run/shm/dispatcher/default.sock",
		"Path to dispatcher socket")
	flag.StringVar(&outputFilePath, "output", "", "Path to the output file")
	flag.Parse()

	// Create SCION UDP socket
	if len(clientAddress) > 0 {
		local, err = snet.AddrFromString(clientAddress)
	} else {
		local, err = scionutil.GetLocalhost()
	}
	check(err)

	if len(serverAddress) > 0 {
		remote, err = snet.AddrFromString(serverAddress)
		check(err)
	} else {
		printUsage()
		check(fmt.Errorf("Error, server address needs to be specified with -s"))
	}

	if sciondFromIA {
		if sciondPath != "" {
			log.Fatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		sciondPath = sciond.GetDefaultSCIONDPath(&local.IA)
	} else if sciondPath == "" {
		sciondPath = sciond.GetDefaultSCIONDPath(nil)
	}
	snet.Init(local.IA, sciondPath, reliable.NewDispatcherService(dispatcherPath))
	udpConnection, err = snet.DialSCION("udp4", local, remote)
	check(err)

	fileName, fileSize, rttApprox, err := fetchFileInfo(udpConnection)
	check(err)

	fetchBlockChan := make(chan uint32, 2)
	receivedBlockChan := make(chan uint32, 2)

	fileBuffer := make([]byte, fileSize)

	// Sends block fetch requests to image server
	go blockFetcher(fetchBlockChan, udpConnection, fileName, fileSize)

	// Receives arriving image blocks
	// Instead of implementation as a goroutine, it can also be implemented as socket read with a timeout.
	// In this approach, the control loop structure is quite clean.
	go blockReceiver(receivedBlockChan, udpConnection, fileBuffer, fileSize)

	// The list of already requested blocks for which no response has yet been received.
	// This is a map because the most common operation is insert and remove.
	// Iteration through all the elements is occurring on in the rare case of packet loss.
	requestedBlockMap := make(map[uint32]time.Time)

	i := uint32(0)
	numTimeouts := 0
	done := false
	for !done {
		waitDuration := rttTimeoutMult * rttApprox
		if len(requestedBlockMap) < maxNumBlocksRequested && i < fileSize {
			// We can fetch an additional block
			requestedBlockMap[i] = time.Now()
			fetchBlockChan <- i
			fmt.Print("r")
			i = i + blockSize
			if len(requestedBlockMap) < maxNumBlocksRequested {
				// If we can fetch yet one more additional block,
				// wait for a short amount of time before requesting the next block
				waitDuration = consecReqWaitTime
			}
		}
		// If a missing block has reached a timeout, then request it again.
		now := time.Now()
		for l, m := range requestedBlockMap {
			if now.Sub(m) > rttTimeoutMult*rttApprox {
				// Timeout expired, let's request it again
				fetchBlockChan <- l
				fmt.Print("T")
				requestedBlockMap[l] = now
			}
		}
		select {
		case k := <-receivedBlockChan:
			fmt.Print(".")
			numTimeouts = 0
			delete(requestedBlockMap, k)
			// Was this the last block?
			if i >= fileSize && len(requestedBlockMap) == 0 {
				done = true
			}
		case <-time.After(waitDuration):
			if waitDuration == consecReqWaitTime {
				// Do not include numTimeouts if it was a short waiting period between consecutive requests
				continue
			}
			numTimeouts++
			if numTimeouts > maxRetries {
				fmt.Println(requestedBlockMap)
				check(fmt.Errorf("Too many missing packets, aborting"))
			}
		}
	}

	// Write file to disk
	if outputFilePath == "" {
		outputFilePath = fileName
	}
	err = ioutil.WriteFile(outputFilePath, fileBuffer, 0600)
	check(err)
	fmt.Println("\nDone, exiting. Total duration", time.Now().Sub(startTime))
}
