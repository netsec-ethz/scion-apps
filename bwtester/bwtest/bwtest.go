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

// package bwtest contains the definitions shared between bwtestserver and
// bwtestclient.
package bwtest

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

const (
	// Maximum duration of a bandwidth test
	MaxDuration time.Duration = time.Minute * 5
	// Maximum amount of time to wait for straggler packets
	StragglerWaitPeriod time.Duration = time.Second
	// Allow sending beyond the finish time by this amount
	GracePeriodSend time.Duration = time.Millisecond * 10
	// Min packet size is 4 bytes, so that 32-bit integer fits in
	// Ideally packet size > 4 bytes, so that part of the PRG is also in packet
	MinPacketSize int64 = 4
	// Max packet size to avoid allocation of too large a buffer, make it large enough for jumbo frames++
	MaxPacketSize int64 = 66000
	// Make sure the port number is a port the server application can connect to
	MinPort uint16 = 1024
)

type Parameters struct {
	BwtestDuration time.Duration
	PacketSize     int64
	NumPackets     int64
	PrgKey         []byte
	Port           uint16
}

type Result struct {
	NumPacketsReceived int64
	CorrectlyReceived  int64
	IPAvar             int64
	IPAmin             int64
	IPAavg             int64
	IPAmax             int64
	// Contains the client's sending PRG key, so that the result can be uniquely identified
	// Only requests that contain the correct key can obtain the result
	PrgKey []byte
}

func Check(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, "Fatal:", e)
		os.Exit(1)
	}
}

type prgFiller struct {
	aes cipher.Block
	buf []byte
}

func newPrgFiller(key []byte) *prgFiller {
	aesCipher, err := aes.NewCipher(key)
	Check(err)
	return &prgFiller{
		aes: aesCipher,
		buf: make([]byte, aes.BlockSize),
	}
}

// Fill the buffer with AES PRG in counter mode
// The value of the ith 16-byte block is simply an encryption of i under the key
func (f *prgFiller) Fill(iv int, data []byte) {
	memzero(f.buf)
	i := uint32(iv)
	j := 0
	for j <= len(data)-aes.BlockSize {
		binary.LittleEndian.PutUint32(f.buf, i)
		f.aes.Encrypt(data, f.buf) // BUG(matzf): this should be data[j:]! data is mostly left zero.
		j = j + aes.BlockSize
		i = i + uint32(aes.BlockSize)
	}
	// Check if fewer than BlockSize bytes are required for the final block
	if j < len(data) {
		binary.LittleEndian.PutUint32(f.buf, i)
		f.aes.Encrypt(f.buf, f.buf)
		copy(data[j:], f.buf[:len(data)-j])
	}
}

func memzero(buf []byte) {
	for i := range buf {
		buf[i] = 0
	}
}

// Encode Result into a sufficiently large byte buffer that is passed in, return the number of bytes written
func EncodeResult(res Result, buf []byte) (int, error) {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(res)
	copy(buf, bb.Bytes())
	return bb.Len(), err
}

// Decode Result from byte buffer that is passed in, returns Result structure and number of bytes consumed
func DecodeResult(buf []byte) (Result, int, error) {
	bb := bytes.NewBuffer(buf)
	is := bb.Len()
	dec := gob.NewDecoder(bb)
	var v Result
	err := dec.Decode(&v)
	return v, is - bb.Len(), err
}

// Encode Parameters into a sufficiently large byte buffer that is passed in, return the number of bytes written
func EncodeParameters(bwtp Parameters, buf []byte) (int, error) {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(bwtp)
	copy(buf, bb.Bytes())
	return bb.Len(), err
}

// Decode Parameters from byte buffer that is passed in, returns BwtestParameters structure and number of bytes consumed
func DecodeParameters(buf []byte) (Parameters, int, error) {
	bb := bytes.NewBuffer(buf)
	is := bb.Len()
	dec := gob.NewDecoder(bb)
	var v Parameters
	err := dec.Decode(&v)
	return v, is - bb.Len(), err
}

