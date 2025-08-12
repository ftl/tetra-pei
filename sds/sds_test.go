package sds

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseMessage(t *testing.T) {
	expectedTimestamp := time.Date(time.Now().Year(), time.April, 11, 10, 15, 0, 0, time.Local)
	tt := []struct {
		desc      string
		header    string
		pdu       string
		expected  IncomingMessage
		immediate bool
		invalid   bool
	}{
		{
			desc:    "empty string",
			invalid: true,
		},
		{
			desc:   "status",
			header: "+CTSDSR: 13,1234567,0,2345678,0,16",
			pdu:    "8004",
			expected: IncomingMessage{
				Header:  Header{AIService: StatusService, Source: "1234567", Destination: "2345678", PDUBits: 16},
				Payload: Status2,
			},
		},
		{
			desc:   "simple text message",
			header: "+CTSDSR: 12,1234567,0,2345678,0,104",
			pdu:    "0201746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 104},
				Payload: SimpleTextMessage{
					protocol: SimpleTextMessaging,
					Encoding: ISO8859_1,
					Text:     "testmessage",
				},
			},
		},
		{
			desc:   "simple text message without text",
			header: "+CTSDSR: 12,1234567,0,2345678,0,16",
			pdu:    "0201",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 16},
				Payload: SimpleTextMessage{
					protocol: SimpleTextMessaging,
					Encoding: ISO8859_1,
					Text:     "",
				},
			},
		},
		{
			desc:   "immediate simple text message",
			header: "+CTSDSR: 12,1234567,0,2345678,0,104",
			pdu:    "0901746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 104},
				Payload: SimpleTextMessage{
					protocol: SimpleImmediateTextMessaging,
					Encoding: ISO8859_1,
					Text:     "testmessage",
				},
			},
			immediate: true,
		},
		{
			desc:   "text message, no report, no store/forward, no timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,120",
			pdu:    "82029C01746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 120},
				Payload: SDSTransfer{
					Protocol:         TextMessaging,
					MessageReference: 0x9C,
					UserData: TextSDU{
						TextHeader: TextHeader{
							Encoding: ISO8859_1,
						},
						Text: "testmessage",
					},
				},
			},
		},
		{
			desc:   "immediate text message, no report, no store/forward, no timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,120",
			pdu:    "89029C01746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 120},
				Payload: SDSTransfer{
					Protocol:         ImmediateTextMessaging,
					MessageReference: 0x9C,
					UserData: TextSDU{
						TextHeader: TextHeader{
							Encoding: ISO8859_1,
						},
						Text: "testmessage",
					},
				},
			},
			immediate: true,
		},
		{
			desc:   "text message, no report, store/forward to SSI, no timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,152",
			pdu:    "82039C5101020301746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 152},
				Payload: SDSTransfer{
					Protocol:         TextMessaging,
					MessageReference: 0x9C,
					StoreForwardControl: StoreForwardControl{
						Valid:              true,
						ValidityPeriod:     ValidityPeriod(5 * time.Minute),
						ForwardAddressType: ForwardToSSI,
						ForwardAddressSSI:  ForwardAddressSSI{1, 2, 3},
					},
					UserData: TextSDU{
						TextHeader: TextHeader{
							Encoding: ISO8859_1,
						},
						Text: "testmessage",
					},
				},
			},
		},
		{
			desc:   "text message, no report, no store/forward, with timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,144",
			pdu:    "82029C81045A8F746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 144},
				Payload: SDSTransfer{
					Protocol:         TextMessaging,
					MessageReference: 0x9C,
					UserData: TextSDU{
						TextHeader: TextHeader{
							Encoding:  ISO8859_1,
							Timestamp: expectedTimestamp,
						},
						Text: "testmessage",
					},
				},
			},
		},
		{
			desc:   "concatenated text message part 1 of 2, no report, no store/forward, with timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,192",
			pdu:    "8A02C981045A8F050003C90201746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 192},
				Payload: SDSTransfer{
					Protocol:         UserDataHeaderMessaging,
					MessageReference: 0xC9,
					UserData: ConcatenatedTextSDU{
						TextSDU: TextSDU{
							TextHeader: TextHeader{
								Encoding:  ISO8859_1,
								Timestamp: expectedTimestamp,
							},
							Text: "testmessage",
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
		},
		{
			desc:   "concatenated text message part 2 of 2, no report, no store/forward, with timestamp",
			header: "+CTSDSR: 12,1234567,0,2345678,0,192",
			pdu:    "8A02CA81045A8F050003C90202746573746D657373616765",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 192},
				Payload: SDSTransfer{
					Protocol:         UserDataHeaderMessaging,
					MessageReference: 0xCA,
					UserData: ConcatenatedTextSDU{
						TextSDU: TextSDU{
							TextHeader: TextHeader{
								Encoding:  ISO8859_1,
								Timestamp: expectedTimestamp,
							},
							Text: "testmessage",
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
		},
		{
			desc:   "SDS-REPORT success, no ack, no store/forward",
			header: "+CTSDSR: 12,1234567,0,2345678,0,32",
			pdu:    "821000C9",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 32},
				Payload: SDSReport{
					protocol:         TextMessaging,
					DeliveryStatus:   ReceiptAckByDestination,
					MessageReference: 0xC9,
				},
			},
		},
		{
			desc:   "SDS-REPORT success, ack required, no store/forward",
			header: "+CTSDSR: 12,1234567,0,2345678,0,32",
			pdu:    "821800CA",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 32},
				Payload: SDSReport{
					protocol:         TextMessaging,
					AckRequired:      true,
					DeliveryStatus:   ReceiptAckByDestination,
					MessageReference: 0xCA,
				},
			},
		},
		{
			desc:   "SDS-ACK success",
			header: "+CTSDSR: 12,1234567,0,2345678,0,32",
			pdu:    "822001C9",
			expected: IncomingMessage{
				Header: Header{AIService: SDSTLService, Source: "1234567", Destination: "2345678", PDUBits: 32},
				Payload: SDSAcknowledge{
					protocol:         TextMessaging,
					DeliveryStatus:   ReceiptReportAck,
					MessageReference: 0xC9,
				},
			},
		},
		{
			desc:   "SDS-SHORT-REPORT success",
			header: "+CTSDSR: 13,1234567,0,2345678,0,16",
			pdu:    "7ACA",
			expected: IncomingMessage{
				Header: Header{AIService: StatusService, Source: "1234567", Destination: "2345678", PDUBits: 16},
				Payload: SDSShortReport{
					ReportType:       MessageReceivedShort,
					MessageReference: 0xCA,
				},
			},
		},
	}
	type immediater interface {
		Immediate() bool
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actual, err := ParseIncomingMessage(tc.header, tc.pdu)
			if tc.invalid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)

				if tc.immediate {
					i, ok := tc.expected.Payload.(immediater)
					assert.True(t, ok)
					assert.True(t, i.Immediate())
				}
			}
		})
	}
}

