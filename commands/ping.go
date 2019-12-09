package commands

import (
	"fmt"
	"time"

	"github.com/andersfylling/disgord"
	rikka "github.com/coadler/rikka2"
)

func NewPingCommand(r *rikka.Rikka) rikka.Command {
	return &pingCmd{bot: r}
}

type pingCmd struct {
	bot *rikka.Rikka
}

func (c *pingCmd) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", c.handle)
}

func (c *pingCmd) Help() []rikka.CommandHelp {
	return []rikka.CommandHelp{
		{
			Name:        "ping",
			Aliases:     nil,
			Section:     rikka.HelpSecionGeneral,
			Description: "View bot latency",
			Usage:       "",
			Examples: []string{
				"`%sping` - View bot latency.",
			},
		},
	}
}

func (c *pingCmd) handle(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.bot, "ping", mc.Message) {
		return
	}

	ctx := mc.Ctx
	start := time.Now()

	msg, err := s.SendMsg(ctx, mc.Message.ChannelID, "Pong!")
	if err != nil {
		c.bot.Log.Error(mc.Ctx, "failed to send pong message")
		return
	}

	took := time.Since(start)
	s.SetMsgContent(ctx, msg.ChannelID, msg.ID, fmt.Sprintf("Pong! - `%s`", took))
}
