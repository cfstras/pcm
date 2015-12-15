package main

import (
	"sync"
)

type Buffer struct {
	pool *sync.Pool
	buf  chan []byte
}

func NewBuffer() *Buffer {
	b := &Buffer{
		buf: make(chan []byte),
		pool: &sync.Pool{
			New: func() interface{} { return make([]byte, 1024) },
		}}

	return b
}

func (b *Buffer) Read(p []byte) (n int, err error) {
	i := 0
	buf := <-b.buf
	i += copy(p[i:], buf)
	b.returnReadBuffer(buf)
	for i < len(p) {
		select {
		case buf := <-b.buf:
			i += copy(p[i:], buf)
			b.returnReadBuffer(buf)
		default:
			return i, nil
		}
	}
	return i, nil
}

func (b *Buffer) returnReadBuffer(buf []byte) {
	if cap(buf) > 256 {
		b.pool.Put(buf)
	}
}

func (b *Buffer) Write(data []byte) (int, error) {
	i := 0
	for i < len(data) {
		buffer := b.pool.Get().([]byte)
		l := copy(buffer, data[i:])
		i += l
		buffer = buffer[:l]
		b.buf <- buffer
	}
	return i, nil
}
