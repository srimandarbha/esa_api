package model

import (
	"database/sql"
	"fmt"
)

type ServerDetails struct {
	Id         int64  `json:"id"`
	Time       string `json:"checkintime"`
	ServerName string `json:"server"`
	Url        string `json:"url"`
}

func queryFileDB(fileDB *sql.DB, query string) ([]ServerDetails, error) {
	var servers []ServerDetails
	fmt.Println("Query fetch from fileDB ")
	rows, err := fileDB.Query("SELECT * FROM activities WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var server ServerDetails
		if err := rows.Scan(&server.Id, &server.Time, &server.ServerName, &server.Url); err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func queryData(memDB *sql.DB, fileDB *sql.DB, query string) ([]ServerDetails, error) {
	var servers []ServerDetails

	rows, err := memDB.Query("SELECT * FROM activities_cache WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		fmt.Printf("Error querying in-memory DB: %v", err)
		return queryFileDB(fileDB, query)
	}
	defer rows.Close()

	for rows.Next() {
		var server ServerDetails
		if err := rows.Scan(&server.Id, &server.Time, &server.ServerName, &server.Url); err != nil {
			fmt.Printf("Error scanning rows in-memory DB: %v", err)
			return queryFileDB(fileDB, query)
		}
		servers = append(servers, server)
	}
	return servers, nil
}
