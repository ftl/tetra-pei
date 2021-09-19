package com

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadLoop_CloseDevice(t *testing.T) {
	device := NewInMemory()
	lines := readLoop(device)
	device.Close()

	_, valid := <-lines

	assert.False(t, valid)
}

func TestReadLoop_ReadLine(t *testing.T) {
	device := NewInMemory()
	lines := readLoop(device)

	go func() {
		time.Sleep(100 * time.Millisecond)
		device.PrepareRead([]byte("hello\r\n\nworld"))
	}()

	firstLine, valid := <-lines

	assert.True(t, valid)
	assert.Equal(t, "hello", firstLine)

	device.Close()
	lastLine, valid := <-lines

	assert.True(t, valid)
	assert.Equal(t, "world", lastLine)

	_, valid = <-lines

	assert.False(t, valid)
}

func TestCOM_CloseDevice(t *testing.T) {
	device := NewInMemory()
	com := New(device)

	device.Close()

	time.Sleep(1 * time.Millisecond)
	assert.True(t, com.Closed())
}

func TestCOM_ReadAllGarbageOnStartup(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	device.PrepareRead([]byte("CME ERROR: 35\r\n\n\nCME ERROR: 35\r\n\n"))

	New(device)

	time.Sleep(1 * time.Millisecond)
	assert.True(t, device.IsReadEmpty())
}

func TestCOM_Indications(t *testing.T) {
	device := NewInMemory()

	com := New(device)
	actual := make([][]string, 3)
	com.AddIndication("Ind0:", 0, func(lines []string) {
		actual[0] = lines
	})
	com.AddIndication("Ind1:", 1, func(lines []string) {
		actual[1] = lines
	})
	com.AddIndication("Ind2:", 2, func(lines []string) {
		actual[2] = lines
	})
	expected := [][]string{
		{"ind0:message"},
		{"Ind1:header", "message"},
		{"IND2:header", "message1", "message2"},
	}

	device.PrepareRead([]byte("ind0:message\r\nInd1:header\r\nmessage\r\nIND2:header\r\nmessage1\r\nmessage2"))
	device.CloseWhenEmpty(true)
	device.WaitUntilClosed()
	time.Sleep(10 * time.Millisecond)

	assert.Equal(t, fmt.Sprintf("%v", expected), fmt.Sprintf("%v", actual))
}

func TestCOM_SimpleCommand(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		device.PrepareRead([]byte("OK\r\n"))
	}()
	response, err := com.AT(context.Background(), "AT")
	assert.NoError(t, err)
	assert.Empty(t, response)
}

func TestCOM_CommandWithData(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		device.PrepareRead([]byte("message1\r\n\r\nmessage2\r\nOK\r\n"))
	}()
	expected := []string{"message1", "message2"}
	actual, err := com.AT(context.Background(), "AT")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestCOM_CancelCommand(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	ctx, cancel := context.WithCancel(context.Background())
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()
	response, err := com.AT(ctx, "AT")
	assert.Error(t, err)
	assert.Empty(t, response)
}

func TestCOM_CommandWithError(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		device.PrepareRead([]byte("first line\r\nError at last\r\n"))
	}()
	response, err := com.AT(context.Background(), "AT")
	assert.Error(t, err)
	assert.Empty(t, response)
}

func TestCOM_CommandWithCMEError(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		device.PrepareRead([]byte("first line\r\n+CME Error: 35\r\n"))
	}()
	response, err := com.AT(context.Background(), "AT")
	assert.Error(t, err)
	assert.Empty(t, response)
}

func TestCOM_CommandWithCMSError(t *testing.T) {
	device := NewInMemory()
	defer device.Close()
	com := New(device)
	go func() {
		device.WaitUntilWritten()
		time.Sleep(10 * time.Millisecond)
		device.PrepareRead([]byte("first line\r\n+CMS Error: 35\r\n"))
	}()
	response, err := com.AT(context.Background(), "AT")
	assert.Error(t, err)
	assert.Empty(t, response)
}
