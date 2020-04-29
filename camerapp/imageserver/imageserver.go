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

// imageserver application. This simple image server sends images via a series of UDP requests.
// For more documentation on the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
// https://github.com/netsec-ethz/scion-apps/blob/master/camerapp/README.md
package main

import (
	"encoding/binary"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

const (
	// MaxFileNameLength is the max acceptable length for file names of served images
	MaxFileNameLength int = 255

	// MaxFileAge defines that after an image was stored for this amount of time,
	// it will be deleted
	MaxFileAge time.Duration = time.Minute * 10

	// MaxFileAgeGracePeriod defines the duration after which an image is still
	// available for download, but it will not be listed any more in new requests
	MaxFileAgeGracePeriod time.Duration = time.Minute * 1

	// Interval after which the file system is read to check for new images
	imageReadInterval time.Duration = time.Second * 59
)

type imageFileType struct {
	name     string
	size     uint32
	content  []byte
	readTime time.Time
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var (
	currentFiles     map[string]*imageFileType
	mostRecentFile   string
	currentFilesLock sync.Mutex
)

func handleImageFiles() {
	for {
		// Read the directory and look for new .jpg images
		direntries, err := ioutil.ReadDir(".")
		check(err)

		for _, entry := range direntries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".jpg") {
				continue
			}
			if len(entry.Name()) > MaxFileNameLength {
				continue
			}
			// Check if we've already read in the image
			currentFilesLock.Lock()
			if _, ok := currentFiles[entry.Name()]; !ok {
				fileContents, err := ioutil.ReadFile(entry.Name())
				check(err)
				newFile := imageFileType{entry.Name(), uint32(entry.Size()), fileContents, time.Now()}
				currentFiles[newFile.name] = &newFile
				mostRecentFile = newFile.name
			}
			currentFilesLock.Unlock()
		}
		// Check if an image should be deleted
		now := time.Now()
		currentFilesLock.Lock()
		for k, v := range currentFiles {
			if now.Sub(v.readTime) > MaxFileAge+MaxFileAgeGracePeriod {
				err = os.Remove(k)
				check(err)
				delete(currentFiles, k)
				if k == mostRecentFile {
					mostRecentFile = ""
				}
			}
		}
		currentFilesLock.Unlock()

		time.Sleep(imageReadInterval)
	}
}

func main() {
	currentFiles = make(map[string]*imageFileType)

	// Fetch arguments from command line
	port := flag.Uint("p", 40002, "Server Port")
	flag.Parse()

	udpConnection, err := appnet.ListenPort(uint16(*port))
	check(err)

	go handleImageFiles()

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		// Handle client requests
		n, remoteUDPaddress, err := udpConnection.ReadFrom(receivePacketBuffer)
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
		if n > 0 {
			if receivePacketBuffer[0] == 'L' {
				// We also need to lock access to mostRecentFile, otherwise a race condition is possible
				// where the file is deleted after the initial check
				currentFilesLock.Lock()
				sendLen := len(mostRecentFile)
				if sendLen == 0 {
					currentFilesLock.Unlock()
					continue
				}
				sendPacketBuffer[0] = 'L'
				sendPacketBuffer[1] = byte(sendLen)
				copy(sendPacketBuffer[2:], []byte(mostRecentFile))
				sendLen = sendLen + 2
				binary.LittleEndian.PutUint32(sendPacketBuffer[sendLen:], currentFiles[mostRecentFile].size)
				currentFilesLock.Unlock()
				sendLen = sendLen + 4
				_, err = udpConnection.WriteTo(sendPacketBuffer[:sendLen], remoteUDPaddress)
				check(err)
			} else if receivePacketBuffer[0] == 'G' && n > 1 {
				filenameLen := int(receivePacketBuffer[1])
				if n >= (2 + filenameLen + 8) {
					currentFilesLock.Lock()
					v, ok := currentFiles[string(receivePacketBuffer[2:filenameLen+2])]
					// We don't need to lock any more, since we now have a pointer to the image structure
					// which does not get changed once set up.
					currentFilesLock.Unlock()
					if !ok {
						continue
					}
					startByte := binary.LittleEndian.Uint32(receivePacketBuffer[filenameLen+2:])
					endByte := binary.LittleEndian.Uint32(receivePacketBuffer[filenameLen+6:])
					if endByte > startByte && endByte <= v.size+1 {
						sendPacketBuffer[0] = 'G'
						// Copy startByte and endByte from request packet
						copy(sendPacketBuffer[1:], receivePacketBuffer[filenameLen+2:filenameLen+10])
						// Copy image contents
						copy(sendPacketBuffer[9:], v.content[startByte:endByte])
						sendLen := 9 + endByte - startByte
						_, err = udpConnection.WriteTo(sendPacketBuffer[:sendLen], remoteUDPaddress)
						check(err)
					}
				}
			}
		}
	}
}
