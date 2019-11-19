package main

import (
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/commands"
	"github.com/coadler/rikka2/commands/logs"
)

func main() {
	fdb.MustAPIVersion(620)
	fdb := fdb.MustOpenDefault()

	r := rikka.New(fdb, Token)

	r.RegisterCommands(
		commands.NewPingCommand(r),
		commands.NewStatsCmd(r),
		logs.NewLogCmd(r, fdb),
		commands.NewExecCommand(r),
	)
	r.Open()
}
