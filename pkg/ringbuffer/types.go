package ringbuffer

import (
	"errors"
	"fmt"
)

var (
	ErrCursorExpired    = errors.New("ringbuffer cursor expired")
	ErrConsumerReplaced = errors.New("consumer replaced")
)

type CursorExpiredError struct {
	Cursor    uint64
	OldestSeq uint64
	LatestSeq uint64
}

func (e *CursorExpiredError) Error() string {
	return fmt.Sprintf(
		"%v: cursor=%d oldest=%d latest=%d",
		ErrCursorExpired,
		e.Cursor,
		e.OldestSeq,
		e.LatestSeq,
	)
}

func (e *CursorExpiredError) Unwrap() error {
	return ErrCursorExpired
}
