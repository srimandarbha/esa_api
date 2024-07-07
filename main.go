package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	handlers "github.com/srimandarbha/esa_dispatch/handlers"
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

func RetreiveApiData(instanceMap map[string]map[string]string) (ResultsMap, error) {
	results := make(ResultsMap)
	var wg sync.WaitGroup
	ch := make(chan struct {
		url        string
		univsearch []UnivSearch
		err        error
	}, len(instanceMap))

	for key, value := range instanceMap {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			fmt.Printf("Instance: %s, Url: %s, Token: %s\n", key, value["url"], value["token"])
			client := http.Client{}
			req, _ := http.NewRequest("GET", value["url"], nil)
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
		}(value["url"])
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

func ScheduledJob(fileDB *sql.DB, memDB *sql.DB, instanceMap map[string]map[string]string) {
	fmt.Println("Scheduled run at", time.Now())
	resultsmap, err := RetreiveApiData(instanceMap)
	if err != nil {
		fmt.Printf("Error retrieving API data: %v", err)
		return
	}
	now := time.Now()
	_, err = fileDB.Exec("CREATE TABLE IF NOT EXISTS activities ( id INTEGER NOT NULL PRIMARY KEY,  time DATETIME NOT NULL,  server TEXT, url TEXT );")
	fmt.Printf("activities table created on fileDB")
	checkErr(err)
	for url, univsearch := range resultsmap {
		fmt.Println(url)
		for k, v := range univsearch {
			insert_query := fmt.Sprintf("INSERT OR IGNORE INTO activities VALUES(%d,\"%s\",\"%s\", \"%s\");", k, now.Format(DDMMYYYYhhmmss), strings.ReplaceAll(v.Name, `"`, `""`), url)
			//fmt.Println(insert_query)
			_, err := fileDB.Exec(insert_query)
			checkErr(err)
		}
	}
	fmt.Println("Executing push of data from memory to filedb ")
	model.RestoreInMemoryDBToFile(fileDB, memDB, "activities")
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

func main() {
	propertiesFile := "config.json"
	if envFile := os.Getenv("CONFIG_FILE"); envFile != "" {
		propertiesFile = envFile
	}

	instances, instanceMap := readProperties(propertiesFile)

	mux := http.NewServeMux()
	dbInstance, err := model.DbInstance()
	checkErr(err)
	model.InitializeMemoryDB(dbInstance.FileDB)
	Scheduled := time.NewTicker(30 * time.Second)
	defer Scheduled.Stop()

	go func() {
		for t := range Scheduled.C {
			fmt.Println("Run at", t)
			ScheduledJob(dbInstance.FileDB, dbInstance.MemDB, instanceMap)
		}
	}()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("server")
		servers, err := model.QueryData(dbInstance.MemDB, dbInstance.FileDB, query)
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

	http.HandleFunc("/login", handlers.LoginHandler(dbInstance.FileDB))

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
		if err := dbInstance.FileDB.Close(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if err := dbInstance.MemDB.Close(); err != nil {
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
