package rikka

import (
	"context"
	"os"

	"github.com/andersfylling/disgord"
	"github.com/apple/foundationdb/bindings/go/src/fdb"
	"go.coder.com/slog"
	"go.coder.com/slog/sloggers/sloghuman"
)

type Command interface {
	Register(func(event string, inputs ...interface{}))
}

func New(fdb fdb.Database, token string) *Rikka {
	return &Rikka{
		Log:    sloghuman.Make(os.Stdout),
		ctx:    context.Background(),
		fdb:    fdb,
		token:  token,
		Prefix: "b.",
		Client: disgord.New(disgord.Config{
			BotToken:           token,
			LoadMembersQuietly: true,
			Logger:             disgord.DefaultLogger(false),
		}),
	}
}

type Rikka struct {
	Log slog.Logger
	ctx context.Context
	fdb fdb.Database

	token  string
	Prefix string

	Client *disgord.Client
}

func (r *Rikka) RegisterCommands(cmds ...Command) {
	r.Log.Info(r.ctx, "registering commands", slog.F("count", len(cmds)))

	for _, e := range cmds {
		e.Register(r.Client.On)
	}
}

func (r *Rikka) Open() {
	defer r.Client.StayConnectedUntilInterrupted(context.Background())

	r.Client.On("READY", func(s disgord.Session, h *disgord.Ready) {
		r.Log.Info(h.Ctx, "ready")
	})
}

func (r *Rikka) Transact(fn func(t fdb.Transaction) error) error {
	_, err := r.fdb.Transact(func(t fdb.Transaction) (interface{}, error) {
		return nil, fn(t)
	})

	return err
}

func (r *Rikka) ReadTransact(fn func(t fdb.ReadTransaction) error) error {
	_, err := r.fdb.ReadTransact(func(t fdb.ReadTransaction) (interface{}, error) {
		return nil, fn(t)
	})

	return err
}
