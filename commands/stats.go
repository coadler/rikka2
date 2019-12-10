package commands

import (
	"fmt"
	"runtime"
	"time"

	"github.com/andersfylling/disgord"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"

	rikka "github.com/coadler/rikka2"
)

func NewStatsCmd(r *rikka.Rikka) rikka.Command {
	return &statsCmd{Rikka: r, start: time.Now()}
}

type statsCmd struct {
	*rikka.Rikka
	start time.Time
}

func (c *statsCmd) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", c.handle)
}

func (c *statsCmd) Help() []rikka.CommandHelp {
	return []rikka.CommandHelp{
		{
			Name:        "stats",
			Aliases:     nil,
			Section:     rikka.HelpSecionInfo,
			Description: "See bot stats",
			Usage:       "",
			Examples: []string{
				"`%sstats` - See bot stats",
			},
		},
	}
}

func (c *statsCmd) handle(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.Rikka, "stats", mc.Message) {
		return
	}

	ctx := mc.Ctx

	memstats := runtime.MemStats{}
	runtime.ReadMemStats(&memstats)

	self, err := c.Client.Myself(ctx)
	if err != nil {
		s.SendMsg(ctx, mc.Message.ChannelID, "Failed to generate stats: "+err.Error())
		return
	}

	guilds := s.GetConnectedGuilds()

	spew.Config.DisableMethods = true

	var guildCount, channelCount, userCount int

	guildCount = len(guilds)
	for _, e := range guilds {
		g, err := s.GetGuild(ctx, e)
		if err != nil {
			s.SendMsg(ctx, mc.Message.ChannelID, "Failed to generate stats: "+err.Error())
			return
		}

		// spew.Dump(g)
		channelCount += len(g.Channels)
		userCount += len(g.Members)
	}

	s.SendMsg(ctx, mc.Message.ChannelID, disgord.CreateMessageParams{Embed: &disgord.Embed{
		Title: "Rikka v2",
		Timestamp: disgord.Time{
			Time: time.Now(),
		},
		Color: 0x79c879,
		Thumbnail: &disgord.EmbedThumbnail{
			URL: discordgo.EndpointUserAvatar(self.ID.String(), self.Avatar),
		},
		Fields: []*disgord.EmbedField{
			&disgord.EmbedField{
				Name:   "Golang",
				Value:  runtime.Version(),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Uptime",
				Value:  time.Since(c.start).String(),
				Inline: true,
			},
			&disgord.EmbedField{
				Name: "Memory used",
				Value: fmt.Sprintf("%s / %s",
					humanize.Bytes(memstats.Alloc),
					humanize.Bytes(memstats.Sys),
				),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Garbage collected",
				Value:  humanize.Bytes(memstats.TotalAlloc),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Goroutines",
				Value:  fmt.Sprintf("%d", runtime.NumGoroutine()),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Guilds",
				Value:  fmt.Sprintf("%d", guildCount),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Channels",
				Value:  fmt.Sprintf("%d", channelCount),
				Inline: true,
			},
			&disgord.EmbedField{
				Name:   "Users",
				Value:  fmt.Sprintf("%d", userCount),
				Inline: true,
			},
		},
	}})

}
