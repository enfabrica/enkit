package scheduler

type eventHeap []*event

func (h eventHeap) Len() int {
	return len(h)
}
func (h eventHeap) Less(i, j int) bool {
	return h[i].when.Before(h[j].when)
}
func (h eventHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}
func (h *eventHeap) Push(x interface{}) {
	*h = append(*h, x.(*event))
}
func (h *eventHeap) Pop() interface{} {
	last := (*h)[len(*h)-1]
	(*h) = (*h)[:len(*h)-1]
	return last
}
