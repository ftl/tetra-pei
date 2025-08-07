package sds

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStack_Put_Status(t *testing.T) {
	value := IncomingMessage{
		Header:  Header{AIService: StatusService, Source: "1234567", Destination: "2345678", PDUBits: 16},
		Payload: Status4,
	}
	expected := StatusMessage{
		Source:      "1234567",
		Destination: "2345678",
		Value:       Status4,
	}

	var status StatusMessage
	statusReceived := false
	stack := NewStack().WithStatusCallback(func(s StatusMessage) {
		status = s
		statusReceived = true
	})

	err := stack.Put(value)

	require.NoError(t, err)
	assert.True(t, statusReceived)
	assert.Equal(t, expected, status)
}

func TestStack_Put_SimpleTextMessage(t *testing.T) {
	value := IncomingMessage{
		Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 224},
		Payload: SimpleTextMessage{
			protocol: SimpleTextMessaging,
			Encoding: ISO8859_1,
			Text:     "testmessage",
		},
	}
	expected := Message{
		Source:      "1234567",
		Destination: "2345678",
		parts: []part{
			{Valid: true, Text: "testmessage"},
		},
	}

	var message Message
	messageReceived := false
	stack := NewStack().WithMessageCallback(func(m Message) {
		message = m
		messageReceived = true
	})

	err := stack.Put(value)

	require.NoError(t, err)
	assert.True(t, messageReceived)
	assert.Equal(t, expected, message)
}

func TestStack_Put_TextMessage(t *testing.T) {
	value := IncomingMessage{
		Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 120},
		Payload: SDSTransfer{
			protocol:         TextMessaging,
			MessageReference: 0xC9,
			UserData: TextSDU{
				TextHeader: TextHeader{
					Encoding:  ISO8859_1,
					Timestamp: time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
				},
				Text: "testmessage",
			},
		},
	}
	expected := Message{
		ID:          0xC9,
		Source:      "1234567",
		Destination: "2345678",
		Timestamp:   time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
		parts: []part{
			{Valid: true, Text: "testmessage"},
		},
	}

	var message Message
	messageReceived := false
	stack := NewStack().WithMessageCallback(func(m Message) {
		message = m
		messageReceived = true
	})

	err := stack.Put(value)

	require.NoError(t, err)
	assert.True(t, messageReceived)
	assert.Equal(t, expected, message)
}

func TestStack_Put_SinglePartConcatenatedMessage(t *testing.T) {
	value := IncomingMessage{
		Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 192},
		Payload: SDSTransfer{
			protocol:         UserDataHeaderMessaging,
			MessageReference: 0xC9,
			UserData: ConcatenatedTextSDU{
				TextSDU: TextSDU{
					TextHeader: TextHeader{
						Encoding:  ISO8859_1,
						Timestamp: time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
					},
					Text: "testmessage",
				},
				UserDataHeader: ConcatenatedTextUDH{
					HeaderLength:     5,
					ElementID:        0,
					ElementLength:    3,
					MessageReference: 0xC9,
					TotalNumber:      1,
					SequenceNumber:   1,
				},
			},
		},
	}
	expected := Message{
		ID:          0xC9,
		Source:      "1234567",
		Destination: "2345678",
		Timestamp:   time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
		parts: []part{
			{Valid: true, Text: "testmessage"},
		},
	}

	var message Message
	messageReceived := false
	stack := NewStack().WithMessageCallback(func(m Message) {
		message = m
		messageReceived = true
	})

	err := stack.Put(value)

	require.NoError(t, err)
	assert.True(t, messageReceived)
	assert.Equal(t, expected, message)
}

func TestStack_Put_MultiPartConcatenatedMessage(t *testing.T) {
	values := []IncomingMessage{
		{
			Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 200},
			Payload: SDSTransfer{
				protocol:         UserDataHeaderMessaging,
				MessageReference: 0xC9,
				UserData: ConcatenatedTextSDU{
					TextSDU: TextSDU{
						TextHeader: TextHeader{
							Encoding:  ISO8859_1,
							Timestamp: time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
						},
						Text: "testmessage1",
					},
					UserDataHeader: ConcatenatedTextUDH{
						HeaderLength:     5,
						ElementID:        0,
						ElementLength:    3,
						MessageReference: 0xC9,
						TotalNumber:      2,
						SequenceNumber:   1,
					},
				},
			},
		},
		{
			Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 208},
			Payload: SDSTransfer{
				protocol:         UserDataHeaderMessaging,
				MessageReference: 0xCA,
				UserData: ConcatenatedTextSDU{
					TextSDU: TextSDU{
						TextHeader: TextHeader{
							Encoding:  ISO8859_1,
							Timestamp: time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
						},
						Text: "\ntestmessage2",
					},
					UserDataHeader: ConcatenatedTextUDH{
						HeaderLength:     5,
						ElementID:        0,
						ElementLength:    3,
						MessageReference: 0xC9,
						TotalNumber:      2,
						SequenceNumber:   2,
					},
				},
			},
		},
	}
	expected := Message{
		ID:          0xC9,
		Source:      "1234567",
		Destination: "2345678",
		Timestamp:   time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
		parts: []part{
			{Valid: true, Text: "testmessage1"},
			{Valid: true, Text: "\ntestmessage2"},
		},
	}

	var message Message
	messageReceived := false
	stack := NewStack().WithMessageCallback(func(m Message) {
		message = m
		messageReceived = true
	})

	for i, value := range values {
		err := stack.Put(value)
		require.NoErrorf(t, err, "part %d", i)
	}

	assert.True(t, messageReceived)
	assert.Equal(t, expected, message)
}

