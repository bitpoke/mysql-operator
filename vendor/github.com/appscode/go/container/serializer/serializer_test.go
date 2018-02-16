package serializer

import "testing"

type fake struct {
	items    int
	val      string
	priority int
}

func (f *fake) Weight() int {
	return f.priority
}

func TestSerializers(t *testing.T) {
	heap := New()
	heap.Add(&fake{val: "hello3", priority: 3})
	heap.Add(&fake{val: "hello1", priority: 1})
	heap.Add(&fake{val: "hello4", priority: 4})
	heap.Add(&fake{val: "hello21", priority: 2})
	heap.Add(&fake{val: "hello22", priority: 2})
	heap.Add(&fake{val: "hello9", priority: 9})
	heap.Add(&fake{val: "hello0", priority: 0})
	heap.Add(&fake{val: "hello100", priority: 100})

	println("================================")
	for it := heap.Iterator(); it.HasNext(); {
		i := it.Now()
		println(i.Weight(), i.(*fake).val, i.(*fake).priority)
	}

	println("================================")
	for it := heap.Iterator(); it.HasNext(); {
		i := it.Now()
		println(i.Weight(), i.(*fake).val, i.(*fake).priority)
	}
}
