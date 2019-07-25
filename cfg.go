package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

var cfg struct {
	AppKey string
}

func init() {
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
		return
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for {
		if derr := dec.Decode(&cfg); derr == io.EOF {
			break
		} else if derr != nil {
			log.Fatal(derr)
			break
		}
	}
}
