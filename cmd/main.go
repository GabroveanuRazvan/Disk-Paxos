package main

import (
	"context"
	"disk-paxos/internal/config"
	"disk-paxos/internal/processor"
	"fmt"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Panic(err)
	}
	fmt.Println(cfg)

	p := processor.NewProcessor(1, cfg)
	if err := p.WriteToDisks(context.Background()); err != nil {
		log.Panic(err)
	}

	for i := range 3 {
		value := fmt.Sprintf("value %d", i)
		if err := p.Propose(value); err != nil {
			log.Println(err)
		}

	}

}
