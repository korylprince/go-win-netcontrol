package hash

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// hash errors
var (
	ErrInvalidHashLength = errors.New("invalid hash length")
	ErrInvalidPassword   = errors.New("invalid password")
)

const (
	saltLen       = 16
	keyLen        = 32
	timeOffset    = saltLen
	memoryOffset  = timeOffset + 4
	threadsOffset = memoryOffset + 4
	hashOffset    = threadsOffset + 1

	// HashLen is the length of a marshaled Hash
	HashLen = hashOffset + keyLen
)

// Hash represents an argon2id key with all of its parameters
type Hash struct {
	salt    [saltLen]byte
	time    uint32
	memory  uint32
	threads uint8
	hash    [keyLen]byte
}

// New returns a new Hash with the given password and parameters
func New(password []byte, time, memory uint32, threads uint8) (*Hash, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("could not generate salt")
	}

	hash := argon2.IDKey(password, salt, time, memory, threads, keyLen)

	var s [saltLen]byte
	copy(s[:], salt)

	var h [keyLen]byte
	copy(h[:], hash)

	return &Hash{salt: s, time: time, memory: memory, threads: threads, hash: h}, nil
}

// MarshalBinary encodes h into binary form and returns the result
func (h *Hash) MarshalBinary() ([]byte, error) {
	hash := make([]byte, HashLen)
	copy(hash, h.salt[:])
	binary.BigEndian.PutUint32(hash[timeOffset:], h.time)
	binary.BigEndian.PutUint32(hash[memoryOffset:], h.memory)
	hash[threadsOffset] = h.threads
	copy(hash[hashOffset:], h.hash[:])
	return hash, nil
}

// UnmarshalBinary decodes data into h
func (h *Hash) UnmarshalBinary(data []byte) error {
	if len(data) != HashLen {
		return ErrInvalidHashLength
	}
	copy(h.salt[:], data[:saltLen])
	h.time = binary.BigEndian.Uint32(data[timeOffset:memoryOffset])
	h.memory = binary.BigEndian.Uint32(data[memoryOffset:threadsOffset])
	h.threads = data[threadsOffset]
	copy(h.hash[:], data[hashOffset:])
	return nil
}

// Validate returns an error if the password cannot be validated against the Hash
func (h *Hash) Validate(password []byte) error {
	hash2 := argon2.IDKey(password, h.salt[:], h.time, h.memory, h.threads, keyLen)
	if subtle.ConstantTimeEq(int32(len(h.hash)), int32(len(hash2))) != 1 {
		return ErrInvalidPassword
	}
	if subtle.ConstantTimeCompare(h.hash[:], hash2) != 1 {
		return ErrInvalidPassword
	}
	return nil
}
