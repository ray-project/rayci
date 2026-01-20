package main

import (
	"log"

	"github.com/ray-project/rayci/goqualgate"
)

func main() {
	if err := goqualgate.Main(); err != nil {
		log.Fatal(err)
	}
}
