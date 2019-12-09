package logs

import (
	"sync"

	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"

	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/middlewares"
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

func (c *logCmd) Help() []rikka.CommandHelp {
	return []rikka.CommandHelp{
		{
			Name:        "log",
			Aliases:     nil,
			Section:     rikka.HelpSectionModeration,
			Description: "Execute shell commands",
			Usage:       "<command>",
			Examples: []string{
				"`%sexec lscpu` - List CPU information.",
			},
		},
	}
}

type logSection interface {
	rikka.Command

	handleCommand(s disgord.Session, h *disgord.MessageCreate, args rikka.Args)
}

type logCmd struct {
	*rikka.Rikka

	fdb fdb.Database

	cmdMu    sync.Mutex
	commands map[string]logSection
}

func (c *logCmd) Register(fn func(event string, inputs ...interface{})) {
	for _, e := range c.commands {
		e.Register(fn)
	}

	// Since log sections have no knowlege of each other, they must have
	// commands routed to them from the base logCmd.
	// TODO: user permissions and not just server owner
	fn("MESSAGE_CREATE", middlewares.ServerOwnerOnly(c.Rikka), c.handleCommand)
}

// handleCommand routes log commands to their respective section or displays
// the help message if none are found.
func (c *logCmd) handleCommand(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.Rikka, "log", mc.Message) {
		return
	}

	ctx := mc.Ctx

	args := rikka.ParseCommand(c.Rikka, mc.Message)
	if len(args) < 1 {
		// eventually send good help info here
		s.SendMsg(ctx, mc.Message.ChannelID, "Please provide a section")
		return
	}

	// if handleCommand is called concurrently this map is safe
	c.cmdMu.Lock()
	sect, ok := c.commands[args.Pop()]
	c.cmdMu.Unlock()
	if !ok {
		// incorrect section
		s.SendMsg(ctx, mc.Message.ChannelID, "Section not found")
		return
	}

	// pass through event to the correct section
	sect.handleCommand(s, mc, args)
}
