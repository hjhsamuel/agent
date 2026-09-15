package ringbuffer

import "sync/atomic"

const Capacity = 1024

type Record[T any] struct {
	Seq   uint64
	Value T
}

type slot[T any] struct {
	record atomic.Pointer[Record[T]]
}

type RingBuffer[T any] struct {
	slots []slot[T]
	mask  uint64
	size  uint64

	writeSeq uint64
	latest   atomic.Uint64
	notify   chan struct{}
	readGen  atomic.Uint64
}

func (r *RingBuffer[T]) Push(value T) uint64 {
	seq := r.writeSeq + 1
	r.writeSeq = seq

	record := &Record[T]{
		Seq:   seq,
		Value: value,
	}

	index := seq & r.mask

	r.slots[index].record.Store(record)
	r.latest.Store(seq)

	select {
	case r.notify <- struct{}{}:
	default:

	}

	return seq
}

func (r *RingBuffer[T]) LatestSeq() uint64 {
	return r.latest.Load()
}

func (r *RingBuffer[T]) OldestSeq() uint64 {
	latest := r.latest.Load()

	if latest == 0 {
		return 0
	}
	if latest < r.size {
		return 1
	}

	return latest - r.size + 1
}

func (r *RingBuffer[T]) Subscribe(afterSeq uint64) *Consumer[T] {
	gen := r.readGen.Add(1)
	var nextSeq uint64
	if afterSeq == 0 {
		oldest := r.OldestSeq()
		if oldest == 0 {
			nextSeq = 1
		} else {
			nextSeq = oldest
		}
	} else {
		nextSeq = afterSeq + 1
	}

	return &Consumer[T]{
		ring:    r,
		nextSeq: nextSeq,
		gen:     gen,
	}
}

func NewRingBuffer[T any](capacity uint64) (*RingBuffer[T], error) {
	// 尽量使用位运算
	if capacity == 0 || capacity&(capacity-1) != 0 {
		capacity = Capacity
	}

	return &RingBuffer[T]{
		slots:  make([]slot[T], capacity),
		mask:   capacity - 1,
		size:   capacity,
		notify: make(chan struct{}, 1),
	}, nil
}
