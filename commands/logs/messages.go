package logs

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	jsoniter "github.com/json-iterator/go"
	"go.coder.com/slog"
	"golang.org/x/xerrors"

	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/middlewares"
)

func newMessageLog(r *rikka.Rikka, fdb fdb.Database) logSection {
	dir, err := directory.CreateOrOpen(fdb, []string{"rikka", "logs", "message_track"}, nil)
	if err != nil {
		r.Log.Fatal(context.Background(), "failed to create directory", slog.Error(err))
	}

	return &messageLog{
		Rikka: r,
		fdb:   fdb,
		dir:   dir,
	}
}

type messageLog struct {
	*rikka.Rikka

	fdb fdb.Database
	dir directory.DirectorySubspace
}

func (c *messageLog) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", middlewares.NoBots, c.storeMessage)
	fn("MESSAGE_UPDATE", middlewares.NoBots, c.logUpdate)
	fn("MESSAGE_DELETE", middlewares.NoBots, c.logDelete)

	c.enableUpdateLog(319567980491046913, 644376487331495967)
	c.enableUpdateLog(309741345264631818, 645532355762585602)
}

func (c *messageLog) handle(s disgord.Session, mc *disgord.MessageCreate) {
	if !rikka.MatchesCommand(c.Rikka, "log", mc.Message) {
		return
	}

	_, args := rikka.ParseCommand(c.Rikka, mc.Message)
	if len(args) < 1 && args[0] != "messages" {
		return
	}

	switch args.Pop() {
	case "delete":
	case "update":
	default:
	}
}

func (c *messageLog) storeMessage(s disgord.Session, mc *disgord.MessageCreate) {
	if !c.guildIsEnabled(mc.Message.GuildID) {
		return
	}

	mc.Message.EditedTimestamp = disgord.Time{Time: time.Now()}
	raw, err := jsoniter.Marshal(mc.Message)
	if err != nil {
		c.Log.Error(mc.Ctx, "failed to marshal message", slog.Error(err))
		return
	}

	err = c.Transact(func(t fdb.Transaction) error {
		t.Set(c.fmtMessageKey(mc.Message.ID), raw)
		return nil
	})
	if err != nil {
		c.Log.Error(mc.Ctx, "failed to save message to fdb", slog.Error(err))
		return
	}
}

func (c *messageLog) logDelete(s disgord.Session, md *disgord.MessageDelete) {
	enabled, channel := c.deleteLogIsEnabled(md.GuildID)
	if !enabled {
		c.Log.Info(md.Ctx, "message delete not enabled")
		return
	}

	s.SendMsg(channel, disgord.CreateMessageParams{
		Embed: &disgord.Embed{},
	})
}

