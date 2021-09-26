package com

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	readBufferSize        = 1024
	atSendingQueueTimeout = 500 * time.Millisecond
)

// NewWithTrace creates a new COM instance that traces all communications to a second writer.
func NewWithTrace(device io.ReadWriter, tracer io.Writer) *COM {
	result := New(device)
	result.tracer = tracer
	return result
}

// New creates a new COM instance using the given io.ReadWriter to communicate with the radio's PEI.
func New(device io.ReadWriter) *COM {
	lines := readLoop(device)
	commands := make(chan command)
	result := &COM{
		commands:    commands,
		closed:      make(chan struct{}),
		indications: make(map[string]indicationConfig),
	}

	go func() {
		result.trace("****\n* SESSION START\n****\n")
		defer result.trace("****\n* SESSION END\n****\n")
		defer close(result.closed)

		var commandCancelled <-chan struct{}
		var activeCommand *command
		var activeIndication *indication
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()

		for {
			select {
			case line, valid := <-lines:
				if !valid {
					return
				}
				result.tracef("rx:  %s\nhex: %X\n--\n", line, line)

				switch {
				case activeIndication != nil:
					activeIndication.AddLine(line)
					if activeIndication.Complete() {
						activeIndication = nil
					}
				case activeCommand != nil:
					activeIndication = result.newIndication(line)
					if activeIndication != nil {
						break
					}
					activeCommand.AddLine(line)
					if activeCommand.Complete() {
						commandCancelled = nil
						activeCommand = nil
					}
				default:
					activeIndication = result.newIndication(line)
				}
			case <-commandCancelled:
				commandCancelled = nil
				activeCommand = nil
			case <-tick.C:
			}
			if activeCommand == nil {
				select {
				case cmd := <-commands:
					if len(cmd.request) == 0 {
						break
					}

					txbytes := make([]byte, 0, len(cmd.request)+2)
					txbytes = append(txbytes, []byte(cmd.request)...)
					lastbyte := txbytes[len(txbytes)-1]
					if (lastbyte != 0x1a) && (lastbyte != 0x1b) {
						txbytes = append(txbytes, 0x0d, 0x0a)
					}
					result.tracef("tx:  %s\nhex: %X\n--\n", txbytes, txbytes)
					device.Write(txbytes)
					commandCancelled = cmd.cancelled
					activeCommand = &cmd
				default:
				}
			}
		}
	}()

	return result
}

// COM allows to communicate with a radio's PEI using AT commands.
type COM struct {
	commands chan<- command
	closed   chan struct{}
	tracer   io.Writer

	indications map[string]indicationConfig
}

func readLoop(r io.Reader) <-chan string {
	lines := make(chan string, 1)
	go func() {
		buf := make([]byte, readBufferSize)
		currentLine := make([]byte, 0, readBufferSize)
		for {
			n, err := r.Read(buf)
			if err == io.EOF {
				if len(currentLine) > 0 {
					lines <- string(currentLine)
				}
				close(lines)
				return
			} else if err != nil {
				if len(currentLine) > 0 {
					lines <- string(currentLine)
				}
				close(lines)
				return
			}

			for _, b := range buf[0:n] {
				switch {
				case b == '\n':
					if len(currentLine) == 0 {
						continue
					}
					lines <- string(currentLine)
					currentLine = currentLine[:0]
				case b < ' ':
					continue
				default:
					currentLine = append(currentLine, b)
				}
			}
		}
	}()
	return lines
}

func (c *COM) Closed() bool {
	select {
	case <-c.closed:
		return true
	default:
		return false
	}
}

func (c *COM) AddIndication(prefix string, trailingLines int, handler func(lines []string)) error {
	config := indicationConfig{
		prefix:        strings.ToUpper(prefix),
		trailingLines: trailingLines,
		handler:       handler,
	}
	c.indications[config.prefix] = config
	return nil
}

func (c *COM) newIndication(line string) *indication {
	for _, config := range c.indications {
		result := config.NewIfMatches(line)
		if result != nil {
			return result
		}
	}
	return nil
}

func (c *COM) ClearSyntaxErrors(ctx context.Context) error {
	for true {
		_, err := c.AT(ctx, "AT")
		if err == nil {
			return nil
		}
		if err.Error() == "+CME ERROR: 35" {
			time.Sleep(200)
		} else {
			return err
		}
	}
	return nil
}

func (c *COM) AT(ctx context.Context, request string) ([]string, error) {
	cmd := command{
		request:   request,
		response:  make(chan []string, 1),
		err:       make(chan error, 1),
		cancelled: ctx.Done(),
		completed: make(chan struct{}),
	}

	select {
	case c.commands <- cmd:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(atSendingQueueTimeout):
		return nil, fmt.Errorf("AT sending queue timeout")
	}

	select {
	case response := <-cmd.response:
		return response, nil
	case err := <-cmd.err:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *COM) ATs(ctx context.Context, requests ...string) error {
	for _, request := range requests {
		_, err := c.AT(ctx, request)
		if err != nil {
			return fmt.Errorf("%s failed: %w", request, err)
		}
	}
	return nil
}

func (c *COM) trace(args ...interface{}) {
	if c.tracer == nil {
		return
	}
	fmt.Fprint(c.tracer, args...)
}

func (c *COM) tracef(format string, args ...interface{}) {
	if c.tracer == nil {
		return
	}
	fmt.Fprintf(c.tracer, format, args...)
}

type indicationConfig struct {
	prefix        string
	trailingLines int
	handler       func(lines []string)
}

func (c *indicationConfig) NewIfMatches(line string) *indication {
	if !strings.HasPrefix(strings.ToUpper(line), c.prefix) {
		return nil
	}
	result := &indication{
		config: *c,
		lines:  []string{line},
	}
	if result.Complete() {
		c.handler([]string{line})
		return nil
	}

	return result
}

type indication struct {
	config indicationConfig
	lines  []string
}

func (ind *indication) AddLine(line string) {
	if ind.Complete() {
		return
	}

	ind.lines = append(ind.lines, line)
	if ind.Complete() {
		go func() {
			ind.config.handler(ind.lines)
		}()
	}
}

func (ind *indication) Complete() bool {
	return len(ind.lines) >= ind.config.trailingLines+1
}

type command struct {
	lines     []string
	request   string
	response  chan []string
	err       chan error
	cancelled <-chan struct{}
	completed chan struct{}
}

func (c *command) AddLine(line string) {
	select {
	case <-c.cancelled:
		return
	case <-c.completed:
		return
	default:
	}

	saniLine := strings.TrimSpace(strings.ToUpper(line))
	switch {
	case saniLine == "OK":
		c.response <- c.lines
		close(c.completed)
	case strings.HasPrefix(saniLine, "ERROR"):
		c.err <- fmt.Errorf("%s", line)
		close(c.completed)
	case strings.HasPrefix(saniLine, "+CME ERROR:"):
		c.err <- fmt.Errorf("%s", line)
		close(c.completed)
	case strings.HasPrefix(saniLine, "+CMS ERROR"):
		c.err <- fmt.Errorf("%s", line)
		close(c.completed)
	default:
		c.lines = append(c.lines, line)
	}
}

func (c *command) Complete() bool {
	select {
	case <-c.cancelled:
		return true
	case <-c.completed:
		return true
	default:
		return false
	}
}
