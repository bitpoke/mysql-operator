package serializer

import (
	"container/heap"
	"sync"
)

type serializedQueue struct {
	sync       sync.RWMutex
	count      int
	serializer *serializer
}

func New() *serializedQueue {
	h := &serializer{}
	heap.Init(h)
	return &serializedQueue{
		count:      0,
		serializer: h,
	}
}

func (s *serializedQueue) Add(i Entry) *serializedQueue {
	s.sync.Lock()
	s.count++
	heap.Push(s.serializer, i)
	s.sync.Unlock()
	return s
}

func (s *serializedQueue) Pop() interface{} {
	s.sync.Lock()
	s.count--
	elm := heap.Pop(s.serializer)
	s.sync.Unlock()
	return elm
}

func (s *serializedQueue) Len() int {
	return s.count
}

func (s *serializedQueue) Iterator() *iterator {
	c := &serializer{}
	*c = append(*c, *s.serializer...)
	//heap.Init(c)
	return &iterator{c}
}

type iterator struct {
	old *serializer
}

func (i *iterator) HasNext() bool {
	return i.old.Len() > 0
}

func (i *iterator) Now() Entry {
	return heap.Pop(i.old).(Entry)
}
