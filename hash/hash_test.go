package hash_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/korylprince/go-win-netcontrol/hash"
)

func TestHash(t *testing.T) {
	pass := "password"
	if p := os.Getenv("HASHPASSWORD"); p != "" {
		pass = p
	}
	h, err := hash.New([]byte(pass), 2, 64*1024, 1)
	if err != nil {
		t.Fatalf("create hash error: want: nil, have: %v", err)
	}

	hBytes, err := h.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal hash error: want: nil, have: %v", err)
	}

	if len(hBytes) != hash.HashLen {
		t.Errorf("hash length: want: %d, have: %d", hash.HashLen, len(hBytes))
	}

	if err = h.Validate([]byte(pass)); err != nil {
		t.Errorf("validate error: want: nil, have: %v", err)
	}

	if err = h.Validate([]byte([]byte("bad"))); !errors.Is(err, hash.ErrInvalidPassword) {
		t.Errorf(`invalid password error: want: "invalid password", have: %v`, err)
	}

	h2 := new(hash.Hash)
	if err = h2.UnmarshalBinary(hBytes[1:]); err == nil {
		t.Fatalf(`invalid hash length: want: "invalid hash length", have: %v`, err)
	}

	if err = h2.UnmarshalBinary(hBytes); err != nil {
		t.Fatalf("unmarshal hash error: want: nil, have: %v", err)
	}

	if err = h2.Validate([]byte(pass)); err != nil {
		t.Fatalf("unmarshaled validate error: want: nil, have: %v", err)
	}

	h2Bytes, err := h2.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal hash error: want: nil, have: %v", err)
	}

	if len(h2Bytes) != hash.HashLen {
		t.Errorf("unmarshaled hash length: want: %d, have: %d", hash.HashLen, len(h2Bytes))
	}

	if !bytes.Equal(hBytes, h2Bytes) {
		t.Errorf("hash comparison: want: %s, have: %s", base64.StdEncoding.EncodeToString(hBytes), base64.StdEncoding.EncodeToString(h2Bytes))
	}

	fmt.Printf("%s: %#v\n", pass, base64.StdEncoding.EncodeToString(hBytes))
}
