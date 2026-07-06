package secrets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

const tokenPrefix = "pine:v1:"

func EncodeToken(serverURL, username, token string) string {
	if token == "" {
		return ""
	}
	return tokenPrefix + base64.RawStdEncoding.EncodeToString(xorToken(serverURL, username, []byte(token)))
}

func DecodeToken(serverURL, username, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, tokenPrefix) {
		return stored, nil
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, tokenPrefix))
	if err != nil {
		return "", fmt.Errorf("decode obfuscated token: %w", err)
	}
	return string(xorToken(serverURL, username, data)), nil
}

func IsObfuscatedToken(stored string) bool {
	return strings.HasPrefix(stored, tokenPrefix)
}

func xorToken(serverURL, username string, data []byte) []byte {
	out := make([]byte, len(data))
	seed := []byte("pine token obfuscation v1|" + strings.TrimRight(serverURL, "/") + "|" + username)
	var counter uint64
	for offset := 0; offset < len(data); {
		var blockInput []byte
		blockInput = append(blockInput, seed...)
		var counterBytes [8]byte
		binary.LittleEndian.PutUint64(counterBytes[:], counter)
		blockInput = append(blockInput, counterBytes[:]...)
		block := sha256.Sum256(blockInput)
		for _, b := range block {
			if offset >= len(data) {
				break
			}
			out[offset] = data[offset] ^ b
			offset++
		}
		counter++
	}
	return out
}
