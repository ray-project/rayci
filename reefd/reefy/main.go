package main

import (
	"context"
	"log"

	"github.com/ray-project/rayci/reefd/reefclient"
)

func main() {
	if err := reefclient.Main(context.Background()); err != nil {
		log.Fatal(err)
	}
}
