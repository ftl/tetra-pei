package com

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemory_Read(t *testing.T) {
	tt := []struct {
		desc     string
		in       string
		bufLen   int
		expected string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"long", "hello", 3, "hel"},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			rw := NewInMemory()
			rw.PrepareRead([]byte(tc.in))
			buf := make([]byte, tc.bufLen)

			n, err := rw.Read(buf)

			assert.NoError(t, err)
			assert.Equal(t, len(tc.expected), n)
			assert.Equal(t, tc.expected, string(buf[0:n]))
		})
	}
}

func TestInMemory_ReadClose(t *testing.T) {
	rw := NewInMemory()

	go func() {
		time.Sleep(100 * time.Nanosecond)
		rw.Close()
	}()

	buf := make([]byte, 10)
	n, err := rw.Read(buf)

	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestInMemory_ReadLater(t *testing.T) {
	rw := NewInMemory()

	go func() {
		time.Sleep(100 * time.Nanosecond)
		rw.PrepareRead([]byte("hello"))
	}()

	buf := make([]byte, 10)
	n, err := rw.Read(buf)

	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf[0:n]))
}

func TestWrite(t *testing.T) {
	rw := NewInMemory()

	n, err := rw.Write([]byte("hello"))

	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(rw.Written()))
	assert.Equal(t, "hello", string(rw.Written()))

	rw.ClearWrite()
	assert.Equal(t, "", string(rw.Written()))
}
