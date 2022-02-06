// Copyright (C) 2021 Michael J. Fromberger. All Rights Reserved.

package jrpc2

type queue struct {
	front, back *entry
	free        *entry
	nelts       int
}

func newQueue() *queue {
	sentinel := new(entry)
	return &queue{front: sentinel, back: sentinel}
}

func (q *queue) isEmpty() bool { return q.front.link == nil }
func (q *queue) size() int     { return q.nelts }
func (q *queue) reset()        { q.front.link = nil; q.back = q.front; q.nelts = 0 }

func (q *queue) alloc(data jmessages) *entry {
	if q.free == nil {
		return &entry{data: data}
	}
	out := q.free
	q.free = out.link
	out.data = data
	out.link = nil
	return out
}

func (q *queue) release(e *entry) {
	e.link, q.free = q.free, e
	e.data = nil
}

func (q *queue) each(f func(jmessages)) {
	for cur := q.front.link; cur != nil; cur = cur.link {
		f(cur.data)
	}
}

func (q *queue) push(m jmessages) {
	e := q.alloc(m)
	q.back.link = e
	q.back = e
	q.nelts++
}

func (q *queue) pop() jmessages {
	out := q.front.link
	q.front.link = out.link
	if out == q.back {
		q.back = q.front
	}
	q.nelts--
	data := out.data
	q.release(out)
	return data
}

type entry struct {
	data jmessages
	link *entry
}
