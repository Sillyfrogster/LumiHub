// Package signing stamps short-lived signatures on the byte-delivery URLs Go
// serves privately. A draft's images keep the media record, the blob and the
// URL a published asset has, and the signature is the whole of what withholds
// them.
package signing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Life is how long a signature stays good for. It only has to outlive the page
// it was minted on.
const Life = 15 * time.Minute

// ExpiresParam and SignatureParam are the query parameters a signature travels
// in.
const (
	ExpiresParam   = "expires"
	SignatureParam = "signature"
)

// Key signs and checks one deployment's private URLs.
type Key struct {
	secret []byte
}

// NewKey draws a key that lives as long as the process. A signature only has to
// outlive the page it was minted on, so nothing is stored, there is no secret
// to leak or rotate, and a restart costs a page reload. A second API process
// would need a shared key instead.
func NewKey() Key {
	secret := make([]byte, 32)
	rand.Read(secret)
	return Key{secret: secret}
}

// Sign returns path with the query a private request has to carry.
func (k Key) Sign(path string, now time.Time) string {
	expires := now.Add(Life).Unix()
	query := url.Values{}
	query.Set(ExpiresParam, strconv.FormatInt(expires, 10))
	query.Set(SignatureParam, k.stamp(path, expires))
	return path + "?" + query.Encode()
}

// Valid reports whether a request carries a signature this key wrote for this
// path and has not run out.
func (k Key) Valid(path, expires, signature string, now time.Time) bool {
	if len(k.secret) == 0 {
		return false
	}
	deadline, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || now.After(time.Unix(deadline, 0)) {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(k.stamp(path, deadline)))
}

func (k Key) stamp(path string, expires int64) string {
	mac := hmac.New(sha256.New, k.secret)
	fmt.Fprintf(mac, "%s\n%d", path, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
