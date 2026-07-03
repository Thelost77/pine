package secrets

import (
	"errors"
	"strings"

	"github.com/zalando/go-keyring"
)

const Service = "pine"

var ErrNotFound = keyring.ErrNotFound

func tokenAccount(serverURL, username string) string {
	return strings.TrimRight(serverURL, "/") + "|" + username
}

func GetToken(serverURL, username string) (string, error) {
	token, err := keyring.Get(Service, tokenAccount(serverURL, username))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return token, err
}

func SetToken(serverURL, username, token string) error {
	return keyring.Set(Service, tokenAccount(serverURL, username), token)
}

func DeleteToken(serverURL, username string) error {
	err := keyring.Delete(Service, tokenAccount(serverURL, username))
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
