package main

import (
	"log"

	"github.com/hjhsamuel/agent/app"
)

func main() {
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
