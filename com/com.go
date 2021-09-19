package com

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"
)

const (
	readBufferSize = 1024
)

func New(device io.ReadWriter) *COM {
	lines := readLoop(device)
	commands := make(chan command)
	result := &COM{
		commands:    commands,
		closed:      make(chan struct{}),
		indications: make(map[string]indicationConfig),
	}

	go func() {
		log.Print("entering COM loop")
		defer log.Print("exiting COM loop")
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
					log.Print("lines channel closed")
					return
				}
				// log.Printf("rx: %s", line)

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
					// log.Printf("tx: %v", txbytes)
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

type COM struct {
	commands chan<- command
	closed   chan struct{}

	indications map[string]indicationConfig
}

func readLoop(r io.Reader) <-chan string {
	lines := make(chan string, 1)
	go func() {
		log.Print("entering read loop")
		defer log.Print("exiting read loop")

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
				log.Printf("read error: %v", err)
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
			log.Printf("%s is an indication", config.prefix)
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
			log.Printf(".")
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
	case <-time.After(500 * time.Millisecond):
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
	log.Printf("%s add line %s, actual: %d, expected %d", ind.config.prefix, line, len(ind.lines), ind.config.trailingLines)
	if ind.Complete() {
		return
	}

	ind.lines = append(ind.lines, line)
	if ind.Complete() {
		log.Printf("%s is complete, actual: %d, expected %d\n%v", ind.config.prefix, len(ind.lines), ind.config.trailingLines, strings.Join(ind.lines, "\n"))
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
