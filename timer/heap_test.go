package timer

import (
	"log"
	"testing"
	"time"
)

func callback(args ...any) error {
	log.Println("callback", args, time.Now().UnixMilli())
	return nil
}

func TestHeap(t *testing.T) {
	h := &MutexTimerHeap{}
	h.AddRepeat(time.Second*1, callback, 1, "one")
	h.Add(time.Second*2, callback, 2, "two")
	h.AddRepeat(time.Second*3, callback, 3, "three")
	h.Add(time.Second*4, callback, 4, "four")
	h.AddRepeat(time.Second*5, callback, 5, "five")
	h.Add(time.Second*8, callback, 8, "eight")
	h.Add(time.Second*16, callback, 16, "sixteen")
	for i := 0; i < 268; i++ {
		time.Sleep(time.Millisecond * 100)
		h.Check()
	}
}
