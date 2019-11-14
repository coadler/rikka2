package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/commands"
)

func main() {
	r := rikka.New(Token)

	fdb.MustAPIVersion(620)
	fdb := fdb.MustOpenDefault()
	r.RegisterCommands(
		commands.NewPingCommand(r),
		commands.NewStatsCmd(r),
		commands.NewMessageTrackCmd(r, fdb),
	)
	r.Open()
}
