package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	model "github.com/srimandarbha/esa_dispatch/models"
)

var (
	startTime      time.Time
	propObj        map[string]interface{}
	instances      []string
	DDMMYYYYhhmmss = "2006-01-02 15:04:05"
)

func init() {
	startTime = time.Now()
}

type ServerDetails struct {
	Id         int64  `json:"id"`
	Time       string `json:"checkintime"`
	ServerName string `json:"server"`
}

type UnivSearch struct {
	Code     string
	Name     string
	Domains  []string
	WPages   []string
	Country  string
	Location string
}

type UpMetric struct {
	Uptime    time.Duration `json:"uptime"`
	Instances []string      `json:"instances"`
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		return
	}
}

func RetreiveApiData(url string) (*http.Response, error) {
	client := http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return res, nil
}

func ScheduledJob(fileDB *sql.DB, memDB *sql.DB) {
	fmt.Println("Scheduled run at", time.Now())
	res, err := RetreiveApiData("http://universities.hipolabs.com/search")
	if err != nil {
		fmt.Println(err)
		return
	}
	var univsearch []UnivSearch
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&univsearch)
	if err != nil {
		fmt.Println(err)
		return
	}
	now := time.Now()
	_, err = memDB.Exec("CREATE TABLE IF NOT EXISTS activities ( id INTEGER NOT NULL PRIMARY KEY,  time DATETIME NOT NULL,  server TEXT  );")
	checkErr(err)
	for k, v := range univsearch {
		insert_query := fmt.Sprintf("INSERT OR IGNORE INTO activities VALUES(%d,\"%s\",\"%s\");", k, now.Format(DDMMYYYYhhmmss), v.Name)
		fmt.Println(insert_query)
		_, err := memDB.Exec(insert_query)
		checkErr(err)
	}
	model.RestoreInMemoryDBToFile(fileDB, "activities")
}

func readProperties(properties_file string) map[string]interface{} {
	file, err := os.Open(properties_file)
	checkErr(err)
	defer file.Close()
	err = json.NewDecoder(file).Decode(&propObj)
	checkErr(err)
	return propObj
}

func queryFileDB(fileDB *sql.DB, query string) ([]ServerDetails, error) {
	var servers []ServerDetails

	rows, err := fileDB.Query("SELECT * FROM activities WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect results
	for rows.Next() {
		var server ServerDetails
		if err := rows.Scan(&server.Id, &server.Time, &server.ServerName); err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}

func queryData(memDB *sql.DB, fileDB *sql.DB, query string) ([]ServerDetails, error) {
	var servers []ServerDetails

	// Try to query the in-memory database
	rows, err := memDB.Query("SELECT * FROM activities WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		// If there's an error, fall back to the file-based database
		fmt.Println("Error querying in-memory DB:", err)
		return queryFileDB(fileDB, query)
	}
	defer rows.Close()

	// Collect results
	for rows.Next() {
		var server ServerDetails
		if err := rows.Scan(&server.Id, &server.Time, &server.ServerName); err != nil {
			// If there's an error, fall back to the file-based database
			fmt.Println("Error scanning rows in-memory DB:", err)
			return queryFileDB(fileDB, query)
		}
		servers = append(servers, server)
	}
	return servers, nil
}

// Handler function for the search endpoint

func main() {
	mux := http.NewServeMux()
	propfile := readProperties("config.json")
	diskDB, memDB, err := model.DbInstance()
	checkErr(err)
	v := reflect.ValueOf(propfile["esa_instances"])
	for _, i := range v.MapKeys() {
		instances = append(instances, i.String())
	}
	Scheduled := time.NewTicker(30 * time.Second)
	defer Scheduled.Stop()

	go func() {
		for t := range Scheduled.C {
			fmt.Println("Run at", t)
			ScheduledJob(diskDB, memDB)
		}
	}()

	upSince := UpMetric{
		Uptime:    time.Since(startTime),
		Instances: instances,
	}
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("server")
		// Query data from the databases
		servers, err := queryData(memDB, diskDB, query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(servers)
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(upSince)
		if err != nil {
			fmt.Printf("Write failed: %v", err)
		}
		w.Write(data)
	})

	defer diskDB.Close()
	defer memDB.Close()

	fmt.Println("Listening on 127.0.0.1:3000")
	http.ListenAndServe(":3000", mux)
}
