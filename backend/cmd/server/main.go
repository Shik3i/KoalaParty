package main

import (
	"fmt"
	"os"

	"github.com/Shik3i/KoalaParty/backend/internal/app"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		info := app.CurrentBuildInformation()
		fmt.Printf("KoalaParty %s (commit %s, built %s)\n", info.Version, info.Commit, info.BuildDate)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := app.Healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "operator" {
		if err := app.Operator(os.Args[2:], os.Stdout); err != nil {
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
