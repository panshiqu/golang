package timer

import (
	"container/heap"
	"log/slog"
	"sync"
	"time"
)

type MutexTimerHeap struct {
	sync.Mutex
	TimerHeap
}

// 仅用于无锁环境
type TimerHeap []*Timer

type Timer struct {
	expire   int64 // 到期时间
	interval int64 // !=0 重复间隔

	fn   func(...any) error
	args []any
}

func (h TimerHeap) Len() int           { return len(h) }
func (h TimerHeap) Less(i, j int) bool { return h[i].expire < h[j].expire }
func (h TimerHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TimerHeap) Push(x any) {
	*h = append(*h, x.(*Timer))
}

func (h *TimerHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
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

func (h *TimerHeap) Check() {
	if h.Len() == 0 {
		return
	}

	t := (*h)[0]
	if time.Now().UnixMilli() < t.expire {
		return
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

	h.Check()
}

func (h *MutexTimerHeap) Add(t time.Duration, fn func(...any) error, args ...any) {
	h.Lock()
	defer h.Unlock()
	h.TimerHeap.Add(t, fn, args...)
}

func (h *MutexTimerHeap) AddRepeat(t time.Duration, fn func(...any) error, args ...any) {
	h.Lock()
	defer h.Unlock()
	h.TimerHeap.AddRepeat(t, fn, args...)
}

func (h *MutexTimerHeap) Check() {
	h.Lock()
	defer h.Unlock()
	h.TimerHeap.Check()
}
