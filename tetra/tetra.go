package tetra

import (
	"encoding/hex"
	"regexp"
	"strings"
)

// Identity represents an identity of a party in a TETRA communication
type Identity string

// IdentityType enum according to [PEI] 6.17.11 and 6.17.12
type IdentityType byte

// All defined IdentityType values
const (
	SSI IdentityType = iota
	TSI
	SNA
	PABX
	PSTN
	ExtendedTSI
)

// TypedIdentity combines an identity with its type in one struct
type TypedIdentity struct {
	Identity Identity
	Type     IdentityType
}

var hexSanitizer = regexp.MustCompile(`\s+`)

// HexToBinary converts the hex representation used along the PEI for binary data into a slice of bytes
func HexToBinary(s string) ([]byte, error) {
	sanitized := hexSanitizer.ReplaceAllString(s, "")
	return hex.DecodeString(sanitized)
}

// BinaryToHex converts a slice of bytes into the hex representation used along the PEI for binary data
func BinaryToHex(pdu []byte) string {
	return strings.ToUpper(hex.EncodeToString(pdu))
}
