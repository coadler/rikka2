package rikka

import (
	"context"
	"os"

	"github.com/andersfylling/disgord"
	"go.coder.com/slog"
	"go.coder.com/slog/sloggers/sloghuman"
)

type Command interface {
	Register(func(event string, inputs ...interface{}))
}

func New(token string) *Rikka {
	return &Rikka{
		Log:    sloghuman.Make(os.Stdout),
		ctx:    context.Background(),
		token:  token,
		Prefix: "b.",
		Client: disgord.New(disgord.Config{
			BotToken:           token,
			LoadMembersQuietly: true,
			Logger:             disgord.DefaultLogger(true),
		}),
	}
}

type Rikka struct {
	Log slog.Logger
	ctx context.Context

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
	defer r.Client.StayConnectedUntilInterrupted()

	r.Client.On("READY", func(s disgord.Session, h *disgord.Ready) {
		r.Log.Info(h.Ctx, "ready")
	})
}
