package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kaleb110/go-touch-grass/cli"
)

func main() {
	app := cli.NewApp()

	app.Register(cli.NewTracker())
	app.Register(cli.NewInstall())

	if err := app.Run(); err != nil {
		// flag.ExitOnError already exits for parse errors; only reach here
		// for runtime errors, which we surface plainly.
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		log.Printf("go-touch-grass: %v", err)
		os.Exit(1)
	}
}
