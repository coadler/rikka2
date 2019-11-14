package commands

import (
	"context"

	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"github.com/apple/foundationdb/bindings/go/src/fdb/directory"
	"github.com/apple/foundationdb/bindings/go/src/fdb/tuple"
	rikka "github.com/coadler/rikka2"
	jsoniter "github.com/json-iterator/go"
	"go.coder.com/slog"
)

func NewMessageTrackCmd(r *rikka.Rikka, fdb fdb.Database) rikka.Command {
	dir, err := directory.CreateOrOpen(fdb, []string{"rikka", "message_track"}, nil)
	if err != nil {
		r.Log.Fatal(context.Background(), "failed to create directory", slog.Error(err))
	}

	return &messageTrackCmd{
		Rikka: r,
		fdb:   fdb,
		dir:   dir,
	}
}

type messageTrackCmd struct {
	*rikka.Rikka

	fdb fdb.Database
	dir directory.DirectorySubspace
}

func (c *messageTrackCmd) Register(fn func(event string, inputs ...interface{})) {
	fn("MESSAGE_CREATE", c.storeMessage)
}

func (c *messageTrackCmd) storeMessage(s disgord.Session, h *disgord.MessageCreate) {
	if !c.guildIsEnabled(h.Message.GuildID) {
		return
	}

	raw, err := jsoniter.Marshal(h.Message)
	if err != nil {
		c.Log.Error(h.Ctx, "failed to marshal message", slog.Error(err))
		return
	}

	err = c.Transact(func(t fdb.Transaction) error {
		t.Set(c.fmtMessageKey(h.Message.GuildID), raw)
		return nil
	})
	if err != nil {
		c.Log.Error(h.Ctx, "failed to save message to fdb", slog.Error(err))
		return
	}
}

func (c *messageTrackCmd) guildIsEnabled(guildID disgord.Snowflake) bool {
	var enabled bool

	err := c.ReadTransact(func(t fdb.ReadTransaction) error {
		enabled = t.Snapshot().Get(c.fmtEnabledKey(guildID)).MustGet() != nil
		return nil
	})
	if err != nil {
		c.Log.Error(context.Background(), "failed to check if guild is enabled", slog.Error(err))
		return false
	}

	return enabled
}

func (c *messageTrackCmd) fmtEnabledKey(guildID disgord.Snowflake) fdb.Key {
	return c.dir.Sub(0).Pack(tuple.Tuple{uint64(guildID)})
}

func (c *messageTrackCmd) fmtMessageKey(guildID disgord.Snowflake) fdb.Key {
	return c.dir.Sub(1).Pack(tuple.Tuple{uint64(guildID)})
}
