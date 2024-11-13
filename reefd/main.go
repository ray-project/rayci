package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	_ "github.com/mattn/go-sqlite3"
	"encoding/json"
)

type MachineConfigInput struct {
	Name string `json:"name"`
	Configuration string `json:"configuration"`
}

type MachineConfig struct {
	id int
	name string
	configuration string
}

// a function to connect to database given a path to .db file
func connectToDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// a function to handle GET request to /machine/configuration that query the machine config based on name
func handleGetRequest(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Parse the query parameter
	name := r.URL.Query().Get("name")

	var rows *sql.Rows
	var err error
	// If name is specified, query the machine config with that name
	if name != "" {
        rows, err = db.Query("SELECT id, name, configuration FROM machine_config WHERE name = ?", name)
    } else {
        rows, err = db.Query("SELECT id, name, configuration FROM machine_config")
    }
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Iterate through the queried rows and print out
	var machineConfigs []MachineConfig
	for rows.Next() {
		var machineConfig MachineConfig
		err := rows.Scan(&machineConfig.id, &machineConfig.name, &machineConfig.configuration)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		machineConfigs = append(machineConfigs, machineConfig)
	}
	if err = rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, machineConfig := range machineConfigs {
		fmt.Fprintf(w, "ID: %d, Name: %s, Configuration: %s\n", machineConfig.id, machineConfig.name, machineConfig.configuration)
	}
}

//a function to handle POST request in MachineConfig format to add a machine configuration to the database
func handlePostRequestMachineConfig(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Make sure the content type is JSON
    if r.Header.Get("Content-Type") != "application/json" {
        http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
        return
    }

	// Parse the JSON body
    config := &MachineConfigInput{}
	err := json.NewDecoder(r.Body).Decode(config)
	if err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}

    // Insert into database
    _, err = db.Exec("INSERT INTO machine_config (name, configuration) VALUES (?, ?)", 
	config.Name, config.Configuration)
    if err != nil {
        http.Error(w, "Database error: "+err.Error(), http.StatusInternalServerError)
        return
    }
}

func main() {
	dbPath := flag.String("db", "reef.db", "Path to .db file")
	flag.Parse()
	if _, err := os.Stat(*dbPath); os.IsNotExist(err) {
		log.Fatalf("File %s does not exist", *dbPath)
	}

	db, err := connectToDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Error connecting to database: %s", err)
	}
	defer db.Close()

	// handle both GET and POST request to /machine/configuration
	http.HandleFunc("/machine/configuration", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetRequest(db, w, r)
		case http.MethodPost:
			handlePostRequestMachineConfig(db, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	log.Printf("Starting server")
	http.ListenAndServe(":1234", nil)
}
