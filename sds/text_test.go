package sds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitsToTextBytes(t *testing.T) {
	tt := []struct {
		desc          string
		encoding      TextEncoding
		bits          int
		expectedBytes int
	}{
		{
			desc:          "7bit, 0",
			encoding:      Packed7Bit,
			bits:          0,
			expectedBytes: 0,
		},
		{
			desc:          "8bit, 0",
			encoding:      ISO8859_1,
			bits:          0,
			expectedBytes: 0,
		},
		{
			desc:          "7bit, 1",
			encoding:      Packed7Bit,
			bits:          1,
			expectedBytes: 0,
		},
		{
			desc:          "8bit, 1",
			encoding:      ISO8859_1,
			bits:          1,
			expectedBytes: 0,
		},
		{
			desc:          "7bit, 8",
			encoding:      Packed7Bit,
			bits:          8,
			expectedBytes: 1,
		},
		{
			desc:          "7bit, 14",
			encoding:      Packed7Bit,
			bits:          14,
			expectedBytes: 2,
		},
		{
			desc:          "8bit, 14",
			encoding:      ISO8859_1,
			bits:          14,
			expectedBytes: 1,
		},
		{
			desc:          "7bit, 56",
			encoding:      Packed7Bit,
			bits:          56,
			expectedBytes: 8,
		},
		{
			desc:          "8bit, 56",
			encoding:      ISO8859_1,
			bits:          56,
			expectedBytes: 7,
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actualBytes := BitsToTextBytes(tc.encoding, tc.bits)
			assert.Equal(t, tc.expectedBytes, actualBytes)
		})
	}
}

func TestSplitToMaxBits(t *testing.T) {
	tt := []struct {
		encoding      TextEncoding
		maxPDUBits    int
		text          string
		expectedParts []string
	}{
		{
			encoding:      Packed7Bit,
			maxPDUBits:    56,
			text:          "7-bit, 056",
			expectedParts: []string{"7-bit, 0", "56"},
		},
		{
			encoding:      ISO8859_1,
			maxPDUBits:    56,
			text:          "8-bit, 056",
			expectedParts: []string{"8-bit, ", "056"},
		},
		{
			encoding:      Packed7Bit,
			maxPDUBits:    128,
			text:          "7-bit, 128",
			expectedParts: []string{"7-bit, 128"},
		},
		{
			encoding:      ISO8859_1,
			maxPDUBits:    128,
			text:          "8-bit, 128",
			expectedParts: []string{"8-bit, 128"},
		},
	}
	for _, tc := range tt {
		t.Run(tc.text, func(t *testing.T) {
			actualParts := SplitToMaxBits(tc.encoding, tc.maxPDUBits, tc.text)
			assert.Equal(t, tc.expectedParts, actualParts)
		})
	}
}

func TestSplitLeadingOPTA(t *testing.T) {
	tt := []struct {
		desc         string
		value        string
		expectedOPTA string
		expectedTail string
	}{
		{
			desc:         "no OPTA",
			value:        "testmessage",
			expectedOPTA: "",
			expectedTail: "testmessage",
		},
		{
			desc:         "only OPTA",
			value:        "ABCD FG#1234567890123456",
			expectedOPTA: "ABCD FG#1234567890123456",
			expectedTail: "",
		},
		{
			desc:         "OPTA and tail",
			value:        "ABCD FG#1234567890123456testmessage",
			expectedOPTA: "ABCD FG#1234567890123456",
			expectedTail: "testmessage",
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actualOPTA, actualTail := SplitLeadingOPTA(tc.value)
			assert.Equal(t, tc.expectedOPTA, actualOPTA)
			assert.Equal(t, tc.expectedTail, actualTail)
		})
	}
}

func TestSplitTrailingITSI(t *testing.T) {
	tt := []struct {
		desc         string
		value        string
		expectedHead string
		expectedITSI string
	}{
		{
			desc:         "no ITSI",
			value:        "testmessage",
			expectedHead: "testmessage",
			expectedITSI: "",
		},
		{
			desc:         "cr cr",
			value:        "testmessage\r\r1234567890123456",
			expectedHead: "testmessage",
			expectedITSI: "1234567890123456",
		},
		{
			desc:         "ctrl-z nul",
			value:        "testmessage\x1a\x001234567890123456",
			expectedHead: "testmessage",
			expectedITSI: "1234567890123456",
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			actualHead, actualITSI := SplitTrailingITSI(tc.value)
			assert.Equal(t, tc.expectedHead, actualHead)
			assert.Equal(t, tc.expectedITSI, actualITSI)
		})
	}
}
