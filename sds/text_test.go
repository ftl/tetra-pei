package sds

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
