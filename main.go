package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"
)

var (
	startTime time.Time
	propObj   map[string]interface{}
	instances []string
)

func init() {
	startTime = time.Now()
}

type UpMetric struct {
	Uptime    time.Duration `json:"uptime"`
	Instances []string      `json:"instances"`
}

func ScheduledJob() {
	log.Println("Scheduled run at", time.Now())
}

func readProperties(properties_file string) map[string]interface{} {
	file, err := os.Open(properties_file)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	if json.NewDecoder(file).Decode(&propObj); err != nil {
		log.Fatal(err)
	}
	return propObj
}

func main() {
	model.dbConn()
	mux := http.NewServeMux()
	propfile := readProperties("config.json")
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
