package models

import (
    "database/sql"
    "fmt"
    "log"
)

var db *sql.DB
var bwDbVer = 1

// InitDB controls the opening connection to the database.
func InitDB(filepath string) {
    var err error
    db, err = sql.Open("sqlite3", filepath)
    if err != nil {
        log.Panic(err)
    }

    if err = db.Ping(); err != nil {
        log.Panic(err)
    }
}

// CloseDB will close the database, only use when app closes.
func CloseDB() {
    var err error
    if err = db.Close(); err != nil {
        log.Panic(err)
    }
}

// LoadDB operates on the DB to load and/or migrate the database.
func LoadDB() {
    createBwTestTable()
    version := getUserVersion()
    log.Printf("Database version: %d", version)
    // add successive migrations here
    if version < 1 {
        addColumn("bwtests", "Path TEXT")
    }
    //set updated version
    if version != bwDbVer {
        setUserVersion(bwDbVer)
        log.Printf("Migrated to database version: %d", bwDbVer)
    }
}

func addColumn(table string, column string) {
    sqlAddCol := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s;`, table, column)
    log.Printf(sqlAddCol)
    res, err := db.Exec(sqlAddCol)
    fmt.Println(res)
    if err != nil {
        log.Println(err.Error())
        panic(err)
    }
}

func getUserVersion() int {
    sqlGetVersion := `PRAGMA user_version;`
    rows, err := db.Query(sqlGetVersion)
    if err != nil {
        log.Println(err.Error())
        panic(err)
    }
    defer rows.Close()
    var version int
    for rows.Next() {
        if err := rows.Scan(&version); err != nil {
            log.Println(err.Error())
            panic(err)
        }
    }
    if err := rows.Err(); err != nil {
        log.Println(err.Error())
        panic(err)
    }
    return version
}

func setUserVersion(version int) {
    sqlSetVersion := fmt.Sprintf(`PRAGMA user_version = %d;`, version)
    log.Printf(sqlSetVersion)
    res, err := db.Exec(sqlSetVersion)
    fmt.Println(res)
    if err != nil {
        log.Println(err.Error())
        panic(err)
    }
}
