package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/reefd"
)

func main() {
	config := &reefd.Config{}
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	flag.Parse()

	log.Printf("serving at %s", *addr)
	if err := reefd.Serve(*addr, config); err != nil {
		log.Fatal(err)
	}
}
