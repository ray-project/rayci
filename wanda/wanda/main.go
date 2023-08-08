package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/wanda"
)

func main() {
	flag.Parse()

	args := flag.Args()

	if len(args) != 1 {
		log.Fatal("needs exactly one argument for the spec file")
	}

	if err := wanda.Make(args[0], nil); err != nil {
		log.Fatal(err)
	}
}
