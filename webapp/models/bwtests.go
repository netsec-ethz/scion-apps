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

// BwTestItem reflects one row in the bwtests table with all columns.
type BwTestItem struct {
	Inserted       int64  // v0, ms
	ActualDuration int    // v0, ms
	CIa            string // v0
	CAddr          string // v0
	CPort          int    // v0
	SIa            string // v0
	SAddr          string // v0
	SPort          int    // v0
	CSDuration     int    // v0, ms
	CSPackets      int    // v0, packets
	CSPktSize      int    // v0, bytes
	CSBandwidth    int    // v0, bps
	CSThroughput   int    // v0, bps
	CSArrVar       int    // v0, ms
	CSArrAvg       int    // v0, ms
	CSArrMin       int    // v0, ms
	CSArrMax       int    // v0, ms
	SCDuration     int    // v0, ms
	SCPackets      int    // v0, packets
	SCPktSize      int    // v0, bytes
	SCBandwidth    int    // v0, bps
	SCThroughput   int    // v0, bps
	SCArrVar       int    // v0, ms
	SCArrAvg       int    // v0, ms
	SCArrMin       int    // v0, ms
	SCArrMax       int    // v0, ms
	Error          string // v0
	Path           string // v1
	Log            string // v2
}

// GetHeaders iterates the BwTestItem and returns struct variable names.
func (bwtest BwTestItem) GetHeaders() []string {
	e := reflect.ValueOf(&bwtest).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		name := e.Type().Field(i).Name
		s = append(s, name)
	}
	return s
}

// ToSlice iterates the BwTestItem and returns struct values.
func (bwtest BwTestItem) ToSlice() []string {
	e := reflect.ValueOf(&bwtest).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		s = append(s, fmt.Sprintf("%v", f.Interface()))
	}
	return s
}

// BwTestGraph reflects one row in the bwtests table with only the
// necessary items to display in a graph.
type BwTestGraph struct {
	Inserted       int64
	ActualDuration int
	CSBandwidth    int
	CSThroughput   int
	SCBandwidth    int
	SCThroughput   int
	Error          string
	Path           string
	Log            string
}

