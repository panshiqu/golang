package timer

import (
	"container/heap"
	"log/slog"
	"sync"
	"time"
)

type Heap struct {
	M *sync.Mutex
	s []*Timer
}

func (h *Heap) Lock() {
	if h.M != nil {
		h.M.Lock()
	}
}

func (h *Heap) Unlock() {
	if h.M != nil {
		h.M.Unlock()
	}
}

func (h *Heap) Len() int           { return len(h.s) }
func (h *Heap) Less(i, j int) bool { return h.s[i].expire < h.s[j].expire }
func (h *Heap) Swap(i, j int) {
	h.s[i], h.s[j] = h.s[j], h.s[i]
	h.s[i].index = i
	h.s[j].index = j
}

func (h *Heap) Push(x any) {
	n := len(h.s)
	item := x.(*Timer)
	item.index = n
	h.s = append(h.s, item)
}

func (h *Heap) Pop() any {
	old := h.s
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	h.s = old[0 : n-1]
	return item
}

func (h *Heap) Add(d time.Duration, fn func(...any) error, args ...any) *Timer {
	h.Lock()
	defer h.Unlock()

	t := &Timer{
		expire: time.Now().Add(d).UnixMilli(),
		h:      h,
		fn:     fn,
		args:   args,
	}
	heap.Push(h, t)
	return t
}

func (h *Heap) AddRepeat(d time.Duration, fn func(...any) error, args ...any) *Timer {
	h.Lock()
	defer h.Unlock()

	t := &Timer{
		expire:   time.Now().Add(d).UnixMilli(),
		interval: d.Milliseconds(),
		h:        h,
		fn:       fn,
		args:     args,
	}
	heap.Push(h, t)
	return t
}

func (h *Heap) expire() (*Timer, <-chan time.Time) {
	h.Lock()
	defer h.Unlock()

	if h.Len() == 0 {
		// receive from nil channel blocks forever
		return nil, nil
	}

	t := h.s[0]
	if n := t.expire - time.Now().UnixMilli(); n > 0 {
		return nil, time.After(time.Duration(n) * time.Millisecond)
	}

	if t.interval != 0 {
		t.expire += t.interval
		heap.Fix(h, 0)
	} else {
		heap.Pop(h)
	}

	return t, nil
}

func (h *Heap) Check() <-chan time.Time {
	t, c := h.expire()
	if t == nil {
		return c
	}

	if err := t.fn(t.args...); err != nil {
		slog.Error("check", slog.Any("err", err))
	}

	return h.Check()
}
