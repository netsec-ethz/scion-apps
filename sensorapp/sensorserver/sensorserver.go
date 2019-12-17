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
	TIMESTRING             string = "Time"
	TIMEFORMAT             string = "2006/01/02 15:04:05"
	SEPARATORSTRING        string = ": "
	TIMEANDSEPARATORSTRING string = TIMESTRING + SEPARATORSTRING
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
		index := strings.Index(line, TIMEANDSEPARATORSTRING)
		if index == 0 {
			// We found a time string, format in case parsing is desired: 2017/11/16 21:29:49
			timestr := line[len(TIMEANDSEPARATORSTRING):]
			sensorDataLock.Lock()
			sensorData[TIMESTRING] = timestr
			sensorDataLock.Unlock()
			continue
		}
		index = strings.Index(line, SEPARATORSTRING)
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
		var timeString string
		sensorDataLock.Lock()
		for k, v := range sensorData {
			if strings.Index(k, TIMESTRING) == 0 {
				timeString = v
			} else {
				sensorValues = sensorValues + v + "\n"
			}
		}
		sensorDataLock.Unlock()
		sensorValues = timeString + "\n" + sensorValues
		copy(sendPacketBuffer, sensorValues)

		_, err = conn.WriteTo(sendPacketBuffer[:len(sensorValues)], clientAddress)
		check(err)
	}
}
