package ulid

import (
	"regexp"
	"time"

	"github.com/oklog/ulid/v2"
)

var generator = DefaultGenerator

// IsULID checks if the given string is a valid ULID
// ULID pattern:
//
//	 01AN4Z07BY      79KA1307SR9X4MV3
//	|----------|    |----------------|
//	 Timestamp          Randomness
//
// 10 characters     16 characters
// Crockford's Base32 is used (excludes I, L, O, and U to avoid confusion and abuse)
func isULID(s string) bool {
	ulidRegex := `^[0123456789ABCDEFGHJKMNPQRSTVWXYZ]{26}$`
	matched, _ := regexp.MatchString(ulidRegex, s)
	return matched
}

// ValidID checks if the given id is valid
func ValidID(id string) bool {
	_, err := ulid.Parse(id)

	return err == nil && isULID(id)
}

// GenerateID generates a new universal ID
func GenerateID() string {
	return generator()
}

func DefaultGenerator() string {
	entropy := ulid.DefaultEntropy()
	now := time.Now()
	ts := ulid.Timestamp(now)
	return ulid.MustNew(ts, entropy).String()
}

func ResetGenerator() {
	generator = DefaultGenerator
}

func MockGenerator(mockValue string) {
	generator = func() string {
		return mockValue
	}
}
