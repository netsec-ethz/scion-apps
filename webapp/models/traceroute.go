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
}

// HopItem reflects one row in the Hop table with all columns.
type HopItem struct {
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

// GetHeaders iterates the HopItem and returns struct variable names.
func (hop HopItem) GetHeaders() []string {
	e := reflect.ValueOf(&hop).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		name := e.Type().Field(i).Name
		s = append(s, name)
	}
	return s
}

// ToSlice iterates the HopItem and returns struct values.
func (hop HopItem) ToSlice() []string {
	e := reflect.ValueOf(&hop).Elem()
	var s []string
	for i := 0; i < e.NumField(); i++ {
		f := e.Field(i)
		s = append(s, fmt.Sprintf("%v", f.Interface()))
	}
	return s
}

// TracerouteGraph reflects one row in the Traceroute table joint with Hop table with only the
// necessary items to display in a graph.
type TracerouteGraph struct {
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
		Error TEXT
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

func createHopTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS hops(
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
		Error
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		tr.Error)
	return err
}

// StoreHopItem operates on the DB to insert a HopItem.
func StoreHopItem(hop *HopItem) error {
	sqlInsert := `
    INSERT INTO hops(
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
		Error
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
			&tr.Error)
		if err != nil {
			return nil, err
		}
		result = append(result, tr)
	}
	return result, nil
}

// ReadTracerouteItemsSince operates on the DB to return all rows in traceroute join hops
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
		a.Error       
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
				Error
			FROM traceroute
			WHERE Inserted > ?
			ORDER BY datetime(Inserted) DESC
	) AS a
	INNER JOIN hops AS h ON a.Inserted = h.RunTimeKey
    `
	rows, err := db.Query(sqlReadSince, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TracerouteGraph
	for rows.Next() {
		trg := TracerouteGraph{}
		err = rows.Scan(
			&trg.Inserted,
			&trg.ActualDuration,
			&trg.Ord,
			&trg.HopIa,
			&trg.HopAddr,
			&trg.IntfID,
			&trg.RespTime1,
			&trg.RespTime2,
			&trg.RespTime3,
			&trg.CmdOutput,
			&trg.Error)
		if err != nil {
			return nil, err
		}
		result = append(result, trg)
	}
	return result, nil
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

// DeleteHopItemsBefore operates on the DB to remote all hops rows
// which are more older than the 'before' epoch in ms.
func DeleteHopItemsBefore(before string) (int64, error) {
	sqlDeleteBefore := `
    DELETE FROM hops
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