func HandleDCConnSend(bwp Parameters, udpConnection io.Writer) error {
	sb := make([]byte, bwp.PacketSize)
	t0 := time.Now()
	interPktInterval := bwp.BwtestDuration
	if bwp.NumPackets > 1 {
		interPktInterval = bwp.BwtestDuration / time.Duration(bwp.NumPackets-1)
	}
	prgFiller := newPrgFiller(bwp.PrgKey)
	for i := int64(0); i < bwp.NumPackets; i++ {
		time.Sleep(time.Until(t0.Add(interPktInterval * time.Duration(i))))
		// Send packet now
		prgFiller.Fill(int(i*bwp.PacketSize), sb)
		// Place packet number at the beginning of the packet, overwriting some PRG data
		binary.LittleEndian.PutUint32(sb, uint32(i*bwp.PacketSize))
		_, err := udpConnection.Write(sb)
		if err != nil {
			return err
		}
	}
	return nil
}

func HandleDCConnReceive(bwp Parameters, udpConnection io.Reader) Result {
	var numPacketsReceived int64
	var correctlyReceived int64
	interPacketArrivalTime := make(map[int]int64, bwp.NumPackets)

	// Make the receive buffer a bit larger to enable detection of packets that are too large
	recBuf := make([]byte, bwp.PacketSize+1)
	cmpBuf := make([]byte, bwp.PacketSize)
	prgFiller := newPrgFiller(bwp.PrgKey)
	for correctlyReceived < bwp.NumPackets {
		n, err := udpConnection.Read(recBuf)
		if err != nil { // Deadline exceeded or read error
			break
		}
		numPacketsReceived++
		if int64(n) != bwp.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			// fmt.Println("Incorrect size.", n, "bytes instead of", bwp.PacketSize)
			continue
		}
		// Could consider pre-computing all the packets in a separate goroutine
		// but since computation is usually much higher than bandwidth, this is
		// not necessary
		// Todo: create separate verif function which only compares the packet
		// so that a discrepancy is noticed immediately without generating the
		// entire packet
		iv := int64(binary.LittleEndian.Uint32(recBuf))
		seqNo := int(iv / bwp.PacketSize)
		interPacketArrivalTime[seqNo] = time.Now().UnixNano()
		prgFiller.Fill(int(iv), cmpBuf)
		binary.LittleEndian.PutUint32(cmpBuf, uint32(iv))
		if bytes.Equal(recBuf[:bwp.PacketSize], cmpBuf) {
			correctlyReceived++
		}
	}

	res := Result{
		NumPacketsReceived: numPacketsReceived,
		CorrectlyReceived:  correctlyReceived,
		PrgKey:             bwp.PrgKey,
	}
	res.IPAvar, res.IPAmin, res.IPAavg, res.IPAmax = aggrInterArrivalTime(interPacketArrivalTime)
	return res
}

func aggrInterArrivalTime(bwr map[int]int64) (IPAvar, IPAmin, IPAavg, IPAmax int64) {
	// reverse map, mapping timestamps to sequence numbers
	revMap := make(map[int64]int, len(bwr))
	keys := make([]int64, 0, len(bwr)) // keys are the timestamps of the received packets
	// fill the reverse map and the keep track of the timestamps
	for k, v := range bwr {
		revMap[v] = k
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] }) // sorted timestamps of the received packets

	// We keep only the interarrival times of successive packets with no drops
	var iat []int64
	i := 1
	for i < len(keys) {
		if revMap[keys[i-1]]+1 == revMap[keys[i]] { // valid measurement without reordering, include
			iat = append(iat, keys[i]-keys[i-1]) // resulting interarrival time
		}
		i += 1
	}

	// Compute variance and average
	var average float64 = 0
	IPAmin = -1
	for _, v := range iat {
		if v > IPAmax {
			IPAmax = v
		}
		if v < IPAmin || IPAmin == -1 {
			IPAmin = v
		}
		average += float64(v) / float64(len(iat))
	}
	IPAvar = IPAmax - int64(average)
	IPAavg = int64(average)
	return
}
