package sds

import (
	"fmt"
	"log"
	"time"
)

/* Text related types and functions */

// TextEncoding enum according to [AI] 29.5.4.1
type TextEncoding byte

// All supported text encoding schemes, according to [AI] table 29.29
const (
	Packed7Bit TextEncoding = 0
	ISO8859_1  TextEncoding = 1
)

// TextBytes returns the length in bytes of an encoded text with
// the given number of characters and the given encoding
func TextBytes(encoding TextEncoding, length int) int {
	bits := TextBytesToBits(encoding, length)
	bytes := bits / 8
	if bits%8 > 0 {
		bytes++
	}
	return bytes
}

// TextBytesToBits returns the length in bits of an encoded text with
// the given number of characters and the given encoding
func TextBytesToBits(encoding TextEncoding, length int) int {
	switch encoding {
	case Packed7Bit:
		return length*8 - length
	default:
		return length * 8
	}
}

// ParseTextHeader in text messages and concatenated text messages.
func ParseTextHeader(bytes []byte) (TextHeader, error) {
	if len(bytes) < 1 {
		return TextHeader{}, fmt.Errorf("text header too short: %d", len(bytes))
	}

	var result TextHeader

	timestampUsed := (bytes[0] & 0x80) == 0x80
	if timestampUsed && len(bytes) < 7 {
		return TextHeader{}, fmt.Errorf("text header with timestamp too short: %d", len(bytes))
	}
	result.Encoding = TextEncoding(bytes[0] & 0x7F)

	var timestamp time.Time
	var err error
	if timestampUsed {
		timestamp, err = DecodeTimestamp(bytes[1:4])
		if err != nil {
			return TextHeader{}, err
		}
	}
	result.Timestamp = timestamp

	return result, nil
}

// TextHeader represents the meta information for text used in text messages according to [AI] 29.5.3.3
// and concatenated text messages according to [AI] 29.5.10.3
type TextHeader struct {
	Encoding  TextEncoding
	Timestamp time.Time
}

// Encode this text header
func (h TextHeader) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes = append(bytes, byte(h.Encoding))
	bits += 8
	if !h.Timestamp.IsZero() {
		bytes[len(bytes)-1] |= 0x80
		bytes = append(bytes, EncodeTimestampUTC(h.Timestamp)...)
		bits += 24
	}

	return bytes, bits
}

// Length returns the length of this text header in bytes.
func (h TextHeader) Length() int {
	if h.Timestamp.IsZero() {
		return 1
	}
	return 4
}

// DecodePayloadText decodes the actual text content using the given encoding scheme according to [AI] 29.5.4
func DecodePayloadText(encoding TextEncoding, bytes []byte) (string, error) {
	switch encoding {
	case ISO8859_1: // only ISO8859-1 at the moment
		return decodeISO8859_1(bytes)
	default: // be lenient and use ISO8859-1 as fallback
		log.Printf("encoding 0x%x is currently not supported, using ISO8859-1 as fallback", encoding)
		return decodeISO8859_1(bytes)
	}
}

func decodeISO8859_1(bytes []byte) (string, error) {
	utf8Buf := make([]rune, len(bytes))
	for i, b := range bytes {
		utf8Buf[i] = rune(b)
	}
	return string(utf8Buf), nil
}

func AppendEncodedPayloadText(bytes []byte, bits int, text string, encoding TextEncoding) ([]byte, int) {
	var encodedBytes []byte
	var encodedBits int
	switch encoding {
	case ISO8859_1: // only ISO8859-1 at the moment
		encodedBytes, encodedBits = encodeISO8859_1(text)
	default: // be lenient and use ISO8859-1 as fallback
		encodedBytes, encodedBits = encodeISO8859_1(text)
	}

	bytes = append(bytes, encodedBytes...)
	bits += encodedBits
	return bytes, bits
}

func encodeISO8859_1(text string) ([]byte, int) {
	return []byte(text), len(text) * 8
}
