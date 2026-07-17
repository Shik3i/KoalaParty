package main

import (
	"fmt"
	"os"

	"github.com/Shik3i/KoalaParty/backend/internal/app"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := app.Healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
