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

package models

import (
	"fmt"
	"reflect"
)

// TracerouteItem reflects one row in the Traceroute table with all columns.
type TracerouteItem struct {
	Inserted       int64 // ms Inserted time as primary key
	ActualDuration int   // ms
	CIa            string
	CAddr          string
	SIa            string
	SAddr          string
	Timeout        float32 // s Default 2
	CmdOutput      string  // command output
	Error          string
	Path           string
}

// TrHopItem reflects one row in the Hop table with all columns.
type TrHopItem struct {
	Inserted   int64 // ms Inserted time as primary key
	RunTimeKey int64 // ms, represent the inserted key in the corresponding entry in traceroute table
	Ord        int
	HopIa      string
	HopAddr    string
	IntfID     int
	RespTime1  float32 // ms
	RespTime2  float32 // ms
	RespTime3  float32 // ms
}

// GetHeaders iterates the TracerouteItem and returns struct variable names.
func (tr TracerouteItem) GetHeaders() []string {
	e := reflect.ValueOf(&tr).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		name := e.Type().Field(i).Name
		s = append(s, name)
	}
	return s
}

// ToSlice iterates the TracerouteItem and returns struct values.
func (tr TracerouteItem) ToSlice() []string {
	e := reflect.ValueOf(&tr).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		s = append(s, fmt.Sprintf("%v", f.Interface()))
	}
	return s
}

// GetHeaders iterates the TrHopItem and returns struct variable names.
func (hop TrHopItem) GetHeaders() []string {
	e := reflect.ValueOf(&hop).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		name := e.Type().Field(i).Name
		s = append(s, name)
	}
	return s
}

// ToSlice iterates the TrHopItem and returns struct values.
func (hop TrHopItem) ToSlice() []string {
	e := reflect.ValueOf(&hop).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		s = append(s, fmt.Sprintf("%v", f.Interface()))
	}
	return s
}

// TracerouteHelper reflects one row in the Traceroute table joint with Hop table with only the
// necessary items to display in a graph.
type TracerouteHelper struct {
	Inserted       int64 // ms Inserted time of the TracerouteItem
	ActualDuration int   // ms
	Ord            int
	HopIa          string
	HopAddr        string
	IntfID         int
	RespTime1      float32 // ms
	RespTime2      float32 // ms
	RespTime3      float32 // ms
	CmdOutput      string  // command output
	Error          string
	Path           string
}

// Store only part of the TrHopItem information used for display in the graph
type ReducedTrHopItem struct {
	HopIa     string
	HopAddr   string
	IntfID    int
	RespTime1 float32 // ms
	RespTime2 float32 // ms
	RespTime3 float32 // ms
}

// parse the result SQL query first in TracerouteHelper structs and then convert them into
// TracerouteGraph in order to avoid redundancy
type TracerouteGraph struct {
	Inserted       int64
	ActualDuration int
	TrHops         []ReducedTrHopItem
	CmdOutput      string // command output
	Error          string
	Path           string
}

