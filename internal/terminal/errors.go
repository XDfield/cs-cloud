package terminal

import (
	"errors"
	"fmt"
)

var ErrSessionLimit = errors.New("terminal session limit reached")

type SessionCreateError struct {
	Err error
}

func (e *SessionCreateError) Error() string {
	if e == nil || e.Err == nil {
		return "terminal: create session failed"
	}
	return fmt.Sprintf("terminal: create session: %v", e.Err)
}

func (e *SessionCreateError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsSessionCreateError(err error) bool {
	var target *SessionCreateError
	return errors.As(err, &target)
}
