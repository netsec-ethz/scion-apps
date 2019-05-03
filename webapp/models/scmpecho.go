package models

import (
	"fmt"

	log "github.com/inconshreveable/log15"
	. "github.com/netsec-ethz/scion-apps/webapp/util"
)

// EchoItem reflects one row in the echo table with all columns
type EchoItem struct{
	Inserted     int64  // Inserted time
	Src          string // source address
	Dst          string // destination address
	ResponseTime int    
	PktLoss      bool   // indicating if the packet is lost
	CmdOutput    string // command output
	Error        string 
}

// createBwTestTable operates on the DB to create the bwtests table.
func createEchoTable() error {
	sqlCreateTable := `
    CREATE TABLE IF NOT EXISTS echo(
        Inserted BIGINT NOT NULL PRIMARY KEY,
		Src TEXT,
		Dst TEXT,
		ResponseTime INT,
	    PktLoss BOOL,
	    CmdOutput TEXT,
        Error TEXT
    );
    `
	_, err := db.Exec(sqlCreateTable)
	return err
}

// StoreBwTestItem operates on the DB to insert a BwTestItem.
func StoreEchoItem(echo *EchoItem) error {
	sqlInsert := `
    INSERT INTO echo(
        Inserted,
        Src,
		Dst,
		ResponseTime,
	    PktLoss,
	    CmdOutput,
        Error
    ) values(?, ?, ?, ?, ?, ?, ?)
    `
	stmt, err := db.Prepare(sqlInsert)
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		echo.Inserted,
		echo.Src,
		echo.Dst,
		echo.ResponseTime,
		echo.PktLoss,
		echo.CmdOutput,
		echo.Error)
	return err
}

// ReadEchoItemsAll operates on the DB to return all echo rows.
func ReadEchoItemsAll() ([]EchoItem, error) {
	sqlReadAll := `
    SELECT
		Inserted,
		Src,
		Dst,
		ResponseTime,
		PktLoss,
		CmdOutput,
		Error
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
			&echo.Src,
			&echo.Dst,
			&echo.ResponseTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error)
		if err != nil {
			return nil, err
		}
		result = append(result, bwtest)
	}
	return result, nil
}

// ReadEchoItemsSince operates on the DB to return all echo rows
// which are more recent than the 'since' epoch in ms.
func ReadEchoItemsSince(since string) ([]EchoItem, error) {
	sqlReadSince := `
    SELECT
		Inserted,
		Src,
		Dst,
		ResponseTime,
		PktLoss,
		CmdOutput,
		Error
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
			&echo.Src,
			&echo.Dst,
			&echo.ResponseTime,
			&echo.PktLoss,
			&echo.CmdOutput,
			&echo.Error)
		if err != nil {
			return nil, err
		}
		result = append(result, echo)
	}
	return result, nil
}

// DeleteEchoItemsBefore operates on the DB to remote all bwtests rows
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