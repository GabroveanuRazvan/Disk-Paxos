package main

import (
	"context"
	"disk-paxos/internal/processor"
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

	p := processor.NewProcessor(0, cfg)
	if err := p.WriteToDisks(context.Background()); err != nil {
		log.Panic(err)
	}

}
