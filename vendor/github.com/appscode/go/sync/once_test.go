package sync_test

import (
	"testing"

	. "github.com/appscode/go/sync"
)

type one int

func (o *one) Increment() {
	*o++
}

func run(t *testing.T, once *Once, o *one, c chan bool) {
	once.Do(func() error { o.Increment(); return nil })
	if v := *o; v != 1 {
		t.Errorf("once failed inside run: %d is not 1", v)
	}
	c <- true
}

func TestOnce(t *testing.T) {
	o := new(one)
	once := new(Once)
	c := make(chan bool)
	const N = 10
	for i := 0; i < N; i++ {
		go run(t, once, o, c)
	}
	for i := 0; i < N; i++ {
		<-c
	}
	if *o != 1 {
		t.Errorf("once failed outside run: %d is not 1", *o)
	}
}

func TestOncePanic(t *testing.T) {
	var once Once
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("Once.Do did not panic")
			}
		}()
		once.Do(func() error {
			panic("failed")
			return nil
		})
	}()

	once.Do(func() error {
		t.Fatalf("Once.Do called twice")
		return nil
	})
}

func BenchmarkOnce(b *testing.B) {
	var once Once
	f := func() error { return nil }
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			once.Do(f)
		}
	})
}
