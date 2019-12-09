package rikka

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andersfylling/disgord"
)

type HelpSection int

const (
	HelpSecionGeneral HelpSection = iota
	HelpSecionInfo
	HelpSectionModeration
	HelpSectionOwner
)

func (s HelpSection) String() string {
	switch s {
	case HelpSecionGeneral:
		return "General"
	case HelpSecionInfo:
		return "Info"
	case HelpSectionModeration:
		return "Moderation"
	case HelpSectionOwner:
		return "Owner"
	default:
		panic("unknown help section: " + strconv.FormatInt(int64(s), 10))
	}
}

var HelpSections = []HelpSection{
	HelpSecionGeneral,
	HelpSecionInfo,
	HelpSectionModeration,
	HelpSectionOwner,
}

type CommandHelp struct {
	Name        string
	Aliases     []string
	Section     HelpSection
	Description string
	Usage       string
	Detailed    string
	Examples    []string
}

func (r *Rikka) registerHelp(s disgord.Session, mc *disgord.MessageCreate) {
	if !MatchesCommand(r, "help", mc.Message) {
		return
	}

	var (
		ctx  = mc.Ctx
		args = ParseCommand(r, mc.Message)
	)

	if len(args) > 0 {
		s.SendMsg(ctx, mc.Message.ChannelID, "Detailed help coming soon :)")
		return
	}

	helps := make([]CommandHelp, 0, len(r.cmds))
	for _, e := range r.cmds {
		helps = append(helps, e.Help()...)
	}

	s.SendMsg(ctx, mc.Message.ChannelID, disgord.CreateMessageParams{Embed: r.embedFromCommandHelp(helps)})
}

func (r *Rikka) embedFromCommandHelp(helps []CommandHelp) *disgord.Embed {
	av, _ := r.self.AvatarURL(1024, true)
	fbuilder := strings.Builder{}

	fields := make([]*disgord.EmbedField, len(HelpSections))
	for i, sect := range HelpSections {
		for _, e := range helps {
			if e.Section == sect {
				if fbuilder.Len() > 0 {
					fbuilder.WriteString(", ")
				}
				fbuilder.WriteString("`" + e.Name + "`")
			}
		}

		fields[i] = &disgord.EmbedField{
			Name:   sect.String(),
			Value:  fbuilder.String(),
			Inline: true,
		}

		fbuilder.Reset()
	}

	return &disgord.Embed{
		Author: &disgord.EmbedAuthor{
			Name:    "Rikka v2 Command Help",
			IconURL: av,
			URL:     "https://github.com/coadler/rikka2",
		},
		Thumbnail: &disgord.EmbedThumbnail{
			URL: av,
		},
		Title:       "Join our server for more information",
		URL:         "https://asdf.com",
		Description: fmt.Sprintf("Type `%shelp [command]` for detailed usage information", r.Prefix),
		Fields:      fields,
	}
}
