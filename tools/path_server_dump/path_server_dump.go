// Copyright 2018 ETH Zurich
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
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/proto"
)

type rawBytes []byte

type asIface struct {
	IA    addr.IA
	ifNum common.IFIDType
}

func newASIface(isd addr.ISD, as addr.AS, ifNum common.IFIDType) asIface {
	return asIface{IA: addr.IA{I: isd, A: as}, ifNum: ifNum}
}
func ifsArrayToString(ifs []asIface) string {
	if len(ifs) == 0 {
		return ""
	}
	strs := []string{fmt.Sprintf("%s %d", ifs[0].IA, ifs[0].ifNum)}
	for i := 1; i < len(ifs)-1; i += 2 {
		strs = append(strs, fmt.Sprintf("%d %s %d", ifs[i].ifNum, ifs[i].IA, ifs[i+1].ifNum))
	}
	strs = append(strs, fmt.Sprintf("%d %s", ifs[len(ifs)-1].ifNum, ifs[len(ifs)-1].IA))
	return strings.Join(strs, ">")
}

type segment struct {
	SegType    proto.PathSegType
	Src        addr.IA
	Dst        addr.IA
	interfaces []asIface
}

func newSegment(segType proto.PathSegType, srcI addr.ISD, srcA addr.AS, dstI addr.ISD, dstA addr.AS, interfaces []asIface) segment {
	return segment{SegType: segType, Src: addr.IA{I: srcI, A: srcA}, Dst: addr.IA{I: dstI, A: dstA}, interfaces: interfaces}
}
func (s segment) String() string {
	toRet := s.SegType.String() + "\t"
	return toRet + ifsArrayToString(s.interfaces)
}

// returns if this segment is < the other segment. It relies on the
// short circuit of the OR op. E.g. (for two dimensions):
// a.T < b.T || ( a.T == b.T && a.L < b.L )
func (s *segment) lessThan(o *segment) bool {
	segsLessThan := func(lhs, rhs *segment) bool {
		for i := 0; i < len(lhs.interfaces); i++ {
			if lhs.interfaces[i].IA != rhs.interfaces[i].IA {
				return lhs.interfaces[i].IA.IAInt() < rhs.interfaces[i].IA.IAInt()
			} else if lhs.interfaces[i].ifNum != rhs.interfaces[i].ifNum {
				return lhs.interfaces[i].ifNum < rhs.interfaces[i].ifNum
			}
		}
		return false
	}
	// reversed Type comparison so core < down < up
	return s.SegType > o.SegType || (s.SegType == o.SegType &&
		(len(s.interfaces) < len(o.interfaces) ||
			(len(s.interfaces) == len(o.interfaces) && (segsLessThan(s, o)))))
}

func findIA() addr.IA {
	SC := os.Getenv("SC")
	if SC == "" {
		panic("Env $SC not defined")
	}
	iaFile := filepath.Join(SC, "gen", "ia")
	if _, err := os.Stat(iaFile); err == nil {
		iaBytes, err := ioutil.ReadFile(iaFile)
		if err != nil {
			panic(fmt.Sprintf("Cannot read %s: %v", iaFile, err))
		}
		ia, err := addr.IAFromFileFmt(string(iaBytes), false)
		if err != nil {
			panic(fmt.Sprintf("Cannot parse IA %s: %v", string(iaBytes), err))
		}
		return ia
	}
	// we have no ia file, complain for now
	panic(fmt.Sprintf("Could not find ia file on %s", iaFile))
}
func findDBFilename() string {
	ia := findIA()
	SC := os.Getenv("SC")
	pathDBFileName := fmt.Sprintf("ps%s-1.path.db", ia.FileFmt(false))
	return filepath.Join(SC, "gen-cache", pathDBFileName)
}

