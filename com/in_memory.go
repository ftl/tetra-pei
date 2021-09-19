package com

import (
	"io"
	"sync"
	"time"
)

func NewInMemory() *InMemory {
	return &InMemory{
		readBuffer:  []byte{},
		writeBuffer: []byte{},
		readLock:    new(sync.RWMutex),
		writeLock:   new(sync.RWMutex),
		writeSignal: make(chan bool),
		closed:      make(chan struct{}),
	}
}

type InMemory struct {
	readBuffer     []byte
	writeBuffer    []byte
	readLock       *sync.RWMutex
	writeLock      *sync.RWMutex
	writeSignal    chan bool
	closed         chan struct{}
	closeWhenEmpty bool
}

func (rw *InMemory) Close() error {
	select {
	case <-rw.closed:
	default:
		close(rw.closed)
	}
	return nil
}

func (rw *InMemory) WaitUntilClosed() {
	<-rw.closed
}

func (rw *InMemory) Read(p []byte) (int, error) {
	for {
		rw.readLock.RLock()
		if len(rw.readBuffer) > 0 {
			rw.readLock.RUnlock()
			break
		}
		rw.readLock.RUnlock()
		select {
		case <-rw.closed:
			return 0, io.EOF
		case <-time.After(10 * time.Millisecond):
			continue
		}
	}

	select {
	case <-rw.closed:
		return 0, io.EOF
	default:
	}

	rw.readLock.Lock()
	defer rw.readLock.Unlock()
	n := len(p)
	if n > len(rw.readBuffer) {
		n = len(rw.readBuffer)
	}
	copy(p, rw.readBuffer[0:n])
	if n < len(rw.readBuffer) {
		rw.readBuffer = rw.readBuffer[n:]
	} else {
		rw.readBuffer = []byte{}
	}
	if rw.closeWhenEmpty && len(rw.readBuffer) == 0 {
		close(rw.closed)
	}
	return n, nil
}

func (rw *InMemory) PrepareRead(p []byte) {
	rw.readLock.Lock()
	defer rw.readLock.Unlock()

	rw.readBuffer = append(rw.readBuffer, p...)
}

func (rw *InMemory) ClearRead() {
	rw.readLock.Lock()
	defer rw.readLock.Unlock()

	rw.readBuffer = []byte{}

	if rw.closeWhenEmpty && len(rw.readBuffer) == 0 {
		close(rw.closed)
	}
}

func (rw *InMemory) IsReadEmpty() bool {
	rw.readLock.RLock()
	defer rw.readLock.RUnlock()

	return len(rw.readBuffer) == 0
}

func (rw *InMemory) CloseWhenEmpty(value bool) {
	rw.readLock.Lock()
	defer rw.readLock.Unlock()

	rw.closeWhenEmpty = value
}

func (rw *InMemory) Write(p []byte) (int, error) {
	rw.writeLock.Lock()
	defer rw.writeLock.Unlock()

	rw.writeBuffer = append(rw.writeBuffer, p...)
	select {
	case rw.writeSignal <- true:
	default:
	}
	return len(p), nil
}

func (rw *InMemory) Written() []byte {
	rw.writeLock.RLock()
	defer rw.writeLock.RUnlock()

	return rw.writeBuffer
}

func (rw *InMemory) ClearWrite() {
	rw.writeLock.Lock()
	defer rw.writeLock.Unlock()

	rw.writeBuffer = []byte{}
}

func (rw *InMemory) WaitUntilWritten() {
	<-rw.writeSignal
}
