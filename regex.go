package rikka

import (
	"regexp"
	"strconv"

	"github.com/andersfylling/disgord"
	"golang.org/x/xerrors"
)

var UserMentionRegex = regexp.MustCompile(`^<@!?(\d+)>$`)
var ChannelMentionRegex = regexp.MustCompile(`^<#!?(\d+)>$`)

func ExtractID(reg *regexp.Regexp, s string) (disgord.Snowflake, error) {
	var (
		match   = ""
		matches = reg.FindStringSubmatch(s)
	)

	if len(matches) < 2 {
		match = s
	} else {
		match = matches[1]
	}

	id, err := strconv.ParseUint(match, 10, 64)
	if err != nil {
		return 0, xerrors.Errorf("parse id: %w", err)
	}

	return disgord.Snowflake(id), nil
}
