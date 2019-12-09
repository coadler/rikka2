package commands

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/andersfylling/disgord"
	"go.coder.com/slog"

	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/middlewares"
)

func NewExecCommand(r *rikka.Rikka) rikka.Command {
	return &execCmd{Rikka: r}
}

type execCmd struct {
	*rikka.Rikka
}

func (c *execCmd) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", middlewares.BotOwnerOnly, c.handle)
}

func (c *execCmd) Help() []rikka.CommandHelp {
	return []rikka.CommandHelp{
		{
			Name:        "exec",
			Aliases:     nil,
			Section:     rikka.HelpSectionOwner,
			Description: "Execute shell commands",
			Usage:       "<command>",
			Examples: []string{
				"`%sexec lscpu` - List CPU information.",
			},
		},
	}
}

func (c *execCmd) handle(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.Rikka, "exec", mc.Message) {
		return
	}

	ctx := mc.Ctx

	sp := strings.Split(mc.Message.Content, " ")
	if len(sp) < 2 {
		s.SendMsg(ctx, mc.Message.ChannelID, "Not enough args")
		return
	}

	var cmd *exec.Cmd

	if len(sp[1:]) == 1 {
		cmd = exec.Command(sp[1])
	} else {
		cmd = exec.Command(sp[1], sp[2:]...)
	}

	const maxOutput = 2000
	out, _ := cmd.CombinedOutput()
	if len(out) > maxOutput {
		out = out[:maxOutput]
	}

	_, err := s.SendMsg(ctx, mc.Message.ChannelID, &disgord.CreateMessageParams{
		Embed: &disgord.Embed{
			Description: "```\n" + string(out) + "\n```",
			Author: &disgord.EmbedAuthor{
				Name:    "zsh",
				IconURL: "https://res-5.cloudinary.com/crunchbase-production/image/upload/c_lpad,h_256,w_256,f_auto,q_auto:eco/zcertdhbiiswm5hebz1u",
			},
			Fields: []*disgord.EmbedField{
				{Name: "Exit status", Value: fmt.Sprintf("%d", cmd.ProcessState.ExitCode()), Inline: true},
				{Name: "Took", Value: fmt.Sprintf("%s", cmd.ProcessState.UserTime()), Inline: true},
				// {Name: "Output", Value: "```\n" + string(out) + "\n```"},
			},
		},
	})
	if err != nil {
		c.Log.Error(mc.Ctx, "failed to send exec output", slog.Error(err))
	}
}