func (c *messageLog) logUpdate(s disgord.Session, mu *disgord.MessageUpdate) {
	c.Log.Info(mu.Ctx, "logging message", slog.F("id", mu.Message.ID))

	enabled, logChannel := c.updateLogIsEnabled(mu.Message.GuildID)
	if !enabled {
		c.Log.Info(mu.Ctx, "message log not enabled")
		return
	}

	oldMsg, err := c.messageFromCache(mu.Message.ID)
	if err != nil {
		c.Log.Error(mu.Ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	guild, err := s.GetGuild(mu.Message.GuildID)
	if err != nil {
		c.Log.Error(mu.Ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	channel, err := s.GetChannel(mu.Message.ChannelID)
	if err != nil {
		c.Log.Error(mu.Ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	spew.Config.DisableMethods = true
	// spew.Dump(mu.Message)
	// guild.Members = nil
	// guild.Channels = nil
	// guild.VoiceStates = nil
	// guild.Presences = nil
	// guild.Emojis = nil
	// guild.Roles = nil
	// spew.Dump(guild)
	// spew.Dump(discordgo.EndpointGuildIcon(guild.ID.String(), guild.Icon))

	_, err = s.SendMsg(logChannel, disgord.CreateMessageParams{
		Embed: &disgord.Embed{
			Title: "Message edited",
			Fields: []*disgord.EmbedField{
				{
					Name: "User",
					Value: fmt.Sprintf("%s %s %s",
						mu.Message.Author.Mention(),
						mu.Message.Author.Tag(),
						mu.Message.Author.ID.String(),
					),
					Inline: true,
				},
				{
					Name:   "Message ID",
					Value:  mu.Message.ID.String(),
					Inline: true,
				},
				{
					Name: "Channel",
					Value: fmt.Sprintf("%s %s",
						channel.Mention(),
						channel.ID.String(),
					),
					Inline: true,
				},
				{
					Name:  "Old content",
					Value: oldMsg.Content,
				},
				{
					Name:  "New content",
					Value: mu.Message.Content,
				},
			},
			Footer: &disgord.EmbedFooter{
				Text:    guild.Name,
				IconURL: discordgo.EndpointGuildIcon(guild.ID.String(), guild.Icon),
			},
			Timestamp: mu.Message.Timestamp,
		},
	})
	if err != nil {
		c.Log.Error(mu.Ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}
}

func (c *messageLog) guildIsEnabled(guildID disgord.Snowflake) bool {
	var enabled bool

	err := c.ReadTransact(func(t fdb.ReadTransaction) error {
		ss := t.Snapshot()
		enabled = false ||
			ss.Get(c.fmtDeleteEnabledKey(guildID)).MustGet() != nil ||
			ss.Get(c.fmtUpdateEnabledKey(guildID)).MustGet() != nil
		return nil
	})
	if err != nil {
		c.Log.Error(context.Background(), "failed to check if guild is enabled", slog.Error(err))
		return false
	}

	return enabled
}

func (c *messageLog) enableUpdateLog(guildID, channel disgord.Snowflake) error {
	err := c.Transact(func(t fdb.Transaction) error {
		idRaw := [8]byte{}
		binary.BigEndian.PutUint64(idRaw[:], uint64(channel))

		t.Set(c.fmtUpdateEnabledKey(guildID), idRaw[:])
		return nil
	})
	if err != nil {
		return xerrors.Errorf("failed to transact enabling update log: %w", err)
	}

	return nil
}

func (c *messageLog) updateLogIsEnabled(guildID disgord.Snowflake) (enabled bool, channel disgord.Snowflake) {
	err := c.ReadTransact(func(t fdb.ReadTransaction) error {
		raw := t.Snapshot().Get(c.fmtUpdateEnabledKey(guildID)).MustGet()
		enabled = raw != nil

		if enabled {
			channel = disgord.Snowflake(binary.BigEndian.Uint64(raw))
		}
		return nil
	})
	if err != nil {
		c.Log.Error(context.Background(), "failed to check if guild update log is enabled", slog.Error(err))
		return false, 0
	}

	return
}

func (c *messageLog) deleteLogIsEnabled(guildID disgord.Snowflake) (enabled bool, channel disgord.Snowflake) {
	err := c.ReadTransact(func(t fdb.ReadTransaction) error {
		raw := t.Snapshot().Get(c.fmtDeleteEnabledKey(guildID)).MustGet()
		enabled = raw != nil

		if enabled {
			channel = disgord.Snowflake(binary.BigEndian.Uint64(raw))
		}
		return nil
	})
	if err != nil {
		c.Log.Error(context.Background(), "failed to check if guild delete log is enabled", slog.Error(err))
		return false, 0
	}

	return
}

var errMsgNotFound = errors.New("message not found in cache")

func (c *messageLog) messageFromCache(id disgord.Snowflake) (*disgord.Message, error) {
	var (
		rawMsg []byte
		msg    disgord.Message
	)

	err := c.ReadTransact(func(t fdb.ReadTransaction) error {
		rawMsg = t.Snapshot().Get(c.fmtMessageKey(id)).MustGet()
		return nil
	})
	if err != nil {
		return nil, xerrors.Errorf("failed to transact message from cache: %w", err)
	}

	if rawMsg == nil {
		return nil, xerrors.Errorf("%w", errMsgNotFound)
	}

	err = jsoniter.Unmarshal(rawMsg, &msg)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal message from cache: %w", err)
	}

	return &msg, nil
}

func (c *messageLog) fmtDeleteEnabledKey(guildID disgord.Snowflake) fdb.Key {
	return c.dir.Sub(0).Pack(tuple.Tuple{uint64(guildID)})
}

func (c *messageLog) fmtUpdateEnabledKey(guildID disgord.Snowflake) fdb.Key {
	return c.dir.Sub(1).Pack(tuple.Tuple{uint64(guildID)})
}

func (c *messageLog) fmtMessageKey(msgID disgord.Snowflake) fdb.Key {
	return c.dir.Sub(2).Pack(tuple.Tuple{uint64(msgID)})
}
