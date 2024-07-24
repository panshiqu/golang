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

func (h *TimerHeap) Lock() {
	if h.M != nil {
		h.M.Lock()
	}
}
func (h *TimerHeap) Unlock() {
	if h.M != nil {
		h.M.Unlock()
	}
}
func (h *TimerHeap) Len() int           { return len(h.s) }
func (h *TimerHeap) Less(i, j int) bool { return h.s[i].expire < h.s[j].expire }
func (h *TimerHeap) Swap(i, j int)      { h.s[i], h.s[j] = h.s[j], h.s[i] }

func (h *TimerHeap) Push(x any) {
	h.s = append(h.s, x.(*Timer))
}

func (h *TimerHeap) Pop() any {
	old := h.s
	n := len(old)
	x := old[n-1]
	h.s = old[0 : n-1]
	return x
}

func (h *TimerHeap) Add(t time.Duration, fn func(...any) error, args ...any) {
	h.Lock()
	defer h.Unlock()

	heap.Push(h, &Timer{
		expire: time.Now().Add(t).UnixMilli(),
		fn:     fn,
		args:   args,
	})
}

func (h *TimerHeap) AddRepeat(t time.Duration, fn func(...any) error, args ...any) {
	h.Lock()
	defer h.Unlock()

	heap.Push(h, &Timer{
		expire:   time.Now().Add(t).UnixMilli(),
		interval: t.Milliseconds(),
		fn:       fn,
		args:     args,
	})
}

func (h *TimerHeap) Check() <-chan time.Time {
	h.Lock()
	if h.Len() == 0 {
		h.Unlock()
		// receive from nil channel blocks forever
		return nil
	}

	t := h.s[0]
	if n := t.expire - time.Now().UnixMilli(); n > 0 {
		h.Unlock()
		return time.After(time.Duration(n) * time.Millisecond)
	}

	if t.interval != 0 {
		t.expire += t.interval
		heap.Fix(h, 0)
	} else {
		heap.Pop(h)
	}
	h.Unlock()

	if err := t.fn(t.args...); err != nil {
		slog.Error("check", slog.Any("err", err))
	}

	return h.Check()
}
