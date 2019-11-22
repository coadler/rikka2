package middlewares

import (
	"fmt"

	"github.com/andersfylling/disgord"
)

func NoBots(i interface{}) interface{} {
	switch mc := i.(type) {
	case *disgord.MessageCreate:
		if messageIsBot(mc.Message) {
			return nil
		}

	case *disgord.MessageUpdate:
		if messageIsBot(mc.Message) {
			return nil
		}

	case *disgord.MessageDelete:
	default:
		fmt.Printf("unknown: %T\n", i)
	}

	return i
}

func messageIsBot(msg *disgord.Message) bool {
	if msg.Author != nil {
		return msg.Author.Bot
	}

	// TODO: messages with a nil author are embeds sent by bots.
	return true
}
