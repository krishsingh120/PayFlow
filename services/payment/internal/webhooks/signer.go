package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidSignatureHeader is returned when the signature header format is malformed.
	ErrInvalidSignatureHeader = errors.New("invalid signature header format")

	// ErrTimestampTolerance is returned when the webhook timestamp drifts outside the tolerance window.
	ErrTimestampTolerance = errors.New("webhook timestamp outside allowed tolerance")

	// ErrSignatureMismatch is returned when the cryptographic signature does not match.
	ErrSignatureMismatch = errors.New("webhook signature does not match")
)

// Sign generates an HMAC-SHA256 signature for the given payload and timestamp.
// Format: t=<timestamp>,v1=<hex_signature>
func Sign(secret string, timestamp int64, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("t=%d,v1=%s", timestamp, sig)
}

// Verify validates the signature header against the secret, payload, and time tolerance.
// Uses hmac.Equal for constant-time comparison to prevent timing attacks.
func Verify(secret string, payload []byte, header string, tolerance time.Duration) error {
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}

	parts := strings.Split(header, ",")
	var timestampStr, signature string

	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestampStr = kv[1]
		case "v1":
			signature = kv[1]
		}
	}

	if timestampStr == "" || signature == "" {
		return ErrInvalidSignatureHeader
	}

	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return ErrInvalidSignatureHeader
	}

	// Replay attack prevention: verify timestamp within tolerance window
	eventTime := time.Unix(ts, 0)
	diff := time.Since(eventTime)
	if diff < -tolerance || diff > tolerance {
		return ErrTimestampTolerance
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d.", ts)))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return ErrSignatureMismatch
	}

	return nil
}
