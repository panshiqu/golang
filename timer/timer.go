package timer

import (
	"container/heap"
	"time"
)

type Timer struct {
	index    int
	expire   int64 // 到期时间
	interval int64 // !=0 重复间隔

	h *Heap

	fn   func(...any) error
	args []any
}

func (t *Timer) Reset(d time.Duration) {
	t.h.Lock()
	defer t.h.Unlock()

	t.expire = time.Now().Add(d).UnixMilli()
	if t.interval != 0 {
		t.interval = d.Milliseconds()
	}
	heap.Fix(t.h, t.index)
}

func (t *Timer) Stop() {
	t.h.Lock()
	defer t.h.Unlock()

	heap.Remove(t.h, t.index)
}

func (t *Timer) Milli() int64 {
	t.h.Lock()
	defer t.h.Unlock()

	if n := time.Now().UnixMilli(); t.expire > n {
		return t.expire - n
	}
	return 0
}
