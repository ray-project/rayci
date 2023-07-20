package main

import (
	"log"
	"os"

	"github.com/ray-project/rayci/raycicmd"
)

func main() {
	if err := raycicmd.Main(os.Args, nil); err != nil {
		log.Fatal(err)
	}
}
