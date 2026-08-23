package format

import (
	"errors"
	"fmt"
)

type FailureReason string

const (
	FailureMalformedInput     FailureReason = "malformed_input"
	FailureUnsupportedFormat  FailureReason = "unsupported_format"
	FailureUnsupportedVersion FailureReason = "unsupported_version"
	FailureSafetyViolation    FailureReason = "safety_violation"
	FailureWrongKind          FailureReason = "wrong_kind"
	FailureLimitExceeded      FailureReason = "limit_exceeded"
	FailureInternal           FailureReason = "internal_failure"
)

type failure struct {
	reason FailureReason
	cause  error
}

func (f failure) Error() string { return fmt.Sprintf("%s: %v", f.reason, f.cause) }
func (f failure) Unwrap() error { return f.cause }

// UnsupportedVersion marks a format revision the module cannot safely read.
func UnsupportedVersion(err error) error {
	return failure{reason: FailureUnsupportedVersion, cause: err}
}

// MalformedInput marks a recognized payload the reader cannot interpret.
func MalformedInput(err error) error {
	return failure{reason: FailureMalformedInput, cause: err}
}

// SafetyViolation marks input the module refuses for safety.
func SafetyViolation(err error) error {
	return failure{reason: FailureSafetyViolation, cause: err}
}

// LimitExceeded marks valid input that is too large to import.
func LimitExceeded(err error) error {
	return failure{reason: FailureLimitExceeded, cause: err}
}

// InternalFailure marks a failure that may succeed on retry.
func InternalFailure(err error) error {
	return failure{reason: FailureInternal, cause: err}
}

// FailureOf returns the failure category carried by err.
func FailureOf(err error) (FailureReason, bool) {
	var classified failure
	if !errors.As(err, &classified) {
		return "", false
	}
	return classified.reason, true
}
