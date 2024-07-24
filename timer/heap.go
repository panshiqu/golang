package timer

import (
	"container/heap"
	"log/slog"
	"sync"
	"time"
)

type TimerHeap struct {
	M *sync.Mutex
	s []*Timer
}

type Timer struct {
	expire   int64 // 到期时间
	interval int64 // !=0 重复间隔

	fn   func(...any) error
	args []any
}

func (h *TimerHeap) Len() int           { return len(h.s) }
func (h *TimerHeap) Less(i, j int) bool { return h.s[i].expire < h.s[j].expire }
func (h *TimerHeap) Swap(i, j int)      { h.s[i], h.s[j] = h.s[j], h.s[i] }

func (h *TimerHeap) Push(x any) {
	if h.M != nil {
		h.M.Lock()
		defer h.M.Unlock()
	}

	h.s = append(h.s, x.(*Timer))
}

func (h *TimerHeap) Pop() any {
	if h.M != nil {
		h.M.Lock()
		defer h.M.Unlock()
	}

	old := h.s
	n := len(old)
	x := old[n-1]
	h.s = old[0 : n-1]
	return x
}

func (h *TimerHeap) first() *Timer {
	if h.M != nil {
		h.M.Lock()
		defer h.M.Unlock()
	}

	if h.Len() == 0 {
		return nil
	}

	return h.s[0]
}

func (h *TimerHeap) Add(t time.Duration, fn func(...any) error, args ...any) {
	heap.Push(h, &Timer{
		expire: time.Now().Add(t).UnixMilli(),
		fn:     fn,
		args:   args,
	})
}

func (h *TimerHeap) AddRepeat(t time.Duration, fn func(...any) error, args ...any) {
	heap.Push(h, &Timer{
		expire:   time.Now().Add(t).UnixMilli(),
		interval: t.Milliseconds(),
		fn:       fn,
		args:     args,
	})
}

func (h *TimerHeap) Check() <-chan time.Time {
	t := h.first()
	if t == nil {
		// receive from nil channel blocks forever
		return nil
	}
	if ms := time.Now().UnixMilli(); ms < t.expire {
		return time.After(time.Duration(t.expire-ms) * time.Millisecond)
	}

	if t.interval != 0 {
		t.expire += t.interval
		heap.Fix(h, 0)
	} else {
		heap.Pop(h)
	}

	if err := t.fn(t.args...); err != nil {
		slog.Error("check", slog.Any("err", err))
	}

	return h.Check()
}