// createBwTestTable operates on the DB to create the bwtests table.
func createBwTestTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS bwtests(
        Inserted BIGINT NOT NULL PRIMARY KEY,
        ActualDuration INT,
        CIa TEXT,
        CAddr TEXT,
        CPort INT,
        SIa TEXT,
        SAddr TEXT,
        SPort INT,
        CSDuration INT,
        CSPackets INT,
        CSPktSize INT,
        CSBandwidth INT,
        CSThroughput INT,
        CSArrVar INT,
        CSArrAvg INT,
        CSArrMin INT,
        CSArrMax INT,
        SCDuration INT,
        SCPackets INT,
        SCPktSize INT,
        SCBandwidth INT,
        SCThroughput INT,
        SCArrVar INT,
        SCArrAvg INT,
        SCArrMin INT,
        SCArrMax INT,
		Error TEXT
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

// StoreBwTestItem operates on the DB to insert a BwTestItem.
func StoreBwTestItem(bwtest *BwTestItem) error {
	sqlInsert := `
    INSERT INTO bwtests(
        Inserted,
        ActualDuration,
        CIa,
        CAddr,
        CPort,
        SIa,
        SAddr,
        SPort,
        CSDuration,
        CSPackets,
        CSPktSize,
        CSBandwidth,
        CSThroughput,
        CSArrVar,
        CSArrAvg,
        CSArrMin,
        CSArrMax,
        SCDuration,
        SCPackets,
        SCPktSize,
        SCBandwidth,
        SCThroughput,
        SCArrVar,
        SCArrAvg,
        SCArrMin,
        SCArrMax,
        Error,
        Path,
        Log
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		bwtest.Inserted,
		bwtest.ActualDuration,
		bwtest.CIa,
		bwtest.CAddr,
		bwtest.CPort,
		bwtest.SIa,
		bwtest.SAddr,
		bwtest.SPort,
		bwtest.CSDuration,
		bwtest.CSPackets,
		bwtest.CSPktSize,
		bwtest.CSBandwidth,
		bwtest.CSThroughput,
		bwtest.CSArrVar,
		bwtest.CSArrAvg,
		bwtest.CSArrMin,
		bwtest.CSArrMax,
		bwtest.SCDuration,
		bwtest.SCPackets,
		bwtest.SCPktSize,
		bwtest.SCBandwidth,
		bwtest.SCThroughput,
		bwtest.SCArrVar,
		bwtest.SCArrAvg,
		bwtest.SCArrMin,
		bwtest.SCArrMax,
		bwtest.Error,
		bwtest.Path,
		bwtest.Log)
	return err
}

// ReadBwTestItemsAll operates on the DB to return all bwtests rows.
func ReadBwTestItemsAll() ([]BwTestItem, error) {
	sqlReadAll := `
    SELECT
        Inserted,
        ActualDuration,
        CIa,
        CAddr,
        CPort,
        SIa,
        SAddr,
        SPort,
        CSDuration,
        CSPackets,
        CSPktSize,
        CSBandwidth,
        CSThroughput,
        CSArrVar,
        CSArrAvg,
        CSArrMin,
        CSArrMax,
        SCDuration,
        SCPackets,
        SCPktSize,
        SCBandwidth,
        SCThroughput,
        SCArrVar,
        SCArrAvg,
        SCArrMin,
        SCArrMax,
        Error,
        Path,
        Log
    FROM bwtests
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BwTestItem
	for rows.Next() {
		bwtest := BwTestItem{}
		err = rows.Scan(
			&bwtest.Inserted,
			&bwtest.ActualDuration,
			&bwtest.CIa,
			&bwtest.CAddr,
			&bwtest.CPort,
			&bwtest.SIa,
			&bwtest.SAddr,
			&bwtest.SPort,
			&bwtest.CSDuration,
			&bwtest.CSPackets,
			&bwtest.CSPktSize,
			&bwtest.CSBandwidth,
			&bwtest.CSThroughput,
			&bwtest.CSArrVar,
			&bwtest.CSArrAvg,
			&bwtest.CSArrMin,
			&bwtest.CSArrMax,
			&bwtest.SCDuration,
			&bwtest.SCPackets,
			&bwtest.SCPktSize,
			&bwtest.SCBandwidth,
			&bwtest.SCThroughput,
			&bwtest.SCArrVar,
			&bwtest.SCArrAvg,
			&bwtest.SCArrMin,
			&bwtest.SCArrMax,
			&bwtest.Error,
			&bwtest.Path,
			&bwtest.Log)
		if err != nil {
			return nil, err
		}
		result = append(result, bwtest)
	}
	return result, nil
}

// ReadBwTestItemsSince operates on the DB to return all bwtests rows
// which are more recent than the 'since' epoch in ms.
func ReadBwTestItemsSince(since string) ([]BwTestGraph, error) {
	sqlReadSince := `
    SELECT
        Inserted,
        ActualDuration,
        CSBandwidth,
        CSThroughput,
        SCBandwidth,
        SCThroughput,
        Error,
        Path,
        Log
    FROM bwtests
    WHERE Inserted > ?
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadSince, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []BwTestGraph
	for rows.Next() {
		bwtest := BwTestGraph{}
		err = rows.Scan(
			&bwtest.Inserted,
			&bwtest.ActualDuration,
			&bwtest.CSBandwidth,
			&bwtest.CSThroughput,
			&bwtest.SCBandwidth,
			&bwtest.SCThroughput,
			&bwtest.Error,
			&bwtest.Path,
			&bwtest.Log)
		if err != nil {
			return nil, err
		}
		result = append(result, bwtest)
	}
	return result, nil
}

// DeleteBwTestItemsBefore operates on the DB to remote all bwtests rows
// which are more older than the 'before' epoch in ms.
func DeleteBwTestItemsBefore(before string) (int64, error) {
	sqlDeleteBefore := `
    DELETE FROM bwtests
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
