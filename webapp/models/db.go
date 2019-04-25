package models

import (
	"database/sql"
	"fmt"

	log "github.com/inconshreveable/log15"
)

var db *sql.DB
var bwDbVer = 2

// InitDB controls the opening connection to the database.
func InitDB(filepath string) error {
	var err error
	db, err = sql.Open("sqlite3", filepath)
	if err != nil {
		return err
	}
	err = db.Ping()
	return err
}

// CloseDB will close the database, only use when app closes.
func CloseDB() error {
	err := db.Close()
	return err
}

// LoadDB operates on the DB to load and/or migrate the database.
func LoadDB() error {
	err := createBwTestTable()
	if err != nil {
		return err
	}
	version, err := getUserVersion()
	if err != nil {
		return err
	}
	log.Info("Loading database version", "version", version)
	// add successive migrations here
	if version < 1 {
		err = addColumn("bwtests", "Path TEXT")
	}
	if version < 2 {
		err = addColumn("bwtests", "Log TEXT")
	}
	if err != nil {
		return err
	}
	//set updated version
	if version < bwDbVer {
		err := setUserVersion(bwDbVer)
		if err != nil {
			return err
		}
		log.Info("Migrated to database version", "version", bwDbVer)
	}
	return err
}

func addColumn(table string, column string) error {
	sqlAddCol := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s;`, table, column)
	log.Info(sqlAddCol)
	_, err := db.Exec(sqlAddCol)
	return err
}

func getUserVersion() (int, error) {
	sqlGetVersion := `PRAGMA user_version;`
	rows, err := db.Query(sqlGetVersion)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	var version int
	for rows.Next() {
		err := rows.Scan(&version)
		if err != nil {
			return version, err
		}
	}
	err = rows.Err()
	if err != nil {
		return version, err
	}
	return version, nil
}

func setUserVersion(version int) error {
	sqlSetVersion := fmt.Sprintf(`PRAGMA user_version = %d;`, version)
	log.Info(sqlSetVersion)
	_, err := db.Exec(sqlSetVersion)
	return err
}
