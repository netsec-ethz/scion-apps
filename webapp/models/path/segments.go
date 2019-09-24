// Copyright 2019 ETH Zurich
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
// limitations under the License.package main

package pathdb

import (
	"database/sql"
	"time"

	. "github.com/netsec-ethz/scion-apps/webapp/util"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/seg"
	"github.com/scionproto/scion/go/proto"
)

type asIface struct {
	IA    addr.IA
	IfNum common.IFIDType
}

func newASIface(isd addr.ISD, as addr.AS, ifNum common.IFIDType) asIface {
	return asIface{IA: addr.IA{I: isd, A: as}, IfNum: ifNum}
}

type segment struct {
	SegType    string
	Src        addr.IA
	Dst        addr.IA
	Interfaces []asIface
	Updated    time.Time
	Expiry     time.Time
}

func newSegment(segType proto.PathSegType, srcI addr.ISD, srcA addr.AS, dstI addr.ISD, dstA addr.AS,
	packedSeg []byte, updateTime, expiryTime int64) segment {

	// traverse the segments to ensure even number of inferfaces in hops
	var err error
	var theseg *seg.PathSegment
	theseg, err = seg.NewSegFromRaw(common.RawBytes(packedSeg))
	if CheckError(err) {
		panic(err)
	}
	var interfaces []asIface
	for _, ase := range theseg.ASEntries {
		hop, err := ase.HopEntries[0].HopField()
		if CheckError(err) {
			panic(err)
		}
		if hop.ConsIngress > 0 {
			interfaces = append(interfaces, newASIface(ase.IA().I, ase.IA().A, hop.ConsIngress))
		}
		if hop.ConsEgress > 0 {
			interfaces = append(interfaces, newASIface(ase.IA().I, ase.IA().A, hop.ConsEgress))
		}
	}
	return segment{SegType: segType.String(), Src: addr.IA{I: srcI, A: srcA}, Dst: addr.IA{I: dstI, A: dstA},
		Interfaces: interfaces, Updated: time.Unix(0, updateTime), Expiry: time.Unix(expiryTime, 0)}
}

// ReadSegTypesAll operates on the DB to return all SegType rows.
func ReadSegTypesAll(db *sql.DB) (map[int64]proto.PathSegType, error) {
	sqlReadAll := `
    SELECT
         SegRowID,
         Type
    FROM SegTypes
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segRowID int64
	var segType proto.PathSegType
	var result = map[int64]proto.PathSegType{}
	for rows.Next() {
		err = rows.Scan(
			&segRowID,
			&segType)
		if err != nil {
			return nil, err
		}
		result[segRowID] = segType
	}
	return result, nil
}

// ReadIntfToSegAll operates on the DB to return all IntfToSeg rows.
func ReadIntfToSegAll(db *sql.DB) (map[int64][]asIface, error) {
	sqlReadAll := `
    SELECT
        IsdID,
        AsID,
        IntfID,
        SegRowID
    FROM IntfToSeg
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segRowID int64
	var isd addr.ISD
	var as addr.AS
	var ifaceID common.IFIDType
	var result = map[int64][]asIface{}
	for rows.Next() {
		err = rows.Scan(
			&isd,
			&as,
			&ifaceID,
			&segRowID)
		if err != nil {
			return nil, err
		}
		result[segRowID] = append(result[segRowID], newASIface(isd, as, ifaceID))
	}
	return result, nil
}

// ReadSegmentsAll operates on the DB to return all Segments rows.
func ReadSegmentsAll(db *sql.DB, segTypes map[int64]proto.PathSegType) ([]segment, error) {
	sqlReadAll := `
    SELECT
        RowID,
        LastUpdated,
        Segment,
        MaxExpiry,
        StartIsdID,
        StartAsID,
        EndIsdID,
        EndAsID
    FROM Segments
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segRowID int64
	var packedSeg []byte
	var lastUpdated, maxExpiry int64
	var startISD, endISD addr.ISD
	var startAS, endAS addr.AS
	var result = []segment{}
	for rows.Next() {
		err = rows.Scan(
			&segRowID,
			&lastUpdated,
			&packedSeg,
			&maxExpiry,
			&startISD,
			&startAS,
			&endISD,
			&endAS)
		if err != nil {
			return nil, err
		}
		segmt := newSegment(segTypes[segRowID], startISD, startAS, endISD, endAS, packedSeg, lastUpdated, maxExpiry)
		result = append(result, segmt)
	}
	return result, nil
}
