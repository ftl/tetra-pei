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
