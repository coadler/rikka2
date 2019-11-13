package main

import (
	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/commands"
)

func main() {
	r := rikka.New(Token)

	r.RegisterCommands(
		commands.NewPingCommand(r),
		commands.NewStatsCmd(r),
	)
	r.Open()
}
