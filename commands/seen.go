package commands

import (
	"context"
	"encoding/binary"
	"strconv"
	"time"

	"cdr.dev/slog"
	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"github.com/dustin/go-humanize"
	"golang.org/x/xerrors"

	rikka "github.com/coadler/rikka2"
)

func NewSeenCommand(r *rikka.Rikka, fdb fdb.Database) rikka.Command {
	dir, err := directory.CreateOrOpen(fdb, []string{"rikka", "seen"}, nil)
	if err != nil {
		r.Log.Fatal(context.Background(), "failed to create directory", slog.Error(err))
	}

	return &seenCmd{
		Rikka: r,
		dir:   dir,
	}
}

type seenCmd struct {
	*rikka.Rikka

	dir directory.DirectorySubspace
}

func (c *seenCmd) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", c.handleSeen, c.handleCommand)
}

func (c *seenCmd) Help() []rikka.CommandHelp {
	return []rikka.CommandHelp{
		{
			Name:        "seen",
			Aliases:     []string{"lastseen", "lastactive"},
			Section:     rikka.HelpSecionInfo,
			Description: "See the last time a user typed in the current channel and guild",
			Usage:       "<mention | user id>",
			Examples: []string{
				"`%sseen @Kitty#0001`           - Use a mention to see seen stats.",
				"`%sseen @Kitty#0001 @thy#0001` - Query multiple users at a time.",
				"`%sseen 105484726235607040`    - Use an id to see seen stats.",
			},
		},
	}
}

func (c *seenCmd) handleCommand(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.Rikka, "seen", mc.Message) {
		return
	}

	var (
		ctx    = mc.Ctx
		userID disgord.Snowflake
	)

	parts := rikka.ParseCommand(c.Rikka, mc.Message)
	if len(parts) == 0 {
		userID = mc.Message.Author.ID
	} else if len(parts) == 1 {
		parsed, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "failed to parse user id")
			return
		}

		userID = disgord.Snowflake(parsed)
	} else {
		s.SendMsg(ctx, mc.Message.ChannelID, "Please only supply one user")
		return
	}

	user, err := s.GetUser(ctx, userID)
	if err != nil {
		c.HandleError(ctx, s, mc.Message, err, "failed to find user")
		return
	}

	lastChannel, lastGuild, err := c.load(userID, mc.Message.ChannelID, mc.Message.GuildID)
	if err != nil {
		c.HandleError(ctx, s, mc.Message, err, "failed to load last seen times")
		return
	}

	lastChannelStr := humanize.Time(lastChannel)
	if lastChannel.IsZero() {
		lastChannelStr = "Never"
	}
	lastGuildStr := humanize.Time(lastGuild)
	if lastGuild.IsZero() {
		lastGuildStr = "Never"
	}

	uav, _ := user.AvatarURL(1024, true)
	s.SendMsg(ctx, mc.Message.ChannelID, disgord.CreateMessageParams{
		Embed: &disgord.Embed{
			Title: "Last seen",
			Author: &disgord.EmbedAuthor{
				Name:    user.Username,
				IconURL: uav,
			},
			Thumbnail: &disgord.EmbedThumbnail{
				URL: uav,
			},
			Fields: []*disgord.EmbedField{
				{Name: "Channel", Value: lastChannelStr, Inline: true},
				{Name: "Guild", Value: lastGuildStr, Inline: true},
			},
		},
	})
}

func (c *seenCmd) load(userID, channelID, guildID disgord.Snowflake) (channel, guild time.Time, err error) {
	var (
		cRaw []byte
		gRaw []byte
	)

	err = c.ReadTransact(func(t fdb.ReadTransaction) error {
		cRaw = t.Get(c.fmtLastSeenKey(userID, channelID)).MustGet()
		gRaw = t.Get(c.fmtLastSeenKey(userID, guildID)).MustGet()
		return nil
	})
	if err != nil {
		return time.Time{}, time.Time{}, xerrors.Errorf("failed to transact last seen times: %w", err)
	}

	if cRaw != nil {
		ns := binary.BigEndian.Uint64(cRaw)
		channel = time.Unix(0, int64(ns))
	}

	if gRaw != nil {
		ns := binary.BigEndian.Uint64(gRaw)
		guild = time.Unix(0, int64(ns))
	}

	return channel, guild, nil
}

func (c *seenCmd) handleSeen(s disgord.Session, mc *disgord.MessageCreate) {
	var (
		now    = time.Now()
		nowRaw [8]byte
	)
	binary.BigEndian.PutUint64(nowRaw[:], uint64(now.UnixNano()))

	c.Transact(func(t fdb.Transaction) error {
		t.Set(c.fmtLastSeenKey(mc.Message.Author.ID, mc.Message.ChannelID), nowRaw[:])
		t.Set(c.fmtLastSeenKey(mc.Message.Author.ID, mc.Message.GuildID), nowRaw[:])
		return nil
	})
}

func (c *seenCmd) fmtLastSeenKey(user, location disgord.Snowflake) fdb.Key {
	return c.dir.Pack(tuple.Tuple{uint64(location), uint64(user)})
}
