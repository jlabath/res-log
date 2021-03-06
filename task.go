package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

//this decorator ensures we are called in decorator mode
func authDecor(next http.Handler) http.Handler {
	closure := func(w http.ResponseWriter, r *http.Request) {
		t, ok := r.Header["X-Appengine-Taskname"]
		if !ok || len(t[0]) == 0 {
			// You may use the presence of the X-Appengine-Taskname header to validate
			// the request comes from Cloud Tasks.
			log.Println("Invalid Task: No X-Appengine-Taskname request header found")
			http.Error(w, "Bad Request - Invalid Task", http.StatusBadRequest)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(closure)
}

func processHookView(w http.ResponseWriter, r *http.Request) {
	if err := processHook(r.Context(), r.Body); err != nil {
		log.Printf("Trouble %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "OK")
}

func saveResourceView(w http.ResponseWriter, r *http.Request) {
	var hook hookStruct
	err := json.NewDecoder(r.Body).Decode(&hook)
	if err != nil {
		log.Printf("trouble decoding %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := saveResource(r.Context(), &hook); err != nil {
		log.Printf("trouble saving %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "OK")
}

func purgeBeforeView(w http.ResponseWriter, r *http.Request) {
	var t time.Time
	err := json.NewDecoder(r.Body).Decode(&t)
	if err != nil {
		log.Printf("trouble decoding %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := purgeBefore(r.Context(), t, ""); err != nil {
		log.Printf("trouble purging with time %v: %v", t, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "OK")

}

func purgeStepView(w http.ResponseWriter, r *http.Request) {
	var arg LaterStepArgs
	err := json.NewDecoder(r.Body).Decode(&arg)
	if err != nil {
		log.Printf("trouble reading request body: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := purgeBefore(r.Context(), arg.When, arg.Cursor); err != nil {
		log.Printf("trouble purging with cursor: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "OK")
}
