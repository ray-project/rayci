package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ray-project/rayci/reefd"
	"github.com/tailscale/hujson"
)

func readConfig(path string) (*reefd.Config, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	bs, err = hujson.Standardize(bs)
	if err != nil {
		return nil, fmt.Errorf("standardize config file: %w", err)
	}

	config := &reefd.Config{}
	if err := json.Unmarshal(bs, config); err != nil {
		return nil, fmt.Errorf("unmarshal config file: %w", err)
	}
	return config, nil
}

func main() {
	addr := flag.String("addr", "localhost:8000", "address to listen on")
	configFile := flag.String("config", "config.hujson", "path to the config file")
	flag.Parse()

	args := flag.Args()
	if len(args) > 1 {
		log.Fatal("Usage: reefd takes 0 or 1 argument")
	}

	config, err := readConfig(*configFile)
	if err != nil {
		log.Fatalf("read config file: %v", err)
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
