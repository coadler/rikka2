package logs

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cdr.dev/slog"
	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	"github.com/bwmarrin/discordgo"
	jsoniter "github.com/json-iterator/go"
	"github.com/minio/minio-go"
	"golang.org/x/xerrors"

	rikka "github.com/coadler/rikka2"
	"github.com/coadler/rikka2/middlewares"
)

func newMessageLog(r *rikka.Rikka, fdb fdb.Database) logSection {
	dir, err := directory.CreateOrOpen(fdb, []string{"rikka", "logs", "message_track"}, nil)
	if err != nil {
		r.Log.Fatal(context.Background(), "failed to create directory", slog.Error(err))
	}

	mc, err := minio.New("127.0.0.1:9000", "H7E789JVJLYIDN9B0KB2", "N5RNoaakuJ1UG7TFgPCgmcnrNKgza38Eg0d8BYsB", false)
	if err != nil {
		r.Log.Fatal(context.Background(), "failed to connect to minio", slog.Error(err))
	}

	const bucket = "message-attachments"
	bucketExists, err := mc.BucketExists(bucket)
	if err != nil {
		r.Log.Fatal(context.Background(), "check if bucket exists", slog.Error(err))
	}

	if !bucketExists {
		err := mc.MakeBucket(bucket, "")
		if err != nil {
			r.Log.Fatal(context.Background(), "failed to create message attachments bucket", slog.Error(err))
		}
	}

	return &messageLog{
		Rikka:            r,
		fdb:              fdb,
		dir:              dir,
		minio:            mc,
		attachmentBucket: bucket,
	}
}

type messageLog struct {
	*rikka.Rikka

	fdb fdb.Database
	dir directory.DirectorySubspace

	minio            *minio.Client
	attachmentBucket string
}

func (c *messageLog) Help() []rikka.CommandHelp {
	const detailed = ``

	return []rikka.CommandHelp{
		{
			Name:        "messages",
			Aliases:     nil,
			Section:     rikka.HelpSectionModeration,
			Description: "Log updates or deletes",
			Usage:       "<update | delete> <enable | disable> [channel id]",
			Detailed:    detailed,
			Examples: []string{
				"`%slog messages enable`                    - Enable message logging to the current channel.",
				"`%slog messages enable 644376487331495967` - Enable message logging to the provided channel id.",
				"`%slog messages enable #my-log-channel`    - Enable message logging to the provided channel mention.",
			},
		},
	}
}

func (c *messageLog) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", middlewares.NoBots, c.storeMessage)
	fn("MESSAGE_UPDATE", middlewares.NoBots, c.logUpdate)
	fn("MESSAGE_DELETE", middlewares.NoBots, c.logDelete)
}

func (c *messageLog) handleCommand(s disgord.Session, mc *disgord.MessageCreate, args rikka.Args) {
	if len(args) < 1 {
		// send help
		return
	}

	typ := args.Pop()
	switch typ {
	case "delete":
		c.handleDeleteCommand(s, mc, args)
	case "update":
		c.handleUpdateCommand(s, mc, args)
	default:
		fmt.Println(typ)
	}
}

// <enable | disable> [channel]

func (c *messageLog) handleDeleteCommand(s disgord.Session, mc *disgord.MessageCreate, args rikka.Args) {
	var (
		ctx       = mc.Ctx
		action    = args.Pop()
		channelID = args.Pop()
	)

	switch action {
	case "enable":
		cid, err := rikka.ExtractID(rikka.ChannelMentionRegex, channelID)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to extract channel id")
			return
		}

		// make sure the channel exists
		ch, err := s.GetChannel(ctx, cid)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to retrieve log channel")
			return
		}

		err = c.enableDeleteLog(ch.GuildID, ch.ID)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to enable delete logging")
			return
		}

		s.SendMsg(ctx, mc.Message.ChannelID, "Enabling delete logs in "+ch.Mention())
		return

	case "disable":
		// err := c.disableDeleteLog(ch.GuildID, ch.ID)
		// if err != nil {
		// 	c.HandleError(ctx, s, mc.Message, err, "Failed to enable delete logging")
		// 	return
		// }

		s.SendMsg(ctx, mc.Message.ChannelID, "Disabling delete logs")
		return

	default:
		s.SendMsg(ctx, mc.Message.ChannelID, "Unknown action. Available actions are: [enable, disable]")
		return
	}
}

// <enable | disable> [channel]

func (c *messageLog) handleUpdateCommand(s disgord.Session, mc *disgord.MessageCreate, args rikka.Args) {
	var (
		ctx       = mc.Ctx
		action    = args.Pop()
		channelID = args.Pop()
	)

	switch action {
	case "enable":
		cid, err := rikka.ExtractID(rikka.ChannelMentionRegex, channelID)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to extract channel id")
			return
		}

		// make sure the channel exists
		ch, err := s.GetChannel(ctx, cid)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to retrieve log channel")
			return
		}

		err = c.enableUpdateLog(ch.GuildID, ch.ID)
		if err != nil {
			c.HandleError(ctx, s, mc.Message, err, "Failed to enable update logging")
			return
		}

		s.SendMsg(ctx, mc.Message.ChannelID, "Enabled update logs in "+ch.Mention())
		return

	case "disable":
		enabled, ch := c.updateLogIsEnabled(mc.Message.GuildID)
		if !enabled {
			s.SendMsg(ctx, mc.Message.ChannelID, "Update logs are not enabled")
			return
		}
		_ = ch

		// err := c.disableUpdateLog(mc.Message.GuildID, ch)
		// if err != nil {
		// 	c.HandleError(ctx, s, mc.Message, err, "Failed to enable delete logging")
		// 	return
		// }

		s.SendMsg(ctx, mc.Message.ChannelID, "Disabled update logging")
		return

	default:
		s.SendMsg(ctx, mc.Message.ChannelID, "Unknown action. Available actions are: [enable, disable]")
		return
	}
}

