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
// limitations under the License.

package pathdb

import (
	"database/sql"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/private/ctrl/path_mgmt/proto"
	seg "github.com/scionproto/scion/pkg/segment"
	"github.com/scionproto/scion/pkg/segment/iface"
	"github.com/scionproto/scion/private/pathdb"

	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

type asIface struct {
	IA    addr.IA
	IfNum iface.ID
}

type segment struct {
	SegType    string
	Src        addr.IA
	Dst        addr.IA
	Interfaces []asIface
	Updated    time.Time
	Expiry     time.Time
}

func newSegment(segType proto.PathSegType, srcIa addr.IA, dstIa addr.IA,
	packedSeg []byte, updateTime, expiryTime int64) segment {

	// traverse the segments to ensure even number of inferfaces in hops
	var err error
	var theseg *seg.PathSegment
	theseg, err = pathdb.UnpackSegment(packedSeg)
	if CheckError(err) {
		panic(err)
	}
	var interfaces []asIface
	for _, ase := range theseg.ASEntries {
		hof := ase.HopEntry.HopField
		if hof.ConsIngress > 0 {
			interfaces = append(interfaces, asIface{ase.Local, iface.ID(hof.ConsIngress)})
		}
		if hof.ConsEgress > 0 {
			interfaces = append(interfaces, asIface{ase.Local, iface.ID(hof.ConsEgress)})
		}
	}
	return segment{SegType: segType.String(), Src: srcIa, Dst: dstIa,
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
	var ifaceID iface.ID
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
		ia, err := addr.IAFrom(isd, as)
		if err != nil {
			return nil, err
		}
		result[segRowID] = append(result[segRowID], asIface{ia, ifaceID})
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
		srcIa, err := addr.IAFrom(startISD, startAS)
		if err != nil {
			return nil, err
		}
		dstIa, err := addr.IAFrom(endISD, endAS)
		if err != nil {
			return nil, err
		}
		segmt := newSegment(segTypes[segRowID], srcIa, dstIa, packedSeg, lastUpdated, maxExpiry)
		result = append(result, segmt)
	}
	return result, nil
}
