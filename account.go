package main

import (
	"crypto/sha256"
	"encoding/base64"
	"math/rand/v2"
)

type account struct {
	playerName string
	guid       string //pemanent id
	pwHash     string
	salt       string
	coins      int
	//friends[] account - store indices
}

func NewAccount(playerName string, pw string) account {
	salt := generateSalt()
	pwHash := hash(pw, salt)
	guid := generateSalt()
	acc := account{playerName, guid, pwHash, salt, 0}
	accountsByGuid[guid] = &acc
	return acc
}

func generateSalt() string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Int32N(int32(len(letterBytes)))]
	}
	return string(b)
}

func hash(pw string, salt string) string {

	hasher := sha256.New()
	hasher.Write([]byte(pw + salt))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	return sha
}
