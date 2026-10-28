package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hjhsamuel/agent/app"
)

var (
	Version   = "dev"
	BuildTime = ""
)

func printVersion() {
	fmt.Printf(`CAgent %s %s\n`, Version, BuildTime)
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
