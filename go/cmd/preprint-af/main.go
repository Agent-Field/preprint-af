package main

import (
	"context"
	"log"

	"github.com/Agent-Field/preprint-af/go/internal/node"
)

func main() {
	n, err := node.Build()
	if err != nil {
		log.Fatal(err)
	}
	n.RegisterAll()
	log.Fatal(n.App.Run(context.Background()))
}
