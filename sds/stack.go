package sds

import (
	"fmt"
	"time"

	"github.com/ftl/tetra-pei/tetra"
)

type Message struct {
	ID          int
	Source      tetra.Identity
	Destination tetra.Identity
	Timestamp   time.Time
	parts       []part
}

func NewMessage(id int, source tetra.Identity, destination tetra.Identity, timestamp time.Time, parts int) Message {
	return Message{
		ID:          id,
		Source:      source,
		Destination: destination,
		Timestamp:   timestamp,
		parts:       make([]part, parts),
	}
}

func (m Message) Complete() bool {
	for _, part := range m.parts {
		if !part.Valid {
			return false
		}
	}
	return true
}

func (m Message) Text() string {
	var result string
	for _, part := range m.parts {
		if part.Valid {
			result += part.Text
		} else if result != "" {
			result += "..."
		}
	}
	return result
}

func (m Message) String() string {
	return fmt.Sprintf("Message 0x%x from %s to %s at %s:\n%s",
		m.ID, m.Source, m.Destination, m.Timestamp.Format(time.RFC3339), m.Text())
}

func (m *Message) SetPart(i int, text string) {
	i -= 1
	if i < 0 || i >= len(m.parts) {
		return
	}

	m.parts[i].Text = text
	m.parts[i].Valid = true
}

type part struct {
	Valid bool
	Text  string
}

type MessageCallback func(Message)

type StatusMessage struct {
	Source      tetra.Identity
	Destination tetra.Identity
	Value       Status
}

func (s StatusMessage) String() string {
	return fmt.Sprintf("Status 0x%x from %s to %s", s.Value, s.Source, s.Destination)
}

type StatusCallback func(StatusMessage)

type ResponseCallback func([]string) error

type Stack struct {
	messageCallback  MessageCallback
	statusCallback   StatusCallback
	responseCallback ResponseCallback
	pendingMessages  map[int]Message
}

func NewStack() *Stack {
	return &Stack{
		pendingMessages: make(map[int]Message),
	}
}

func (s *Stack) WithMessageCallback(callback MessageCallback) *Stack {
	s.messageCallback = callback
	return s
}

func (s *Stack) WithStatusCallback(callback StatusCallback) *Stack {
	s.statusCallback = callback
	return s
}

func (s *Stack) WithResponseCallback(callback ResponseCallback) *Stack {
	s.responseCallback = callback
	return s
}

func (s *Stack) Put(part IncomingMessage) error {
	switch payload := part.Payload.(type) {
	case Status:
		// log.Print("incoming status")
		if s.statusCallback == nil {
			return nil
		}
		s.statusCallback(StatusMessage{
			Source:      part.Header.Source,
			Destination: part.Header.Destination,
			Value:       payload,
		})
	case SimpleTextMessage:
		// log.Print("incoming simple text message")
		if s.messageCallback == nil {
			return nil
		}
		message := NewMessage(
			0,
			part.Header.Source,
			part.Header.Destination,
			time.Time{},
			1)
		message.SetPart(1, payload.Text)
		s.messageCallback(message)
	case SDSTransfer:
		// log.Print("incoming SDS-TRANSFER")
		return s.putSDSTransfer(part.Header, payload)
	default:
		return fmt.Errorf("unexpected message type %T", payload)
	}

	return nil
}

func (s *Stack) putSDSTransfer(header Header, sdsTransfer SDSTransfer) error {
	var messageID int
	var message Message
	var ok bool

	switch sdu := sdsTransfer.UserData.(type) {
	case TextSDU:
		messageID = int(sdsTransfer.MessageReference)
		message = NewMessage(
			messageID,
			header.Source,
			header.Destination,
			sdu.Timestamp,
			1,
		)
		message.SetPart(1, sdu.Text)

		if s.responseCallback != nil && sdsTransfer.ReceivedReportRequested() {
			ackRequired := false // TODO should be configurable or a parameter
			sdsReport := NewSDSReport(sdsTransfer, ackRequired, ReceiptAckByDestination)

			s.responseCallback([]string{
				SwitchToSDSTL,
				SendMessage(header.Source, sdsReport),
			})
		}
	case ConcatenatedTextSDU:
		messageID = int(sdu.UserDataHeader.MessageReference)
		message, ok = s.pendingMessages[messageID]
		if !ok {
			message = NewMessage(
				messageID,
				header.Source,
				header.Destination,
				sdu.Timestamp,
				int(sdu.UserDataHeader.TotalNumber),
			)
		} else if message.Source != header.Source ||
			message.Destination != header.Destination ||
			len(message.parts) != int(sdu.UserDataHeader.TotalNumber) {
			return fmt.Errorf("part does not match message 0x%x: %s != %s | %s != %s | %d != %d", message.ID, message.Source, header.Source, message.Destination, header.Destination, len(message.parts), int(sdu.UserDataHeader.TotalNumber))
		}
		message.SetPart(int(sdu.UserDataHeader.SequenceNumber), sdu.Text)
	case ConcatenatedSDSMessageSDU:
		now := time.Now()
		messageID = int(sdu.ConcatenationReference)
		message, ok = s.pendingMessages[messageID]
		if !ok {
			message = NewMessage(
				messageID,
				header.Source,
				header.Destination,
				now,
				int(sdu.TotalNumber),
			)
		} else if message.Source != header.Source ||
			message.Destination != header.Destination ||
			len(message.parts) != int(sdu.TotalNumber) {
			return fmt.Errorf("part does not match message 0x%x: %s != %s | %s != %s | %d != %d", message.ID, message.Source, header.Source, message.Destination, header.Destination, len(message.parts), int(sdu.TotalNumber))
		}
		message.SetPart(int(sdu.SequenceNumber), string(sdu.PayloadData))
	default:
		return fmt.Errorf("unexpected SDS-TRANSFER SDU: %T", sdu)
	}

	if message.Complete() && s.messageCallback != nil {
		s.messageCallback(message)
		delete(s.pendingMessages, message.ID)
	} else {
		s.pendingMessages[message.ID] = message
	}

	return nil
}
