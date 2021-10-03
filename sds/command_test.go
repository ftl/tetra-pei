package sds

import (
	"context"
	"testing"

	"github.com/ftl/tetra-pei/tetra"
	"github.com/stretchr/testify/assert"
)

func TestRequestMaxPDUBits(t *testing.T) {
	tt := []struct {
		desc     string
		response []string
		expected int
		invalid  bool
	}{
		{
			desc:    "empty",
			invalid: true,
		},
		{
			desc: "happy path",
			response: []string{
				"+CMGS: (0-16777214,00000001-10231638316777214,1-255,0-999999999999999999999999),(8-1184)",
				"",
				"OK",
			},
			expected: 1184,
		},
	}
	for _, tc := range tt {
		t.Run(tc.desc, func(t *testing.T) {
			requester := func(_ context.Context, _ string) ([]string, error) {
				return tc.response, nil
			}
			actual, err := RequestMaxMessagePDUBits(context.Background(), tetra.RequesterFunc(requester))
			if tc.invalid {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expected, actual)
			}
		})
	}
}
