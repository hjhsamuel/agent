package main

import (
	"fmt"
	"log"

	"github.com/hjhsamuel/agent/app"
)

var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

func printVersion() {
	fmt.Printf(`Agent %s:
  Commit: %s
  BuildTime: %s
`, Version, Commit, BuildTime)
}

func main() {
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
