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

func (c *pingCmd) handle(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.bot, "ping", mc.Message) {
		return
	}

	start := time.Now()

	msg, err := s.SendMsg(mc.Message.ChannelID, "Pong!")
	if err != nil {
		c.bot.Log.Error(mc.Ctx, "failed to send pong message")
	}

	took := time.Since(start)
	s.SetMsgContent(msg.ChannelID, msg.ID, fmt.Sprintf("Pong! - `%s`", took))
}
