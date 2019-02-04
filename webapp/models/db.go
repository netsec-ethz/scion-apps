package models

import (
    "database/sql"
    "fmt"

    log "github.com/inconshreveable/log15"
)

var db *sql.DB
var bwDbVer = 1

// InitDB controls the opening connection to the database.
func InitDB(filepath string) {
    var err error
    db, err = sql.Open("sqlite3", filepath)
    if err != nil {
        log.Error("sql.Open", err)
        panic(err)
    }

    if err = db.Ping(); err != nil {
        log.Error("db.Ping()", err)
        panic(err)
    }
}

// CloseDB will close the database, only use when app closes.
func CloseDB() {
    var err error
    if err = db.Close(); err != nil {
        log.Error("db.Close()", err)
        panic(err)
    }
}

// LoadDB operates on the DB to load and/or migrate the database.
func LoadDB() {
    createBwTestTable()
    version := getUserVersion()
    log.Info("Loading database version:", "version", version)
    // add successive migrations here
    if version < 1 {
        addColumn("bwtests", "Path TEXT")
    }
    //set updated version
    if version != bwDbVer {
        setUserVersion(bwDbVer)
        log.Info("Migrated to database version:", "version", bwDbVer)
    }
}

func addColumn(table string, column string) {
    sqlAddCol := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s;`, table, column)
    log.Info(sqlAddCol)
    _, err := db.Exec(sqlAddCol)
    if err != nil {
        log.Error("db.Exec(sqlAddCol)", err)
        panic(err)
    }
}

func getUserVersion() int {
    sqlGetVersion := `PRAGMA user_version;`
    rows, err := db.Query(sqlGetVersion)
    if err != nil {
        log.Error("db.Query(sqlGetVersion)", err)
        panic(err)
    }
    defer rows.Close()
    var version int
    for rows.Next() {
        if err := rows.Scan(&version); err != nil {
            log.Error("rows.Scan(&version)", err)
            panic(err)
        }
    }
    if err := rows.Err(); err != nil {
        log.Error("rows.Err() get version", err)
        panic(err)
    }
    return version
}

func setUserVersion(version int) {
    sqlSetVersion := fmt.Sprintf(`PRAGMA user_version = %d;`, version)
    log.Info(sqlSetVersion)
    _, err := db.Exec(sqlSetVersion)
    if err != nil {
        log.Error("db.Exec(sqlSetVersion)", err)
        panic(err)
    }
}
