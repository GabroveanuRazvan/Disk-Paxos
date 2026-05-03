package main

import (
	"disk-paxos/internal/config"
	"fmt"
	"log"
)

func main() {

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(cfg)
}
