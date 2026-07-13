package security

import (
	"errors"
	"os"
	"strings"
)

const redactedValue = "[REDACTED]"

func RedactString(value string) string {
	key := strings.TrimSpace(os.Getenv("HYPIXEL_API_KEY"))
	if key == "" {
		return value
	}
	return strings.ReplaceAll(value, key, redactedValue)
}

func RedactError(err error) error {
	if err == nil {
		return nil
	}
	return redactedError{err: err}
}

type redactedError struct {
	err error
}

func (err redactedError) Error() string {
	return RedactString(err.err.Error())
}

func (err redactedError) Unwrap() error {
	return err.err
}

func (err redactedError) Is(target error) bool {
	return errors.Is(err.err, target)
}
