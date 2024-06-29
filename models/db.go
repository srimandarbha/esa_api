package model

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	err      error
	filePath = "chinook.db?mode=memory&cache=shared"
)

type Db struct {
	Conn *sql.DB
}

func (db *Db) Init() (*sql.DB, *sql.DB, error) {
	// Connect to the file-based database
	db.Conn, err = sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, nil, err
	}
	memDB, _ := sql.Open("sqlite3", "file::memory:?cache=shared")

	return db.Conn, memDB, nil
}

func DbInstance() (*sql.DB, *sql.DB, error) {
	db := Db{}
	diskDB, memDB, err := db.Init()
	return diskDB, memDB, err
}

func RestoreInMemoryDBFromFile(memDB *sql.DB, tblName string) {
	memConn, err := memDB.Conn(context.TODO())
	if err != nil {
		memDB.Close()
		return
	}
	defer memConn.Close()
	temp_query := fmt.Sprintf("ATTACH DATABASE '%s' AS file_db ; DROP TABLE  IF EXISTS emp; create TEMP table emp as select * from file_db.%s; DETACH DATABASE file_db", filePath, tblName)
	_, err = memConn.ExecContext(context.TODO(), temp_query)
	if err != nil {
		memDB.Close()
		return
	}
	fmt.Println("Restore to In-Memory DB from filedb completed")
}
