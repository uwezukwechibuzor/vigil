//go:build ignore

// Example: check Base chain health.
package main

import (
	"context"
	"fmt"
	"log"

	vigil "github.com/uwezukwechibuzor/vigil/sdks/go"
)

func main() {
	client := vigil.New() // reads VIGIL_API_KEY from env
	health, err := client.ChainHealth(context.Background(), 8453)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Base health: %s | fee: %s\n", health.Status, health.FeeLevel)
}
