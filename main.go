package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	http.Handle("/", getMux())
	//http.HandleFunc("/", http.NotFound)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	//if not running in PROD say as much
	if os.Getenv("IN_PROD") == "" {
		log.Printf("Development environment")
		http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
