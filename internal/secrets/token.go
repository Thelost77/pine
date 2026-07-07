package secrets

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const (
	tokenPrefixV2 = "pine:v2:"
	nonceSize     = 16
	macSize       = sha256.Size
)

var machineIdentity = detectMachineIdentity

func EncodeToken(_, _, token string) string {
	if token == "" {
		return ""
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		fallback := sha256.Sum256([]byte(machineIdentity() + "|" + token))
		copy(nonce, fallback[:nonceSize])
	}
	encKey, macKey := tokenKeys()
	ciphertext := xorTokenV2(encKey, nonce, []byte(token))
	mac := tokenMAC(macKey, nonce, ciphertext)
	payload := make([]byte, 0, len(nonce)+len(ciphertext)+len(mac))
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	payload = append(payload, mac...)
	return tokenPrefixV2 + base64.RawStdEncoding.EncodeToString(payload)
}

func DecodeToken(serverURL, username, stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if strings.HasPrefix(stored, tokenPrefixV2) {
		return decodeTokenV2(stored)
	}
	if strings.HasPrefix(stored, "pine:") {
		return "", fmt.Errorf("decode obfuscated token: unsupported token format")
	}
	return stored, nil
}

func IsObfuscatedToken(stored string) bool {
	return strings.HasPrefix(stored, tokenPrefixV2)
}

func IsCurrentToken(stored string) bool {
	return strings.HasPrefix(stored, tokenPrefixV2)
}

func decodeTokenV2(stored string) (string, error) {
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, tokenPrefixV2))
	if err != nil {
		return "", fmt.Errorf("decode obfuscated token: %w", err)
	}
	if len(data) < nonceSize+macSize {
		return "", fmt.Errorf("decode obfuscated token: payload too short")
	}
	nonce := data[:nonceSize]
	macStart := len(data) - macSize
	ciphertext := data[nonceSize:macStart]
	storedMAC := data[macStart:]
	encKey, macKey := tokenKeys()
	expectedMAC := tokenMAC(macKey, nonce, ciphertext)
	if !hmac.Equal(storedMAC, expectedMAC) {
		return "", fmt.Errorf("decode obfuscated token: integrity check failed")
	}
	return string(xorTokenV2(encKey, nonce, ciphertext)), nil
}

func tokenKeys() ([sha256.Size]byte, [sha256.Size]byte) {
	root := sha256.Sum256([]byte(machineIdentity() + "|" + staticPepper()))
	encKey := sha256.Sum256(append(root[:], []byte("|enc")...))
	macKey := sha256.Sum256(append(root[:], []byte("|mac")...))
	return encKey, macKey
}

func staticPepper() string {
	parts := []string{"pi", "ne", "|tok", "en|", "obf", "usc", "ati", "on|", "v2"}
	sum := sha256.Sum256([]byte(strings.Join(parts, "")))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}

func tokenMAC(key [sha256.Size]byte, nonce, ciphertext []byte) []byte {
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(tokenPrefixV2))
	mac.Write(nonce)
	mac.Write(ciphertext)
	return mac.Sum(nil)
}

func xorTokenV2(key [sha256.Size]byte, nonce, data []byte) []byte {
	out := make([]byte, len(data))
	var counter uint64
	for offset := 0; offset < len(data); {
		blockInput := make([]byte, 0, len(key)+len(nonce)+8)
		blockInput = append(blockInput, key[:]...)
		blockInput = append(blockInput, nonce...)
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

func detectMachineIdentity() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(path)
		if err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id
			}
		}
	}
	name, err := os.Hostname()
	if err == nil && strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return "unknown-machine"
}
