package models

import (
    "database/sql"
    "fmt"

    log "github.com/inconshreveable/log15"
    . "github.com/netsec-ethz/scion-apps/webapp/util"
)

var db *sql.DB
var bwDbVer = 2

// InitDB controls the opening connection to the database.
func InitDB(filepath string) {
    var err error
    db, err = sql.Open("sqlite3", filepath)
    if CheckError(err) {
        panic(err)
    }
    err = db.Ping()
    if CheckError(err) {
        panic(err)
    }
}

// CloseDB will close the database, only use when app closes.
func CloseDB() {
    err := db.Close()
    if CheckError(err) {
        panic(err)
    }
}

// LoadDB operates on the DB to load and/or migrate the database.
func LoadDB() {
    createBwTestTable()
    version := getUserVersion()
    log.Info("Loading database version", "version", version)
    // add successive migrations here
    if version < 1 {
        addColumn("bwtests", "Path TEXT")
    }
    if version < 2 {
        addColumn("bwtests", "Log TEXT")
    }
    //set updated version
    if version < bwDbVer {
        setUserVersion(bwDbVer)
        log.Info("Migrated to database version", "version", bwDbVer)
    }
}

func addColumn(table string, column string) {
    sqlAddCol := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s;`, table, column)
    log.Info(sqlAddCol)
    _, err := db.Exec(sqlAddCol)
    if CheckError(err) {
        panic(err)
    }
}

func getUserVersion() int {
    sqlGetVersion := `PRAGMA user_version;`
    rows, err := db.Query(sqlGetVersion)
    if CheckError(err) {
        panic(err)
    }
    defer rows.Close()
    var version int
    for rows.Next() {
        err := rows.Scan(&version)
        if CheckError(err) {
            panic(err)
        }
    }
    err = rows.Err()
    if CheckError(err) {
        panic(err)
    }
    return version
}

func setUserVersion(version int) {
    sqlSetVersion := fmt.Sprintf(`PRAGMA user_version = %d;`, version)
    log.Info(sqlSetVersion)
    _, err := db.Exec(sqlSetVersion)
    if CheckError(err) {
        panic(err)
    }
}