func (c *messageLog) storeMessage(s disgord.Session, mc *disgord.MessageCreate) {
	if !c.guildIsEnabled(mc.Message.GuildID) {
		return
	}

	ctx := mc.Ctx

	for _, e := range mc.Message.Attachments {
		resp, err := http.Get(e.ProxyURL)
		if err != nil {
			c.Log.Error(ctx, "failed to get message attachment", slog.Error(err))
			continue
		}
		defer resp.Body.Close()

		_, err = c.minio.PutObject(
			c.attachmentBucket,
			fmt.Sprintf("%d/%d", mc.Message.ID, e.ID),
			resp.Body,
			resp.ContentLength,
			minio.PutObjectOptions{},
		)
		if err != nil {
			c.Log.Error(ctx, "failed to upload message attachment", slog.Error(err))
			continue
		}
	}

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
	enabled, logChannel := c.deleteLogIsEnabled(md.GuildID)
	if !enabled {
		return
	}

	ctx := md.Ctx

	oldMsg, err := c.messageFromCache(md.MessageID)
	if err != nil {
		c.Log.Error(ctx, "failed to log message update", slog.Error(err))
		return
	}

	attachments := []disgord.CreateMessageFileParams{}
	for _, e := range oldMsg.Attachments {
		obj, err := c.minio.GetObject(c.attachmentBucket, fmt.Sprintf("%d/%d", md.MessageID, e.ID), minio.GetObjectOptions{})
		if err != nil {
			c.Log.Error(ctx, "failed to retrieve attachment from cache", slog.Error(err))
			continue
		}
		defer obj.Close()

		attachments = append(attachments, disgord.CreateMessageFileParams{
			FileName: e.Filename,
			Reader:   obj,
		})
	}

	author, err := s.GetUser(ctx, oldMsg.Author.ID)
	if err != nil {
		c.Log.Error(ctx, "failed to load author from cache", slog.Error(err))
		return
	}

	guild, err := s.GetGuild(ctx, md.GuildID)
	if err != nil {
		c.Log.Error(ctx, "failed to load guild from cache", slog.Error(err))
		return
	}

	channel, err := s.GetChannel(ctx, md.ChannelID)
	if err != nil {
		c.Log.Error(ctx, "failed to load channel from cache", slog.Error(err))
		return
	}

	uav, _ := author.AvatarURL(1024, true)
	_, err = s.SendMsg(ctx, logChannel, disgord.CreateMessageParams{
		Embed: &disgord.Embed{
			Title: "Message Deleted",
			Thumbnail: &disgord.EmbedThumbnail{
				URL: uav,
			},
			Fields: deleteEmbedFields(oldMsg, author, channel, guild),
			Footer: &disgord.EmbedFooter{
				Text:    guild.Name,
				IconURL: discordgo.EndpointGuildIcon(guild.ID.String(), guild.Icon),
			},
			Timestamp: disgord.Time{Time: time.Now()},
		},
		Files: attachments,
	})
	if err != nil {
		c.Log.Error(ctx, "failed to send delete log", slog.Error(err))
		return
	}
}

func deleteEmbedFields(
	oldMsg *disgord.Message,
	user *disgord.User,
	channel *disgord.Channel,
	guild *disgord.Guild,
) []*disgord.EmbedField {
	fields := []*disgord.EmbedField{
		{
			Name: "User",
			Value: fmt.Sprintf("%s %s %s",
				oldMsg.Author.Mention(),
				oldMsg.Author.Tag(),
				oldMsg.Author.ID.String(),
			),
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
			Name:  "Message ID",
			Value: oldMsg.ID.String(),
		},
	}

	if oldMsg.Content != "" {
		fields = append(fields, &disgord.EmbedField{
			Name:  "Deleted content",
			Value: oldMsg.Content,
		})
	}

	return fields
}

func (c *messageLog) logUpdate(s disgord.Session, mu *disgord.MessageUpdate) {
	ctx := mu.Ctx

	enabled, logChannel := c.updateLogIsEnabled(mu.Message.GuildID)
	if !enabled {
		return
	}

	oldMsg, err := c.messageFromCache(mu.Message.ID)
	if err != nil {
		c.Log.Error(ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	guild, err := s.GetGuild(ctx, mu.Message.GuildID)
	if err != nil {
		c.Log.Error(ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	channel, err := s.GetChannel(ctx, mu.Message.ChannelID)
	if err != nil {
		c.Log.Error(ctx, "failed to log message update", slog.Error(err), slog.F("message_id", mu.Message.ID))
		return
	}

	uav, _ := mu.Message.Author.AvatarURL(1024, true)
	_, err = s.SendMsg(ctx, logChannel, disgord.CreateMessageParams{
		Embed: &disgord.Embed{
			Title: "Message edited",
			Thumbnail: &disgord.EmbedThumbnail{
				URL: uav,
			},
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
					Name: "Channel",
					Value: fmt.Sprintf("%s %s",
						channel.Mention(),
						channel.ID.String(),
					),
					Inline: true,
				},
				{
					Name:  "Message ID",
					Value: mu.Message.ID.String(),
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

func (c *messageLog) enableDeleteLog(guildID, channel disgord.Snowflake) error {
	err := c.Transact(func(t fdb.Transaction) error {
		idRaw := [8]byte{}
		binary.BigEndian.PutUint64(idRaw[:], uint64(channel))

		t.Set(c.fmtDeleteEnabledKey(guildID), idRaw[:])
		return nil
	})
	if err != nil {
		return xerrors.Errorf("failed to transact enabling delete log: %w", err)
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
