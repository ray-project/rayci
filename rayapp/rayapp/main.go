package main

import (
	"flag"
	"fmt"
	"github.com/ray-project/rayci/rayapp"
	"log"
)

func main() {
	base := flag.String("base", ".", "base directory")
	output := flag.String("output", "_build", "output directory")
	buildFile := flag.String("build", "BUILD.yaml", "build file")

	flag.Parse()

	args := flag.Args()
	switch args[0] {
	case "build-all":
		if err := rayapp.BuildAll(*buildFile, *base, *output); err != nil {
			log.Fatal(err)
		}
	case "build":
		if err := rayapp.Build(*buildFile, args[1], *base, *output); err != nil {
			log.Fatal(err)
		}
	case "help":
		fmt.Println("Usage: rayapp build-all | build <template-name> | help")
	default:
		log.Fatal("unknown command")
	}
}