// returns the name of the created file
func copyDBToTemp(filename string) string {
	copyOneFile := func(dstDir, srcFileName string) error {
		src, err := os.Open(srcFileName)
		if err != nil {
			return fmt.Errorf("Cannot open %s: %v", srcFileName, err)
		}
		dstFilename := filepath.Join(dstDir, filepath.Base(srcFileName))
		dst, err := os.Create(dstFilename)
		if err != nil {
			return fmt.Errorf("Cannot open %s: %v", dstFilename, err)
		}
		_, err = io.Copy(dst, src)
		if err != nil {
			return fmt.Errorf("Cannot copy %s to %s: %v", srcFileName, dstFilename, err.Error())
		}
		return nil
	}
	dirName, err := ioutil.TempDir("/tmp", "pathserver_dump")
	if err != nil {
		panic(fmt.Sprintf("Error creating temporary dir: %v", err))
	}

	err = copyOneFile(dirName, filename)
	if err != nil {
		panic(err.Error())
	}
	err = copyOneFile(dirName, filename+"-wal")
	if err != nil {
		fmt.Printf("No panic: %v", err)
	}
	err = copyOneFile(dirName, filename+"-shm")
	if err != nil {
		fmt.Printf("No panic: %v", err)
	}
	baseFilename := filepath.Base(filename)
	return filepath.Join(dirName, baseFilename)
}
func removeAllDir(dirName string) {
	err := os.RemoveAll(dirName)
	if err != nil {
		fmt.Printf("Error when removing temp dir %s: %v\n", dirName, err)
	}
}

func main() {
	foundFilename := findDBFilename()
	filename := copyDBToTemp(foundFilename)
	defer removeAllDir(filepath.Dir(filename))

	// TODO it would be ideal to open the DB file in place instead of copying it, but we always
	// get a "database is locked" error. Tried with a combination of ?mode=ro&_journal=OFF&_mutex=no&
	// _txlock=immediate&journal=wal&_query_only=yes?_locking=normal&immutable=true
	// Fails because of setting journal (vendor/.../mattn/.../sqlite3.go:1480), for all journal modes")
	db, err := sql.Open("sqlite3", filename+"?mode=ro")
	if err != nil {
		panic(err.Error())
	}
	// TODO: three queries? query 1 and 3 coud be easily joined
	sqlstmt := `SELECT SegRowID, Type from SegTypes`
	rows, err := db.Query(sqlstmt)
	if err != nil {
		panic(err.Error())
	}
	var segRowID int64
	var segType proto.PathSegType
	segTypes := map[int64]proto.PathSegType{}
	for rows.Next() {
		err = rows.Scan(&segRowID, &segType)
		if err != nil {
			panic(err.Error())
		}
		segTypes[segRowID] = segType
	}
	rows.Close()

	sqlstmt = `SELECT IsdID, AsID, IntfID, SegRowID FROM IntfToSeg`
	rows, err = db.Query(sqlstmt)
	if err != nil {
		panic(err.Error())
	}
	var isd addr.ISD
	var as addr.AS
	var ifaceID common.IFIDType
	segInterfaces := map[int64][]asIface{}
	for rows.Next() {
		err = rows.Scan(&isd, &as, &ifaceID, &segRowID)
		if err != nil {
			panic(err.Error())
		}
		segInterfaces[segRowID] = append(segInterfaces[segRowID], newASIface(isd, as, ifaceID))
	}
	rows.Close()

	sqlstmt = `SELECT RowID, SegID, FullID, LastUpdated, InfoTs, Segment, MaxExpiry,
    StartIsdID, StartAsID, EndIsdID, EndAsID FROM Segments`
	rows, err = db.Query(sqlstmt)
	if err != nil {
		panic(err.Error())
	}
	var segID, fullID, packedSeg rawBytes
	var lastUpdated, infoTS, maxExpiry []uint8
	var startISD, endISD addr.ISD
	var startAS, endAS addr.AS
	segments := []segment{}
	for rows.Next() {
		err = rows.Scan(&segRowID, &segID, &fullID, &lastUpdated, &infoTS, &packedSeg, &maxExpiry,
			&startISD, &startAS, &endISD, &endAS)
		if err != nil {
			panic(err.Error())
		}
		segmt := newSegment(segTypes[segRowID], startISD, startAS, endISD, endAS, segInterfaces[segRowID])
		segments = append(segments, segmt)
	}
	rows.Close()
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].lessThan(&segments[j])
	})
	for _, seg := range segments {
		fmt.Println(seg)
	}
}
