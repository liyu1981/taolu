package main

import (
	"context"
	"log"
	"os"
)

func main() {
	server, err := newServer()
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	if err := server.Run(context.Background(), newTransport()); err != nil {
		log.Printf("Server failed: %v", err)
		os.Exit(1)
	}
}
