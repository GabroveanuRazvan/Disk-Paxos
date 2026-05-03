package main

import (
	"fmt"
	"log"

	"disk-paxos/internal/config"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(cfg)
}
