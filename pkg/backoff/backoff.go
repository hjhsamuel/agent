package backoff

import (
	"math/rand/v2"
	"time"
)

type Backoff struct {
	base time.Duration
	max  time.Duration

	ratio float64
}

func (b *Backoff) Delay(index int) time.Duration {
	if index < 0 {
		index = 0
	}

	delay := b.base
	for i := 0; i < index; i++ {
		if delay >= b.max/2 {
			delay = b.max
			break
		}
		delay *= 2
	}

	if delay > b.max {
		delay = b.max
	}
	if b.ratio <= 0 {
		return delay
	}

	maxJitter := time.Duration(float64(delay) * b.ratio)
	if maxJitter <= 0 {
		return delay
	}

	jitter := time.Duration(rand.Int64N(int64(maxJitter) + 1))
	return delay + jitter
}

func NewBackoff(
	base time.Duration,
	max time.Duration,
	ratio float64,
) *Backoff {
	if base <= 0 {
		base = time.Millisecond * 500
	}
	if max < base {
		max = base
	}
	if ratio < 0 {
		ratio = 0
	}

	return &Backoff{base: base, max: max, ratio: ratio}
}
