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

// sensorserver application
// For documentation on how to setup and run the application see:
// https://github.com/netsec-ethz/scion-apps/blob/master/README.md
package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/netsec-ethz/scion-apps/pkg/appnet"
)

const (
	timeString             string = "Time"
	separatorString        string = ": "
	timeAndSeparatorString string = timeString + separatorString
)

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var sensorData map[string]string
var sensorDataLock sync.Mutex

func init() {
	sensorData = make(map[string]string)
}

// Obtains input from sensor observation application
func parseInput() {
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		line := input.Text()
		index := strings.Index(line, timeAndSeparatorString)
		if index == 0 {
			// We found a time string, format in case parsing is desired: 2017/11/16 21:29:49
			timestr := line[len(timeAndSeparatorString):]
			sensorDataLock.Lock()
			sensorData[timeString] = timestr
			sensorDataLock.Unlock()
			continue
		}
		index = strings.Index(line, separatorString)
		if index > 0 {
			sensorType := line[:index]
			sensorDataLock.Lock()
			sensorData[sensorType] = line
			sensorDataLock.Unlock()
		}
	}
}

func main() {
	go parseInput()

	// Fetch arguments from command line
	port := flag.Uint("p", 40002, "Server Port")
	flag.Parse()

	conn, err := appnet.ListenPort(uint16(*port))
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		_, clientAddress, err := conn.ReadFrom(receivePacketBuffer)
		check(err)

		// Packet received, send back response to same client
		var sensorValues string
		var timeStr string
		sensorDataLock.Lock()
		for k, v := range sensorData {
			if strings.Index(k, timeString) == 0 {
				timeStr = v
			} else {
				sensorValues = sensorValues + v + "\n"
			}
		}
		sensorDataLock.Unlock()
		sensorValues = timeStr + "\n" + sensorValues
		copy(sendPacketBuffer, sensorValues)

		_, err = conn.WriteTo(sendPacketBuffer[:len(sensorValues)], clientAddress)
		check(err)
	}
}
