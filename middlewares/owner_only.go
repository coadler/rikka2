package middlewares

import (
	"fmt"

	"github.com/andersfylling/disgord"
)

func OwnerOnly(i interface{}) interface{} {
	switch mc := i.(type) {
	case *disgord.MessageCreate:
		if !messageIsOwner(mc.Message) {
			return nil
		}

	case *disgord.MessageUpdate:
		if !messageIsOwner(mc.Message) {
			return nil
		}

	default:
		fmt.Printf("unknown: %T", i)
	}

	return i
}

func messageIsOwner(m *disgord.Message) bool {
	if m.Author != nil {
		return m.Author.ID == disgord.Snowflake(105484726235607040)
	}

	return false
}
