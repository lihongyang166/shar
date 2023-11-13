package main

import (
	_ "embed"
	"gitlab.com/shar-workflow/nats-proto-gen-go/commands"
)

func main() {
	commands.Execute()
}
