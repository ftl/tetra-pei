package ctrl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGPSPositionResponse(t *testing.T) {
	value := "+GPSPOS: 12:34:56,N: 49_01.2345,E: 010_12.3456,5"
	expectedParts := []string{value, "12", "34", "56", "N", "49", "01.2345", "E", "010", "12.3456", "5"}
	actualParts := gpsPositionResponse.FindStringSubmatch(value)

	assert.Equal(t, expectedParts, actualParts)
}

func TestDegreesMinutesToDecimalDegrees(t *testing.T) {
	tt := []struct {
		direction string
		degrees   float64
		minutes   float64
		expected  float64
	}{
		{"N", 49, 1.2345, 49.020575},
		{"S", 49, 1.2345, -49.020575},
		{"W", 49, 1.2345, -49.020575},
		{"E", 49, 1.2345, 49.020575},
	}
	for _, tc := range tt {
		t.Run(tc.direction, func(t *testing.T) {
			actual := degreesMinutesToDecimalDegrees(tc.direction, tc.degrees, tc.minutes)
			assert.Equal(t, tc.expected, actual)

		})
	}
}

func TestTalkgroupRangeResponse(t *testing.T) {
	tt := []struct {
		response string
		kind     string
		min      string
		max      string
	}{
		{
			response: "+CNUMS: (0),(1-2000),(1-2000)",
			kind:     "S",
			min:      "1",
			max:      "2000",
		},
		{
			response: "+CNUMD: (0,1,3),(1-10000),(1-10000)",
			kind:     "D",
			min:      "1",
			max:      "10000",
		},
	}
	for _, tc := range tt {
		t.Run(tc.response, func(t *testing.T) {
			parts := talkgroupRangeResponse.FindStringSubmatch(tc.response)
			assert.Equal(t, 6, len(parts))
			assert.Equal(t, tc.kind, parts[1])
			assert.Equal(t, tc.min, parts[2])
			assert.Equal(t, tc.max, parts[5])
		})
	}
}

func TestParseTalkgroupInfo(t *testing.T) {
	tt := []struct {
		line string
		gtsi string
		name string
	}{
		{
			line: "+CNUMD: 1,123456712341234,Test Group",
			gtsi: "123456712341234",
			name: "Test Group",
		},
		{
			line: "+CNUMS: 1,123456712341234,Test Group",
			gtsi: "123456712341234",
			name: "Test Group",
		},
		{
			line: "1,123456712341234,Test Group",
			gtsi: "123456712341234",
			name: "Test Group",
		},
	}
	for _, tc := range tt {
		t.Run(tc.line, func(t *testing.T) {
			info, err := parseTalkgroupInfo(tc.line)
			assert.NoError(t, err)
			assert.Equal(t, tc.gtsi, info.GTSI)
			assert.Equal(t, tc.name, info.Name)
		})
	}
}
