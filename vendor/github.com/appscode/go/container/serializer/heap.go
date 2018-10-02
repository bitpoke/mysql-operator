package serializer

type Entry interface {
	Weight() int
}

type serializer []Entry

func (pq serializer) Len() int { return len(pq) }

func (pq serializer) Less(i, j int) bool {
	return pq[i].Weight() > pq[j].Weight()
}

func (pq serializer) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *serializer) Push(x interface{}) {
	item := x.(Entry)
	*pq = append(*pq, item)
}

func (pq *serializer) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}
