package logs

import (
	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"

	rikka "github.com/coadler/rikka2"
)

func NewLogCmd(r *rikka.Rikka, fdb fdb.Database) rikka.Command {
	return &logCmd{
		Rikka: r,
		fdb:   fdb,
		commands: map[string]logSection{
			"messages": newMessageLog(r, fdb),
		},
	}
}

type logSection interface {
	rikka.Command

	handleCommand(s disgord.Session, h *disgord.MessageCreate)
}

type logCmd struct {
	*rikka.Rikka

	fdb      fdb.Database
	commands map[string]logSection
}

func (c *logCmd) Register(fn func(event string, inputs ...interface{})) {
	for _, e := range c.commands {
		e.Register(fn)
	}
}
