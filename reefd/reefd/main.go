package main

import (
	"context"
	"flag"
	"log"

	"github.com/ray-project/rayci/reefd"
)

func main() {
	config := &reefd.Config{}
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	flag.Parse()

	args := flag.Args()
	if len(args) > 1 {
		log.Fatal("Usage: reefd takes 0 or 1 argument")
	}

	if len(args) == 0 {
		// serving by default.
		log.Printf("serving at %s", *addr)
		ctx := context.Background()
		if err := reefd.Serve(ctx, *addr, config); err != nil {
			log.Fatal(err)
		}
		return
	}

	cmd := args[0]
	switch cmd {
	case "reap-windows":
		log.Printf("reaping windows")
		ctx := context.Background()
		if err := reefd.ReapDeadWindowsInstances(ctx); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown command %q", cmd)
	}
}
