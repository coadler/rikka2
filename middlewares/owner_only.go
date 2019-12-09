package middlewares

import (
	"context"
	"fmt"

	"github.com/andersfylling/disgord"

	rikka "github.com/coadler/rikka2"
)

func BotOwnerOnly(i interface{}) interface{} {
	switch mc := i.(type) {
	case *disgord.MessageCreate:
		if !messageIsBotOwner(mc.Message) {
			return nil
		}

	case *disgord.MessageUpdate:
		if !messageIsBotOwner(mc.Message) {
			return nil
		}

	default:
		fmt.Printf("unknown: %T", i)
	}

	return i
}

func messageIsBotOwner(m *disgord.Message) bool {
	if m.Author != nil {
		return m.Author.ID == disgord.Snowflake(105484726235607040)
	}

	return false
}

func ServerOwnerOnly(r *rikka.Rikka) func(i interface{}) interface{} {
	return func(i interface{}) interface{} {
		switch mc := i.(type) {
		case *disgord.MessageCreate:
			if !messageIsServerOwner(r, mc.Message) {
				return nil
			}

		case *disgord.MessageUpdate:
			if !messageIsServerOwner(r, mc.Message) {
				return nil
			}

		default:
			fmt.Printf("unknown: %T", i)
		}

		return i
	}
}

func messageIsServerOwner(r *rikka.Rikka, m *disgord.Message) bool {
	g, err := r.Client.GetGuild(context.Background(), m.GuildID)
	if err != nil {
		return false
	}

	return g.OwnerID == m.Author.ID || messageIsBotOwner(m)
}
