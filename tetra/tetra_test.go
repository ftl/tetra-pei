package tetra

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHexBinaryRoundtrip(t *testing.T) {
	hex := "82000201546573746E6163687269636874"

	pdu, err := HexToBinary(hex)
	assert.NoError(t, err)

	actual := BinaryToHex(pdu)
	assert.Equal(t, hex, actual)
}