func TestStack_Put_MultiPartConcatenatedSDSMessage(t *testing.T) {
	values := []IncomingMessage{
		{
			Header: Header{AIService: SDSTLService, Source: "262100101234567", Destination: "262100102345678", PDUBits: 896},
			Payload: SDSTransfer{
				protocol:                        ConcatenatedSDSMessaging,
				ServiceSelectionShortFormReport: true,
				MessageReference:                0xB9,
				UserData: ConcatenatedSDSMessageSDU{
					ConcatenationReference: 3,
					TotalNumber:    2,
					SequenceNumber: 1,
					PayloadPID: Callout,
					PayloadData: []byte{0xc3, 0xd, 0x19, 0x67, 0x11, 0x11, 0x1c, 0x0, 0xb, 0x0, 0xc, 0x0, 0xd, 0x0, 0xe, 0x13, 0xbc, 0x13, 0xbd, 0x13, 0xed, 0x13, 0xf0, 0x13, 0xf1, 0x13, 0xf4, 0x13, 0xf6, 0x14, 0x19, 0x14, 0x23, 0x14, 0x66, 0xff, 0x54, 0x65, 0x73, 0x74, 0xfe, 0xa, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0xa, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0xa, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54},
				},
			},
		},
		{
			Header: Header{AIService: SDSTLService, Source: "262100101234567", Destination: "262100102345678", PDUBits: 208},
			Payload: SDSTransfer{
				protocol:                        ConcatenatedSDSMessaging,
				ServiceSelectionShortFormReport: true,
				MessageReference:                0xBA,
				UserData: ConcatenatedSDSMessageSDU{
					ConcatenationReference: 3,
					TotalNumber:    2,
					SequenceNumber: 2,
					PayloadData: []byte{0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0x54, 0x65, 0x73, 0x74, 0xa, 0x54, 0x65, 0x73},
				},
			},
		},
	}
	expected := Message{
		ID:          0x03,
		Source:      "262100101234567",
		Destination: "262100102345678",
		parts: []part{
			{Valid: true, Text: "\xc3\r\x19g\x11\x11\x1c\x00\v\x00\f\x00\r\x00\x0e\x13\xbc\x13\xbd\x13\xed\x13\xf0\x13\xf1\x13\xf4\x13\xf6\x14\x19\x14#\x14f\xffTest\xfe\nTestTestTestTestTestTestTestTestTest\nTestTestTest\nTestTestTestT"},
			{Valid: true, Text: "estTestTestTest\nTes"},
		},
	}

	var message Message
	messageReceived := false
	stack := NewStack().WithMessageCallback(func(m Message) {
		message = m
		messageReceived = true
	})

	for i, value := range values {
		err := stack.Put(value)
		require.NoErrorf(t, err, "part %d", i)
	}

	// reset runtime-generated timestamp to avoid timestamp mismatch in test
	message.Timestamp = time.Time{}

	assert.True(t, messageReceived)
	assert.Equal(t, expected, message)
}

func TestStack_Put_TextMessage_ReceiptReportRequested(t *testing.T) {
	value := IncomingMessage{
		Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 120},
		Payload: SDSTransfer{
			protocol:              TextMessaging,
			MessageReference:      0xC9,
			DeliveryReportRequest: MessageReceivedReportRequested,
			UserData: TextSDU{
				TextHeader: TextHeader{
					Encoding:  ISO8859_1,
					Timestamp: time.Date(2021, time.April, 11, 10, 15, 0, 0, time.Local),
				},
				Text: "testmessage",
			},
		},
	}
	expected := []string{"AT+CTSDS=12,0,0,0,1", "AT+CMGS=1234567,32\r\n821000C9\x1a"}

	responses := make([]string, 0)
	responseReceived := false
	stack := NewStack().WithResponseCallback(func(s []string) error {
		responses = s
		responseReceived = true
		return nil
	})

	err := stack.Put(value)

	require.NoError(t, err)
	assert.True(t, responseReceived)
	assert.Equal(t, expected, responses)
}
