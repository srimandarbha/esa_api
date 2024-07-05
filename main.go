package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	model "github.com/srimandarbha/esa_dispatch/models"
)

var (
	startTime      time.Time
	instances      []string
	DDMMYYYYhhmmss = "2006-01-02 15:04:05"
	instanceMap    = make(map[string]map[string]string)
)

func init() {
	startTime = time.Now()
}

type InstanceDetails struct {
	Url   string `json:"url"`
	Token string `json:"token"`
}

// Define the Config struct
type Config struct {
	EsaInstances map[string]InstanceDetails `json:"esa_instances"`
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

type ResultsMap map[string][]UnivSearch

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		return
	}
}

func RetreiveApiData(urls []string) (ResultsMap, error) {
	for key, value := range instanceMap {
		fmt.Printf("Instance: %s, Key: %s, Value: %s \n", key, value["url"], value["token"])
	}
	results := make(ResultsMap)
	var wg sync.WaitGroup
	ch := make(chan struct {
		url        string
		univsearch []UnivSearch
		err        error
	}, len(urls))

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			client := http.Client{}
			req, _ := http.NewRequest("GET", url, nil)
			req.Header.Set("accept", "application/json")
			res, err := client.Do(req)
			if err != nil {
				ch <- struct {
					url        string
					univsearch []UnivSearch
					err        error
				}{url, nil, err}
				return
			}
			defer res.Body.Close()
			var univsearch []UnivSearch
			err = json.NewDecoder(res.Body).Decode(&univsearch)
			ch <- struct {
				url        string
				univsearch []UnivSearch
				err        error
			}{url, univsearch, err}
		}(url)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for result := range ch {
		if result.err != nil {
			fmt.Printf("Error fetching data from URL: %s, Error: %v", result.url, result.err)
			continue
		}
		results[result.url] = result.univsearch
	}

	return results, nil
}

func ScheduledJob(fileDB *sql.DB, memDB *sql.DB, urls []string) {
	fmt.Println("Scheduled run at", time.Now())
	resultsmap, err := RetreiveApiData(urls)
	if err != nil {
		fmt.Printf("Error retrieving API data: %v", err)
		return
	}
	now := time.Now()
	_, err = memDB.Exec("CREATE TABLE IF NOT EXISTS activities ( id INTEGER NOT NULL PRIMARY KEY,  time DATETIME NOT NULL,  server TEXT  );")
	checkErr(err)
	for url, univsearch := range resultsmap {
		fmt.Println(url)
		for k, v := range univsearch {
			insert_query := fmt.Sprintf("INSERT OR IGNORE INTO activities VALUES(%d,\"%s\",\"%s\");", k, now.Format(DDMMYYYYhhmmss), v.Name)
			fmt.Println(insert_query)
			_, err := memDB.Exec(insert_query)
			checkErr(err)
		}
	}
	fmt.Println("Executing push of data from memory to filedb ")
	model.RestoreInMemoryDBToFile(fileDB, "activities")
}

func readProperties(propertiesFile string) ([]string, map[string]map[string]string) {
	jsonFile, err := os.Open(propertiesFile)
	checkErr(err)
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	checkErr(err)

	var config Config

	err = json.Unmarshal(byteValue, &config)
	checkErr(err)

	for key, details := range config.EsaInstances {
		instanceMap[key] = map[string]string{
			"url":   details.Url,
			"token": details.Token,
		}
		instances = append(instances, details.Url)
	}
	return instances, instanceMap
}

func queryFileDB(fileDB *sql.DB, query string) ([]ServerDetails, error) {
	var servers []ServerDetails

	rows, err := fileDB.Query("SELECT * FROM activities WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

	rows, err := memDB.Query("SELECT * FROM activities WHERE server LIKE ?", "%"+query+"%")
	if err != nil {
		fmt.Printf("Error querying in-memory DB: %v", err)
		return queryFileDB(fileDB, query)
	}
	defer rows.Close()

	for rows.Next() {
		var server ServerDetails
		if err := rows.Scan(&server.Id, &server.Time, &server.ServerName); err != nil {
			fmt.Printf("Error scanning rows in-memory DB: %v", err)
			return queryFileDB(fileDB, query)
		}
		servers = append(servers, server)
	}
	return servers, nil
}

func main() {
	propertiesFile := "config.json"
	if envFile := os.Getenv("CONFIG_FILE"); envFile != "" {
		propertiesFile = envFile
	}

	instances, instanceMap := readProperties(propertiesFile)
	urls := []string{
		"http://universities.hipolabs.com/search",
	}

	mux := http.NewServeMux()
	diskDB, memDB, err := model.DbInstance()
	checkErr(err)

	fmt.Println(instanceMap)
	Scheduled := time.NewTicker(30 * time.Second)
	defer Scheduled.Stop()

	go func() {
		for t := range Scheduled.C {
			fmt.Println("Run at", t)
			ScheduledJob(diskDB, memDB, urls)
		}
	}()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("server")
		servers, err := queryData(memDB, diskDB, query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(servers)
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		upSince := UpMetric{
			Uptime:    time.Since(startTime),
			Instances: instances,
		}
		data, err := json.Marshal(upSince)
		if err != nil {
			fmt.Printf("Write failed: %v", err)
		}
		w.Write(data)
	})

	server := &http.Server{
		Addr:    ":3000",
		Handler: mux,
	}

	// Graceful shutdown handling
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("Shutting down server...")

		if err := server.Close(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := diskDB.Close(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := memDB.Close(); err != nil {
			fmt.Println("memDB Close:", err)
			os.Exit(0)
		}
		fmt.Println("Server gracefully stopped")
	}()

	fmt.Println("Listening on 127.0.0.1:3000")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Println(err)
	}
}
