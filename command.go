package rikka

import (
	"context"
	"fmt"
	"strings"

	"cdr.dev/slog"
	"github.com/andersfylling/disgord"
)

type Command interface {
	Register(func(event string, inputs ...interface{}))
	Help() []CommandHelp
}

type Args []string

func (a *Args) Pop() string {
	if len(*a) == 0 {
		return ""
	}

	arg := (*a)[0]
	*a = (*a)[1:]
	return arg
}

// MatchesCommandString returns true if a message matches a command.
// Commands will be matched ignoring case with a prefix if they are not private messages.
func MatchesCommandString(bot *Rikka, commandString string, private bool, message string) bool {
	lowerMessage := strings.ToLower(strings.TrimSpace(message))
	lowerPrefix := strings.ToLower(bot.Prefix)

	if strings.HasPrefix(lowerMessage, lowerPrefix) {
		lowerMessage = lowerMessage[len(lowerPrefix):]
	} else if !private {
		return false
	}

	lowerMessage = strings.TrimSpace(lowerMessage)
	lowerCommand := strings.ToLower(commandString)

	return lowerMessage == lowerCommand || strings.HasPrefix(lowerMessage, lowerCommand+" ")
}

// MatchesCommand returns true if a message matches a command.
func MatchesCommand(bot *Rikka, commandString string, message *disgord.Message) bool {
	return MatchesCommandString(bot, commandString, false, message.Content)
}

// ParseCommandString will strip all prefixes from a message string, and return that string, and a space separated tokenized version of that string.
func ParseCommandString(bot *Rikka, message string) Args {
	message = strings.TrimSpace(message)

	lowerMessage := strings.ToLower(message)
	lowerPrefix := strings.ToLower(bot.Prefix)

	if strings.HasPrefix(lowerMessage, lowerPrefix) {
		message = message[len(lowerPrefix):]
	}
	rest := strings.Fields(message)

	if len(rest) > 1 {
		rest = rest[1:]
		return rest
	}

	return nil
}

// ParseCommand parses a message.
func ParseCommand(bot *Rikka, message *disgord.Message) Args {
	return ParseCommandString(bot, message.Content)
}

func (r *Rikka) HandleError(ctx context.Context, s disgord.Session, msg *disgord.Message, err error, errMsg string) {
	r.Log.Error(ctx, errMsg, slog.Error(err))
	_, err = s.SendMsg(ctx, msg.ChannelID, fmt.Sprintf("%s: %s", errMsg, err))
	if err != nil {
		r.Log.Error(ctx, "failed to send error message to channel", slog.Error(err))
	}
}
