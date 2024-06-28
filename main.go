package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	model "github.com/srimandarbha/esa_dispatch/models"
)

var (
	startTime time.Time
	propObj   map[string]interface{}
	jsonObj   map[string]interface{}
	instances []string
)

func init() {
	startTime = time.Now()
}

type UpMetric struct {
	Uptime    time.Duration `json:"uptime"`
	Instances []string      `json:"instances"`
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func RetreiveApiData(url string) (*http.Response, error) {
	client := http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("accept", "application/json")
	res, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return res, nil
}

func ScheduledJob() {
	log.Println("Scheduled run at", time.Now())
	res, err := RetreiveApiData("http://universities.hipolabs.com/search")
	if err != nil {
		log.Println(err)
	}
	defer res.Body.Close()
	err = json.NewDecoder(res.Body).Decode(&jsonObj)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println(jsonObj)
}

func readProperties(properties_file string) map[string]interface{} {
	file, err := os.Open(properties_file)
	checkErr(err)
	defer file.Close()
	err = json.NewDecoder(file).Decode(&propObj)
	checkErr(err)
	return propObj
}

func main() {
	mux := http.NewServeMux()
	propfile := readProperties("config.json")
	fileDB, memDB, err := model.RestoreInMemoryDBFromFile("chinook.db", "employees")
	checkErr(err)
	memDB.Ping()
	fileDB.Ping()
	v := reflect.ValueOf(propfile["esa_instances"])
	for _, i := range v.MapKeys() {
		instances = append(instances, i.String())
	}
	Scheduled := time.NewTicker(5 * time.Second)
	defer Scheduled.Stop()

	go func() {
		for t := range Scheduled.C {
			log.Println("Run at", t)
			ScheduledJob()
		}
	}()

	upSince := UpMetric{
		Uptime:    time.Since(startTime),
		Instances: instances,
	}

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		data, err := json.Marshal(upSince)
		if err != nil {
			log.Printf("Write failed: %v", err)
		}
		w.Write(data)
	})

	log.Println("Listening on 127.0.0.1:3000")
	http.ListenAndServe(":3000", mux)
}
