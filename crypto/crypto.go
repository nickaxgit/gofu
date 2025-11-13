package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"math/rand/v2"
)

func Random16string() string {
	const letterBytes = "1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 16)
	for i := range b {
		b[i] = letterBytes[rand.Int32N(int32(len(letterBytes)))]
	}
	return string(b)
}

func Hash(pw string, salt string) string {

	hasher := sha256.New()
	hasher.Write([]byte(pw + salt + "$~pepper3n3ss!"))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))

	return sha
}
