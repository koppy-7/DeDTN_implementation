package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"projects/jetbotPBFT/pbft"
)

func main() {
	if err := pbft.LoadDotEnv(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "warn: failed to load .env: %v\n", err)
	}

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go config/A.json")
	}
	configFile := os.Args[1]

	f, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	var cfg pbft.Config
	if err := json.Unmarshal(f, &cfg); err != nil {
		log.Fatal(err)
	}

	node := pbft.NewNode(cfg)
	node.Start()

	if cfg.IsPrimary {

		node.WatchSemanticFile("/workspace/semantic/latest.json")

		fmt.Println("This node is Primary. Sending proposal...")
	}

	select {}
}