func TestTimestampRoundtrip(t *testing.T) {
	now := time.Now()
	expected := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local).UTC()

	actual, err := DecodeTimestamp(EncodeTimestampUTC(now))

	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestValidityPeriod_Decode(t *testing.T) {
	tt := []struct {
		value    byte
		expected time.Duration
	}{
		{0, 0},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{6, 1 * time.Minute},
		{7, 2 * time.Minute},
		{10, 5 * time.Minute},
		{11, 10 * time.Minute},
		{12, 20 * time.Minute},
		{16, 1 * time.Hour},
		{17, 2 * time.Hour},
		{22, 12 * time.Hour},
		{23, 18 * time.Hour},
		{25, 2 * 24 * time.Hour},
		{26, 4 * 24 * time.Hour},
		{30, 12 * 24 * time.Hour},
		{31, time.Duration(InfinitelyValid)},
	}
	for _, tc := range tt {
		t.Run(fmt.Sprintf("%d", tc.value), func(t *testing.T) {
			actual := ParseValidityPeriod(tc.value)
			assert.Equal(t, ValidityPeriod(tc.expected), actual)
		})
	}
}

func TestValidityPeriod_Encode(t *testing.T) {
	tt := []struct {
		value    time.Duration
		expected byte
	}{
		{0, 0},
		{1 * time.Millisecond, 1},
		{1 * time.Second, 1},
		{1*time.Second + 1*time.Millisecond, 1},
		{10 * time.Second, 1},
		{10*time.Second + 1*time.Millisecond, 2},
		{20 * time.Second, 2},
		{1 * time.Minute, 6},
		{1*time.Minute + 1*time.Millisecond, 7},
		{2 * time.Minute, 7},
		{2*time.Minute + 1*time.Millisecond, 8},
		{5 * time.Minute, 10},
		{5*time.Minute + 1*time.Millisecond, 11},
		{10 * time.Minute, 11},
		{10*time.Minute + 1*time.Millisecond, 12},
		{1 * time.Hour, 16},
		{1*time.Hour + 1*time.Millisecond, 17},
		{2 * time.Hour, 17},
		{2*time.Hour + 1*time.Millisecond, 18},
		{6 * time.Hour, 21},
		{6*time.Hour + 1*time.Millisecond, 22},
		{7 * time.Hour, 22},
		{12 * time.Hour, 22},
		{12*time.Hour + 1*time.Millisecond, 23},
		{24 * time.Hour, 24},
		{24*time.Hour + 1*time.Millisecond, 25},
		{2 * 24 * time.Hour, 25},
		{2*24*time.Hour + 1*time.Millisecond, 26},
		{3 * 24 * time.Hour, 26},
		{4 * 24 * time.Hour, 26},
		{4*24*time.Hour + 1*time.Millisecond, 27},
		{5 * 24 * time.Hour, 27},
		{6 * 24 * time.Hour, 27},
		{7 * 24 * time.Hour, 28},
		{8 * 24 * time.Hour, 28},
		{9 * 24 * time.Hour, 29},
		{10 * 24 * time.Hour, 29},
		{11 * 24 * time.Hour, 30},
		{12 * 24 * time.Hour, 30},
		{12*24*time.Hour + 1*time.Millisecond, 31},
	}
	for _, tc := range tt {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			actual, _ := ValidityPeriod(tc.value).Encode()
			assert.Equal(t, tc.expected, actual[0])
		})
	}
}

