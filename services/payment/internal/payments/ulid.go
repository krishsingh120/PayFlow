package payments

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyLock sync.Mutex
	entropyPool = ulid.Monotonic(rand.Reader, 0)
)

// NewPaymentID generates a cryptographically-seeded, monotonic ULID prefixed with "pay_".
// ULIDs provide millisecond precision ordering, URL safety, and 128-bit collision resistance.
func NewPaymentID() string {
	entropyLock.Lock()
	defer entropyLock.Unlock()

	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropyPool)
	return fmt.Sprintf("pay_%s", id.String())
}

// NewRefundID generates a monotonic ULID prefixed with "ref_".
func NewRefundID() string {
	entropyLock.Lock()
	defer entropyLock.Unlock()

	id := ulid.MustNew(ulid.Timestamp(time.Now()), entropyPool)
	return fmt.Sprintf("ref_%s", id.String())
}
