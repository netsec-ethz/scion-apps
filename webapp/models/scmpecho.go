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

// CmdItem could be either EchoItem or BwTestItem
type CmdItem interface {
	GetHeaders() []string
	ToSlice() []string
}

// EchoItem reflects one row in the echo table with all columns
type EchoItem struct {
	Inserted       int64 // ms Inserted time
	ActualDuration int   // ms
	CIa            string
	CAddr          string
	SIa            string
	SAddr          string
	Count          int     // Default 1
	Timeout        float32 // s Default 2
	Interval       float32 // s Default 1
	ResponseTime   float32 // ms
	RunTime        float32
	PktLoss        int    // percent Indicating pkt loss rate
	CmdOutput      string // command output
	Error          string
	Path           string
}

// EchoGraph reflects one row in the echo table with only the
// necessary items to display in a graph
type EchoGraph struct {
	Inserted       int64
	ActualDuration int
	ResponseTime   float32
	RunTime        float32
	PktLoss        int
	CmdOutput      string
	Error          string
	Path           string
}

// createEchoTable operates on the DB to create the echo table.
func createEchoTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS echo(
        Inserted BIGINT NOT NULL PRIMARY KEY,
        ActualDuration INT,
        CIa TEXT,
        CAddr TEXT,
        SIa TEXT,
        SAddr TEXT,
        Count INT,
        Timeout REAL,
        Interval REAL,
        ResponseTime REAL,
        RunTime REAL,
        PktLoss INT,
        CmdOutput TEXT,
        Error TEXT,
        Path TEXT
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

// GetHeaders iterates the EchoItem and returns struct variable names.
func (echo EchoItem) GetHeaders() []string {
	e := reflect.ValueOf(&echo).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		name := e.Type().Field(i).Name
		s = append(s, name)
	}
	return s
}

// ToSlice iterates the EchoItem and returns struct values.
func (echo EchoItem) ToSlice() []string {
	e := reflect.ValueOf(&echo).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		s = append(s, fmt.Sprintf("%v", f.Interface()))
	}
	return s
}

// StoreEchoItem operates on the DB to insert a EchoItem.
func StoreEchoItem(echo *EchoItem) error {
	sqlInsert := `
    INSERT INTO echo(
        Inserted,
        ActualDuration,
        CIa,
        CAddr,
        SIa,
        SAddr,
        Count,
        Timeout,
        Interval,
        ResponseTime,
        RunTime,
        PktLoss,
        CmdOutput,
        Error,
        Path
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		echo.Inserted,
		echo.ActualDuration,
		echo.CIa,
		echo.CAddr,
		echo.SIa,
		echo.SAddr,
		echo.Count,
		echo.Timeout,
		echo.Interval,
		echo.ResponseTime,
		echo.RunTime,
		echo.PktLoss,
		echo.CmdOutput,
		echo.Error,
		echo.Path)
	return err
}

// ReadEchoItemsAll operates on the DB to return all echo rows.
func ReadEchoItemsAll() ([]EchoItem, error) {
	sqlReadAll := `
    SELECT
        Inserted,
        ActualDuration,
        CIa,
        CAddr,
        SIa,
        SAddr,
        Count,
        Timeout,
        Interval,
        ResponseTime,
        RunTime,
        PktLoss,
        CmdOutput,
        Error,
        Path
        FROM echo
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EchoItem
	for rows.Next() {
		echo := EchoItem{}
		err = rows.Scan(
			&echo.Inserted,
			&echo.ActualDuration,
			&echo.CIa,
			&echo.CAddr,
			&echo.SIa,
			&echo.SAddr,
			&echo.Count,
			&echo.Timeout,
			&echo.Interval,
			&echo.ResponseTime,
			&echo.RunTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error,
			&echo.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, echo)
	}
	return result, nil
}

// ReadEchoItemsSince operates on the DB to return all echo rows
// which are more recent than the 'since' epoch in ms.
func ReadEchoItemsSince(since string) ([]EchoGraph, error) {
	sqlReadSince := `
    SELECT
        Inserted,
        ActualDuration,
        ResponseTime,
        RunTime,
        PktLoss,
        CmdOutput,
        Error,
        Path
    FROM echo
    WHERE Inserted > ?
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadSince, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EchoGraph
	for rows.Next() {
		echo := EchoGraph{}
		err = rows.Scan(
			&echo.Inserted,
			&echo.ActualDuration,
			&echo.ResponseTime,
			&echo.RunTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error,
			&echo.Path)
		if err != nil {
			return nil, err
		}
		result = append(result, echo)
	}
	return result, nil
}

// DeleteEchoItemsBefore operates on the DB to remote all echo rows
// which are more older than the 'before' epoch in ms.
func DeleteEchoItemsBefore(before string) (int64, error) {
	sqlDeleteBefore := `
    DELETE FROM echo
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
