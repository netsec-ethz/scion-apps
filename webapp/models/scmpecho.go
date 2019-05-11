package models

import (
	"fmt"
	"reflect"
)

// EchoItem reflects one row in the echo table with all columns
type EchoItem struct{
	Inserted     int64  // Inserted time
	CIa            string 
	CAddr          string 
	CPort          int    
	SIa            string 
	SAddr          string 
	SPort          int    
	ResponseTime int    
	PktLoss      bool   // indicating if the packet is lost
	CmdOutput    string // command output
	Error        string 
	Path         string
	Log          string
}

// createEchoTable operates on the DB to create the echo table.
func createEchoTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS echo(
        Inserted BIGINT NOT NULL PRIMARY KEY,
		CIa TEXT,
        CAddr TEXT,
        CPort INT,
        SIa TEXT,
        SAddr TEXT,
        SPort INT,
		ResponseTime INT,
	    PktLoss BOOL,
	    CmdOutput TEXT,
		Error TEXT,
		Path TEXT,
		Log TEXT
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
        CIa,
        CAddr,
        CPort,
        SIa,
        SAddr,
        SPort,
		ResponseTime,
	    PktLoss,
	    CmdOutput,
		Error,
		Path,
        Log
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		echo.Inserted,
		echo.CIa,
		echo.CAddr,
		echo.CPort,
		echo.SIa,
		echo.SAddr,
		echo.SPort,
		echo.ResponseTime,
		echo.PktLoss,
		echo.CmdOutput,
		echo.Error,
		echo.Path,
		echo.Log)
	return err
}

// ReadEchoItemsAll operates on the DB to return all echo rows.
func ReadEchoItemsAll() ([]EchoItem, error) {
	sqlReadAll := `
    SELECT
		Inserted,
		CIa,
        CAddr,
        CPort,
        SIa,
        SAddr,
        SPort,
		ResponseTime,
		PktLoss,
		CmdOutput,
		Error,
		Path,
		Log
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
			&echo.CIa,
			&echo.CAddr,
			&echo.CPort,
			&echo.SIa,
			&echo.SAddr,
			&echo.SPort,
			&echo.ResponseTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error,
			&echo.Path,
			&echo.Log)
		if err != nil {
			return nil, err
		}
		result = append(result, echo)
	}
	return result, nil
}

// ReadEchoItemsSince operates on the DB to return all echo rows
// which are more recent than the 'since' epoch in ms.
func ReadEchoItemsSince(since string) ([]EchoItem, error) {
	sqlReadSince := `
    SELECT
		Inserted,
	    CIa,
		CAddr,
		CPort,
		SIa,
		SAddr,
		SPort,
		ResponseTime,
		PktLoss,
		CmdOutput,
		Error,
		Path,
		Log
	FROM echo
    WHERE Inserted > ?
    ORDER BY datetime(Inserted) DESC
    `
	rows, err := db.Query(sqlReadSince, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []EchoItem
	for rows.Next() {
		echo := EchoItem{}
		err = rows.Scan(
			&echo.Inserted,
			&echo.CIa,
			&echo.CAddr,
			&echo.CPort,
			&echo.SIa,
			&echo.SAddr,
			&echo.SPort,
			&echo.ResponseTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error,
			&echo.Path,
			&echo.Log)
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