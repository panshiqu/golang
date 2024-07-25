package timer

import (
	"log"
	"sync"
	"testing"
	"time"
)

func callback(args ...any) error {
	log.Println("callback", args, time.Now().UnixMilli())
	return nil
}

func TestHeap(t *testing.T) {
	h := &Heap{M: &sync.Mutex{}}
	h.AddRepeat(time.Second*1, callback, 1, "one")
	h.Add(time.Second*2, callback, 2, "two")
	three := h.AddRepeat(time.Second*3, callback, 3, "three")
	h.Add(time.Second*4, callback, 4, "four")
	five := h.AddRepeat(time.Second*5, callback, 5, "five")
	h.Add(time.Second*8, callback, 8, "eight")
	sixteen := h.Add(time.Second*16, callback, 16, "sixteen")
	twenty := h.Add(time.Second*16, callback, 20, "twenty")
	deadline := time.Now().Add(13 * time.Second)
	for time.Now().Before(deadline) {
		<-h.Check()
	}
	three.Stop()
	sixteen.Stop()
	five.Reset(4 * time.Second)
	twenty.Reset(7 * time.Second)
	deadline = time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		<-h.Check()
	}
}