// createTracerouteTable operates on the DB to create the traceroute table.
func createTracerouteTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS traceroute(
        Inserted BIGINT NOT NULL PRIMARY KEY,
        ActualDuration INT,
        CIa TEXT,
        CAddr TEXT,
        SIa TEXT,
        SAddr TEXT,
		Timeout REAL,
		CmdOutput TEXT,
		Error TEXT,
		Path TEXT
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

func createTrHopTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS trhops(
        Inserted BIGINT NOT NULL PRIMARY KEY,
        RunTimeKey BIGINT,
    	Ord INT,
		HopIa TEXT,
		HopAddr TEXT,
        IntfID INT,
		RespTime1 REAL,
		RespTime2 REAL,
		RespTime3 REAL
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

// StoreTracerouteItem operates on the DB to insert a TracerouteItem.
func StoreTracerouteItem(tr *TracerouteItem) error {
	sqlInsert := `
    INSERT INTO traceroute(
		Inserted,
        ActualDuration,
        CIa,
        CAddr,
        SIa,
        SAddr,
		Timeout,
		CmdOutput,
		Error,
		Path
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		tr.Inserted,
		tr.ActualDuration,
		tr.CIa,
		tr.CAddr,
		tr.SIa,
		tr.SAddr,
		tr.Timeout,
		tr.CmdOutput,
		tr.Error,
		tr.Path)
	return err
}

// StoreTrHopItem operates on the DB to insert a TrHopItem.
func StoreTrHopItem(hop *TrHopItem) error {
	sqlInsert := `
    INSERT INTO trhops(
		Inserted,
        RunTimeKey,
        Ord,
		HopIa,
		HopAddr,
        IntfID,
		RespTime1,
		RespTime2,
		RespTime3
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		hop.Inserted,
		hop.RunTimeKey,
		hop.Ord,
		hop.HopIa,
		hop.HopAddr,
		hop.IntfID,
		hop.RespTime1,
		hop.RespTime2,
		hop.RespTime3)
	return err
}

// ReadTracerouteItemsAll operates on the DB to return all traceroute rows.
func ReadTracerouteItemsAll() ([]TracerouteItem, error) {
	sqlReadAll := `
	SELECT
		Inserted,
		ActualDuration,
		CIa,
		CAddr,
		SIa,
		SAddr,
		Timeout,
		CmdOutput,
		Error,
		Path
	FROM traceroute
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TracerouteItem
	for rows.Next() {
		tr := TracerouteItem{}
		err = rows.Scan(
			&tr.Inserted,
			&tr.ActualDuration,
			&tr.CIa,
			&tr.CAddr,
			&tr.SIa,
			&tr.SAddr,
			&tr.Timeout,
			&tr.CmdOutput,
			&tr.Error,
			&tr.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, tr)
	}
	return result, nil
}

// ReadTracerouteItemsSince operates on the DB to return all rows in traceroute join trhops
// which are more recent than the 'since' epoch in ms.
func ReadTracerouteItemsSince(since string) ([]TracerouteGraph, error) {
	sqlReadSince := `
	SELECT
		a.Inserted,   
		a.ActualDuration, 
		h.Ord,          
		h.HopIa,          
		h.HopAddr,        
		h.IntfID,         
		h.RespTime1,      
		h.RespTime2,     
		h.RespTime3,	   
		a.CmdOutput,       
		a.Error,
		a.Path       
	FROM (
			SELECT
				Inserted,
				ActualDuration,
				CIa,
				CAddr,
				SIa,
				SAddr,
				Timeout,
				CmdOutput,
				Error,
				Path
			FROM traceroute
			WHERE Inserted > ?			
	) AS a
	INNER JOIN trhops AS h ON a.Inserted = h.RunTimeKey
	ORDER BY datetime(a.Inserted) DESC
    `
	rows, err := db.Query(sqlReadSince, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TracerouteHelper
	for rows.Next() {
		trf := TracerouteHelper{}
		err = rows.Scan(
			&trf.Inserted,
			&trf.ActualDuration,
			&trf.Ord,
			&trf.HopIa,
			&trf.HopAddr,
			&trf.IntfID,
			&trf.RespTime1,
			&trf.RespTime2,
			&trf.RespTime3,
			&trf.CmdOutput,
			&trf.Error,
			&trf.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, trf)
	}

	var graphEntries []TracerouteGraph
	var lastEntryInsertedTime int64

	// Store the infos of the last hop
	var inserted int64
	var actualDuration int
	var trhops []ReducedTrHopItem
	var cmdOutput, path, errors string

	for _, hop := range result {
		if lastEntryInsertedTime == 0 {
			lastEntryInsertedTime = hop.Inserted
			trhops = nil
		} else if lastEntryInsertedTime != hop.Inserted {
			trg := TracerouteGraph{
				Inserted:       inserted,
				ActualDuration: actualDuration,
				TrHops:         trhops,
				CmdOutput:      cmdOutput,
				Error:          errors,
				Path:           path}

			lastEntryInsertedTime = hop.Inserted
			graphEntries = append(graphEntries, trg)
			trhops = nil
		}

		inserted = hop.Inserted
		actualDuration = hop.ActualDuration
		cmdOutput = hop.CmdOutput
		errors = hop.Error
		path = hop.Path

		rth := ReducedTrHopItem{HopIa: hop.HopIa,
			HopAddr:   hop.HopAddr,
			IntfID:    hop.IntfID,
			RespTime1: hop.RespTime1,
			RespTime2: hop.RespTime2,
			RespTime3: hop.RespTime3}
		trhops = append(trhops, rth)
	}

	return graphEntries, nil
}

// DeleteTracerouteItemsBefore operates on the DB to remote all traceroute rows
// which are more older than the 'before' epoch in ms.
func DeleteTracerouteItemsBefore(before string) (int64, error) {
	sqlDeleteBefore := `
    DELETE FROM traceroute
    WHERE Inserted < ?
    `
	res, err := db.Exec(sqlDeleteBefore, before)
	if err != nil {
		return 0, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return count, err
	}
	return count, nil
}

// DeleteTrHopItemsBefore operates on the DB to remote all trhops rows
// which are more older than the 'before' epoch in ms.
func DeleteTrHopItemsBefore(before string) (int64, error) {
	sqlDeleteBefore := `
    DELETE FROM trhops
    WHERE RunTimeKey < ?
    `
	res, err := db.Exec(sqlDeleteBefore, before)
	if err != nil {
		return 0, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return count, err
	}
	return count, nil
}
