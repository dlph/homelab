package main

import (
	"log"

	"github.com/dlph/homelab/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
