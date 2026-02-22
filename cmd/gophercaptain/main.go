package main

import (
	"os"

	"github.com/ecairns22/GopherCaptain/cmd/gophercaptain/commands"
)

func main() {
	if err := commands.Root().Execute(); err != nil {
		os.Exit(1)
	}
}
