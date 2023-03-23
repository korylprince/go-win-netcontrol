package main

import (
	"encoding/base64"
	"fmt"

	"github.com/korylprince/go-win-netcontrol/hash"
)

// default password is "password"
// generate new hash with `HASHPASSWORD="<password>" go test ./hash -v`
// override at build time with `go build -ldflags "-X main.passhashstr=<hash>"`
var passhashstr = "+qhTwm04Dpw5pQooSWds+gAAAAIAAQAAAdOE2CPYWHU5vcTz5fgGTd3dSQiNKW5OA5U+QtsV/ukG"

var passhash = mustParseHash(passhashstr)

func mustParseHash(s string) *hash.Hash {
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(fmt.Errorf("could not decode hash: %w", err))
	}
	h := new(hash.Hash)
	if err := h.UnmarshalBinary(buf); err != nil {
		panic(fmt.Errorf("could not unmarshal hash: %w", err))
	}
	return h
}

// Validate validates the password against the embedded hash
func Validate(pass string) bool {
	return passhash.Validate([]byte(pass)) == nil
}
