package logs

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/andersfylling/disgord"
	jsoniter "github.com/json-iterator/go"
)

var msg = &disgord.Message{
	ID:        644584062064918528,
	ChannelID: 319588744023769089,
	Author: &disgord.User{
		ID:            105484726235607040,
		Username:      "thy",
		Discriminator: 1,
		Avatar:        "8c886806025cc2ac357eb058a5d93340",
	},
	Member: &disgord.Member{
		GuildID: 319567980491046913,
		Nick:    "(•̀ᴗ•́)و🍅",
		Roles: []disgord.Snowflake{
			319596717454393345,
			344479246367850497,
			386959952071360523,
			639232963170533415,
			639235921719459845,
			641655666951323660,
		},
		JoinedAt: disgord.Time{Time: time.Now()},
	},
	Content:   "b.stats",
	Timestamp: disgord.Time{Time: time.Now()},
	Nonce:     "644584061779574784",
	GuildID:   319567980491046913,
}

var out []byte

func BenchmarkEncodeJSONstd(b *testing.B) {
	var r []byte
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r, _ = json.Marshal(msg)
	}

	out = r
}

func BenchmarkEncodeJSONiter(b *testing.B) {
	var r []byte
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		r, _ = jsoniter.Marshal(msg)
	}

	out = r
}

func BenchmarkEncodeGob(b *testing.B) {
	var (
		r   []byte
		buf = bytes.NewBuffer(nil)
	)
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc := gob.NewEncoder(buf)
		_ = enc.Encode(msg)
		r = buf.Bytes()
		buf.Reset()
	}

	out = r
}

var (
	jsonEncoded, _ = jsoniter.Marshal(msg)
	decodeOut      *disgord.Message
	_, _           = fmt.Println("json encoded len:", len(jsonEncoded))
)

func BenchmarkDecodeJSONstd(b *testing.B) {
	var msg *disgord.Message
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(jsonEncoded, msg)
	}

	decodeOut = msg
}

func BenchmarkDecodeJSONiter(b *testing.B) {
	var msg *disgord.Message
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = jsoniter.Unmarshal(jsonEncoded, msg)
	}

	decodeOut = msg
}

var (
	gobEncoded = bytes.NewBuffer(nil)
	enc        = gob.NewEncoder(gobEncoded)
	_          = enc.Encode(msg)
	reset      = gobEncoded.Bytes()
	_, _       = fmt.Println("gob encoded len:", gobEncoded.Len())
)

func BenchmarkDecodeGob(b *testing.B) {
	var msg *disgord.Message
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		enc := gob.NewDecoder(gobEncoded)
		_ = enc.Decode(msg)
		gobEncoded.Reset()
		gobEncoded.Write(reset)
	}

	decodeOut = msg
}
