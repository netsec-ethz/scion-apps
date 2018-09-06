package models

import (
    "fmt"
    "log"
    "reflect"
    "strconv"
    "time"
)

var bwTestDbExpire = time.Duration(24) * time.Hour

// BwTestItem reflects one row in the bwtests table will all columns.
type BwTestItem struct {
    Inserted       int64 // ms
    ActualDuration int   // ms
    CIa            string
    CAddr          string
    CPort          int
    SIa            string
    SAddr          string
    SPort          int
    CSDuration     int // ms
    CSPackets      int // packets
    CSPktSize      int // bytes
    CSBandwidth    int // bps
    CSThroughput   int // bps
    CSArrVar       int // ms
    CSArrAvg       int // ms
    CSArrMin       int // ms
    CSArrMax       int // ms
    SCDuration     int // ms
    SCPackets      int // packets
    SCPktSize      int // bytes
    SCBandwidth    int // bps
    SCThroughput   int // bps
    SCArrVar       int // ms
    SCArrAvg       int // ms
    SCArrMin       int // ms
    SCArrMax       int // ms
    Error          string
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
// neccessary items to display in a graph.
type BwTestGraph struct {
    Inserted       int64
    ActualDuration int
    CSBandwidth    int
    CSThroughput   int
    SCBandwidth    int
    SCThroughput   int
    Error          string
}

// CreateBwTestTable operates on the DB to create the bwtests table.
func CreateBwTestTable() {
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
    if err != nil {
        panic(err)
    }
}

// StoreBwTestItem operates on the DB to insert a BwTestItem.
func StoreBwTestItem(bwtest *BwTestItem) {
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
        Error
    ) values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
    stmt, err := db.Prepare(sqlInsert)
    if err != nil {
        panic(err)
    }
    defer stmt.Close()

    _, err2 := stmt.Exec(
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
        bwtest.Error)
    if err2 != nil {
        panic(err2)
    }
}

// ReadBwTestItemsAll operates on the DB to return all bwtests rows.
func ReadBwTestItemsAll() []BwTestItem {
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
        Error
    FROM bwtests
    ORDER BY datetime(Inserted) DESC
    `
    rows, err := db.Query(sqlReadAll)
    if err != nil {
        panic(err)
    }
    defer rows.Close()

    var result []BwTestItem
    for rows.Next() {
        bwtest := BwTestItem{}
        err2 := rows.Scan(
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
            &bwtest.Error)
        if err2 != nil {
            panic(err2)
        }
        result = append(result, bwtest)
    }
    return result
}

// ReadBwTestItemsSince operates on the DB to return all bwtests rows
// which are more recent than the 'since' epoch in ms.
func ReadBwTestItemsSince(since string) []BwTestGraph {
    sqlReadSince := `
    SELECT
        Inserted,
        ActualDuration,
        CSBandwidth,
        CSThroughput,
        SCBandwidth,
        SCThroughput,
        Error
    FROM bwtests
    WHERE Inserted > ?
    ORDER BY datetime(Inserted) DESC
    `
    rows, err := db.Query(sqlReadSince, since)
    if err != nil {
        panic(err)
    }
    defer rows.Close()

    var result []BwTestGraph
    for rows.Next() {
        bwtest := BwTestGraph{}
        err2 := rows.Scan(
            &bwtest.Inserted,
            &bwtest.ActualDuration,
            &bwtest.CSBandwidth,
            &bwtest.CSThroughput,
            &bwtest.SCBandwidth,
            &bwtest.SCThroughput,
            &bwtest.Error)
        if err2 != nil {
            panic(err2)
        }
        result = append(result, bwtest)
    }
    return result
}

// DeleteBwTestItemsBefore operates on the DB to remote all bwtests rows
// which are more older than the 'before' epoch in ms.
func DeleteBwTestItemsBefore(before string) int64 {
    sqlDeleteBefore := `
    DELETE FROM bwtests
    WHERE Inserted < ?
    `
    res, err := db.Exec(sqlDeleteBefore, before)
    if err != nil {
        panic(err)
    }
    count, err := res.RowsAffected()
    if err != nil {
        panic(err)
    }
    return count
}

// MaintainDatabase is a goroutine that runs independanly to cleanup the
// database according to the defined schedule.
func MaintainDatabase() {
    for {
        before := time.Now().Add(-bwTestDbExpire)
        count := DeleteBwTestItemsBefore(strconv.FormatInt(before.UnixNano()/1e6, 10))
        if count > 0 {
            log.Println("Deleting", count, "bwtests db rows older than", bwTestDbExpire)
        }
        time.Sleep(bwTestDbExpire)
    }
}
