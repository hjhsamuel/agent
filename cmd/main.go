package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hjhsamuel/agent/app"
)

var (
	Version   = ""
	CommitID  = ""
	BuildTime = ""
)

func printVersion() {
	fmt.Printf(`cagent %s
CommitID: %s
BuildTime: %s
`, Version, CommitID, BuildTime)
}

func main() {
	var (
		showVersion = flag.Bool("v", false, "show version")
	)
	flag.Parse()

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
