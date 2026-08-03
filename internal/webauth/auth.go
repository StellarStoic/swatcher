// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package webauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 2
	argonKeyLen  = 32
)

type Config struct {
	Salt          string `json:"salt"`
	PasswordHash  string `json:"passwordHash"`
	SessionSecret string `json:"sessionSecret"`
}

func SetPassword(path, password string) error {
	if len(password) < 5 {
		return errors.New("password must contain at least 5 characters")
	}
	salt := make([]byte, 16)
	secret := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate password salt: %w", err)
	}
	if _, err := rand.Read(secret); err != nil {
		return fmt.Errorf("generate session secret: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	config := Config{Salt: base64.RawURLEncoding.EncodeToString(salt), PasswordHash: base64.RawURLEncoding.EncodeToString(hash), SessionSecret: base64.RawURLEncoding.EncodeToString(secret)}
	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Load(path string) (Config, error) {
	var config Config
	b, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(b, &config); err != nil {
		return config, err
	}
	if config.Salt == "" || config.PasswordHash == "" || config.SessionSecret == "" {
		return Config{}, errors.New("web password configuration is incomplete")
	}
	return config, nil
}

func Verify(config Config, password string) bool {
	salt, saltErr := base64.RawURLEncoding.DecodeString(config.Salt)
	want, hashErr := base64.RawURLEncoding.DecodeString(config.PasswordHash)
	if saltErr != nil || hashErr != nil || len(want) != argonKeyLen {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func NewSession(config Config, expires time.Time) (string, error) {
	secret, err := base64.RawURLEncoding.DecodeString(config.SessionSecret)
	if err != nil || len(secret) < 32 {
		return "", errors.New("invalid session secret")
	}
	payload := strconv.FormatInt(expires.Unix(), 10)
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func ValidSession(config Config, token string, now time.Time) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	expires, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || now.Unix() >= expires {
		return false
	}
	secret, secretErr := base64.RawURLEncoding.DecodeString(config.SessionSecret)
	provided, macErr := base64.RawURLEncoding.DecodeString(parts[1])
	if secretErr != nil || macErr != nil {
		return false
	}
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(parts[0]))
	return hmac.Equal(provided, mac.Sum(nil))
}
