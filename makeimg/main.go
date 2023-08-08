package main

import (
	"flag"
	"log"

	"github.com/ray-project/rayci/imgspec"
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Fatal("needs exactly one argument for the spec file")
	}

	if err := imgspec.MakeImage(args[0]); err != nil {
		log.Fatal(err)
	}
}