func TestStatusBytes(t *testing.T) {
	assert.Equal(t, []byte{0x80, 0x04}, Status2.Bytes())
}

func TestParseHeader(t *testing.T) {
	tt := []struct {
		desc     string
		value    string
		expected Header
		invalid  bool
	}{
		{
			desc:    "empty string",
			value:   "",
			invalid: true,
		},
		{
			desc:  "valid minimum set",
			value: "+CTSDSR: 12,1234567,16",
			expected: Header{
				AIService:   SDSTLService,
				Destination: "1234567",
				PDUBits:     16,
			},
		},
		{
			desc:  "valid minimum set with identity type",
			value: "+CTSDSR: 12,1234567,0,16",
			expected: Header{
				AIService:   SDSTLService,
				Destination: "1234567",
				PDUBits:     16,
			},
		},
		{
			desc:  "valid with source identity",
			value: "+CTSDSR: 12,1234567,0,2345678,0,16",
			expected: Header{
				AIService:   SDSTLService,
				Source:      "1234567",
				Destination: "2345678",
				PDUBits:     16,
			},
		},
		// field order according to ETSI TS 100 392-5 V2.7.1
		// +CTSDSR: <AI service>, [<calling party identity>], [<calling party identity type>], <called party identity>, <called party identity type>, <length>, [<end to end encryption>]<CR><LF>user data
		{
			desc:  "valid with source identity and end-to-end encryption",
			value: "+CTSDSR: 12,1234567,0,2345678,0,16,1",
			expected: Header{
				AIService:   SDSTLService,
				Source:      "1234567",
				Destination: "2345678",
				PDUBits:     16,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actual, err := ParseHeader(tc.value)
			if tc.invalid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	expectedTimestamp := time.Date(time.Now().Year(), time.April, 11, 8, 15, 0, 0, time.UTC)
	tt := []struct {
		desc          string
		values        []Encoder
		expectedBytes []byte
		expectedBits  int
	}{
		{
			desc: "single entities",
			values: []Encoder{
				SimpleTextMessaging,
				ConsumedReportAck,
			},
			expectedBytes: []byte{0x02, 0x03},
			expectedBits:  16,
		},
		{
			desc: "SDS-REPORT",
			values: []Encoder{
				SDSReport{
					protocol:         TextMessaging,
					AckRequired:      true,
					DeliveryStatus:   ReceiptAckByDestination,
					MessageReference: 0xCA,
				},
			},
			expectedBytes: []byte{0x82, 0x18, 0x00, 0xCA},
			expectedBits:  32,
		},
		{
			desc: "SDS-TRANSFER text message, delivery report requested",
			values: []Encoder{
				SDSTransfer{
					Protocol:              TextMessaging,
					DeliveryReportRequest: MessageReceivedReportRequested,
					MessageReference:      0xC9,
					UserData: TextSDU{
						TextHeader: TextHeader{
							Encoding:  ISO8859_1,
							Timestamp: expectedTimestamp,
						},
						Text: "testmessage",
					},
				},
			},
			expectedBytes: []byte{0x82, 0x06, 0xC9, 0x81, 0x44, 0x5A, 0x0F, 0x74, 0x65, 0x73, 0x74, 0x6D, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65},
			expectedBits:  144,
		},
		{
			desc: "simple text message",
			values: []Encoder{
				SimpleTextMessage{
					protocol: SimpleTextMessaging,
					Encoding: ISO8859_1,
					Text:     "testmessage",
				},
			},
			expectedBytes: []byte{0x02, 0x01, 0x74, 0x65, 0x73, 0x74, 0x6D, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65},
			expectedBits:  104,
		},
		{
			desc: "SDS-TRANSFER concatenated text message with UDH",
			values: []Encoder{
				SDSTransfer{
					Protocol:         UserDataHeaderMessaging,
					MessageReference: 0xC9,
					UserData: ConcatenatedTextSDU{
						TextSDU: TextSDU{
							TextHeader: TextHeader{
								Encoding:  ISO8859_1,
								Timestamp: expectedTimestamp,
							},
							Text: "testmessage",
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
			expectedBytes: []byte{0x8A, 0x02, 0xC9, 0x81, 0x44, 0x5A, 0x0F, 0x05, 0x00, 0x03, 0xC9, 0x02, 0x01, 0x74, 0x65, 0x73, 0x74, 0x6D, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65},
			expectedBits:  192,
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actualBytes := make([]byte, 0)
			actualBits := 0
			for _, value := range tc.values {
				actualBytes, actualBits = value.Encode(actualBytes, actualBits)
			}
			assert.Equal(t, tc.expectedBytes, actualBytes)
			assert.Equal(t, tc.expectedBits, actualBits)
		})
	}
}
