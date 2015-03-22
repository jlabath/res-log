package library

import (
	"crypto/sha256"
	"encoding/hex"
)

//AppKey256 is the hash of our app key we need to include in response upon receiving a webhook
var AppKey256 string

func init() {
	md := sha256.Sum256([]byte(cfg.AppKey))
	AppKey256 = hex.EncodeToString(md[:])
}
