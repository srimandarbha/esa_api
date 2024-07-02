package model

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	err        error
	filePath   = "chinook.db"
	memoryPath = "file:chinook.db?mode=memory&cache=shared"
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
	memDB, _ := sql.Open("sqlite3", memoryPath)

	return db.Conn, memDB, nil
}

func DbInstance() (*sql.DB, *sql.DB, error) {
	db := Db{}
	diskDB, memDB, err := db.Init()
	return diskDB, memDB, err
}

func RestoreInMemoryDBToFile(fileDB *sql.DB, tblName string) {
	fileConn, err := fileDB.Conn(context.TODO())
	if err != nil {
		fmt.Println(err)
		return
	}
	defer fileConn.Close()
	temp_query := fmt.Sprintf("ATTACH DATABASE '%s' AS file_db ; DROP TABLE  IF EXISTS activities; create table %s as select * from file_db.activities; DETACH DATABASE file_db", memoryPath, tblName)
	fmt.Println(temp_query)
	_, err = fileConn.ExecContext(context.TODO(), temp_query)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("Restore from In-Memory DB to filedb completed")
}
