package ringbuffer

import (
	"context"
	"fmt"
)

type Consumer[T any] struct {
	ring    *RingBuffer[T]
	nextSeq uint64
	sub     *subscription
}

func (c *Consumer[T]) Cursor() uint64 {
	if c.nextSeq == 0 {
		return 0
	}
	return c.nextSeq - 1
}

func (c *Consumer[T]) checkGen() error {
	if c.ring.subscriber.Load() != c.sub {
		return ErrConsumerReplaced
	}
	return nil
}

func (c *Consumer[T]) TryNext() (*Record[T], bool, error) {
	if err := c.checkGen(); err != nil {
		return nil, false, err
	}

	latest := c.ring.latest.Load()

	if c.nextSeq > latest {
		return nil, false, nil
	}

	oldest := uint64(1)
	if latest >= c.ring.size {
		oldest = latest - c.ring.size + 1
	}

	if c.nextSeq < oldest {
		return nil, false, &CursorExpiredError{
			Cursor:    c.nextSeq - 1,
			OldestSeq: oldest,
			LatestSeq: latest,
		}
	}

	index := c.nextSeq & c.ring.mask

	record := c.ring.slots[index].record.Load()
	if record == nil {
		return nil, false, nil
	}

	if record.Seq > c.nextSeq {
		return nil, false, &CursorExpiredError{
			Cursor:    c.nextSeq - 1,
			OldestSeq: record.Seq,
			LatestSeq: latest,
		}
	}

	if record.Seq < c.nextSeq {
		return nil, false, nil
	}

	c.nextSeq++

	return record, true, nil
}

func (c *Consumer[T]) Next(ctx context.Context) (*Record[T], error) {
	for {
		record, ok, err := c.TryNext()
		if err != nil {
			return nil, err
		}
		if ok {
			return record, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.sub.done:
			return nil, ErrConsumerReplaced
		case <-c.sub.notify:

		}
	}
}

func (c *Consumer[T]) Drain() ([]*Record[T], error) {
	if err := c.checkGen(); err != nil {
		return nil, err
	}

	ring := c.ring
	latest := ring.latest.Load()
	if c.nextSeq > latest {
		return nil, nil
	}

	oldest := uint64(1)
	if latest >= ring.size {
		oldest = latest - ring.size + 1
	}

	if c.nextSeq < oldest {
		return nil, &CursorExpiredError{
			Cursor:    c.nextSeq - 1,
			OldestSeq: oldest,
			LatestSeq: latest,
		}
	}

	count := latest - c.nextSeq + 1
	records := make([]*Record[T], 0, count)
	for seq := c.nextSeq; seq <= latest; seq++ {
		index := seq & ring.mask
		record := ring.slots[index].record.Load()
		if record == nil {
			// 正常不会进入
			return nil, fmt.Errorf("ringbuffer record missing: seq=%d", seq)
		}
		if record.Seq > seq {
			// 已被 producer 覆盖
			return nil, &CursorExpiredError{
				Cursor:    seq - 1,
				OldestSeq: record.Seq,
				LatestSeq: ring.latest.Load(),
			}
		}
		if record.Seq < seq {
			return nil, fmt.Errorf("ringbuffer record not published: expected %d actual %d", seq, record.Seq)
		}

		records = append(records, record)
	}
	c.nextSeq = latest + 1
	return records, nil
}

func (c *Consumer[T]) Notify() <-chan struct{} {
	return c.sub.notify
}

func (c *Consumer[T]) Done() <-chan struct{} { return c.sub.done }
