package main

import (
	"log"
	"os"

	"github.com/kaleb110/go-touch-grass/cli"
)

func main() {
	app := cli.NewApp()

	app.Register(cli.NewTracker())

	if err := app.Run(); err != nil {
		log.Printf("Error: %v", err)
		os.Exit(1)
	}
}
