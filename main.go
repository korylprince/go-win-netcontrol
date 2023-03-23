package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

func main() {
	if len(os.Args) > 1 {
		ctx := kong.Parse(&CLI, kong.ConfigureHelp(kong.HelpOptions{NoExpandSubcommands: true}))
		if err := ctx.Run(); err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	} else {
		runUI()
	}
}
