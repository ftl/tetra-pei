package sds

import (
	"fmt"
	"regexp"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

/* Text related types and functions */

// TextEncoding enum according to [AI] 29.5.4.1
type TextEncoding byte

// All defined text encoding schemes, according to [AI] table 29.29
const (
	Packed7Bit TextEncoding = iota
	ISO8859_1
	ISO8859_2
	ISO8859_3
	ISO8859_4
	ISO8859_5
	ISO8859_6
	ISO8859_7
	ISO8859_8
	ISO8859_9
	ISO8859_10
	ISO8859_13
	ISO8859_14
	ISO8859_15
	CodePage437
	CodePage737
	CodePage850
	CodePage852
	CodePage855
	CodePage857
	CodePage860
	CodePage861
	CodePage863
	CodePage865
	CodePage866
	CodePage869
	UTF16BE
	VISCII
)

// TextCodecs contains encoding.Encoding instances for all supported text encoding schemes.
// Beware that not all defined schemes are actually supported here.
var TextCodecs = map[TextEncoding]encoding.Encoding{
	ISO8859_1:   charmap.ISO8859_1,
	ISO8859_2:   charmap.ISO8859_2,
	ISO8859_3:   charmap.ISO8859_3,
	ISO8859_4:   charmap.ISO8859_4,
	ISO8859_5:   charmap.ISO8859_5,
	ISO8859_6:   charmap.ISO8859_6,
	ISO8859_7:   charmap.ISO8859_7,
	ISO8859_8:   charmap.ISO8859_8,
	ISO8859_9:   charmap.ISO8859_9,
	ISO8859_10:  charmap.ISO8859_10,
	ISO8859_13:  charmap.ISO8859_13,
	ISO8859_14:  charmap.ISO8859_14,
	ISO8859_15:  charmap.ISO8859_15,
	CodePage437: charmap.CodePage437,
	CodePage850: charmap.CodePage850,
	CodePage852: charmap.CodePage852,
	CodePage855: charmap.CodePage855,
	CodePage860: charmap.CodePage860,
	CodePage863: charmap.CodePage863,
	CodePage865: charmap.CodePage865,
	CodePage866: charmap.CodePage866,
	UTF16BE:     unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
}

var fallbackCodec encoding.Encoding = charmap.ISO8859_1 // be lenient and use ISO8859-1 as fallback if anything goes havoc

// EncodingByName maps allows to access all the supported encodings by their name as string
var EncodingByName = map[string]TextEncoding{
	"ISO8859-1":   ISO8859_1,
	"ISO8859-2":   ISO8859_2,
	"ISO8859-3":   ISO8859_3,
	"ISO8859-4":   ISO8859_4,
	"ISO8859-5":   ISO8859_5,
	"ISO8859-6":   ISO8859_6,
	"ISO8859-7":   ISO8859_7,
	"ISO8859-8":   ISO8859_8,
	"ISO8859-9":   ISO8859_9,
	"ISO8859-10":  ISO8859_10,
	"ISO8859-13":  ISO8859_13,
	"ISO8859-14":  ISO8859_14,
	"ISO8859-15":  ISO8859_15,
	"CodePage437": CodePage437,
	"CodePage850": CodePage850,
	"CodePage852": CodePage852,
	"CodePage855": CodePage855,
	"CodePage860": CodePage860,
	"CodePage863": CodePage863,
	"CodePage865": CodePage865,
	"CodePage866": CodePage866,
	"UTF16BE":     UTF16BE,
}

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

// BitsToTextBytes returns the number of bytes of a text that fit into the given number of bits using the given encoding
func BitsToTextBytes(encoding TextEncoding, bits int) int {
	switch encoding {
	case Packed7Bit:
		return bits / 7
	default:
		return bits / 8
	}
}

// SplitToMaxBits splits the given text into parts that do not exceed the given maximum number of bits using the given encoding
func SplitToMaxBits(encoding TextEncoding, maxPDUBits int, text string) []string {
	if text == "" {
		return []string{}
	}

	maxPartLength := BitsToTextBytes(encoding, maxPDUBits)
	maxPartsCount := len(text)/maxPartLength + 1
	result := make([]string, 0, maxPartsCount)

	remainingText := text
	for len(remainingText) > maxPartLength {
		part := remainingText[0:maxPartLength]
		remainingText = remainingText[maxPartLength:]
		if part != "" {
			result = append(result, part)
		}
	}
	if len(remainingText) > 0 {
		result = append(result, remainingText)
	}
	return result
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
func DecodePayloadText(textEncoding TextEncoding, bytes []byte) (string, error) {
	var decoder *encoding.Decoder
	codec, ok := TextCodecs[textEncoding]
	if ok {
		decoder = codec.NewDecoder()
	} else { // we have no matching codec, but be lenient and use the fallback
		decoder = fallbackCodec.NewDecoder()
	}

	utf8, err := decoder.Bytes(bytes)
	return string(utf8), err
}

// AppendEncodedPayloadText encodes the given payload text using the given text encoding and appends the result to the given byte slice.
func AppendEncodedPayloadText(bytes []byte, bits int, text string, textEncoding TextEncoding) ([]byte, int) {
	var encodedBytes []byte
	var encodedBits int
	var err error

	var encoder *encoding.Encoder
	codec, ok := TextCodecs[textEncoding]
	if ok {
		encoder = codec.NewEncoder()
	} else { // we have no matching codec, but be lenient and use the fallback
		encoder = fallbackCodec.NewEncoder()
	}

	encodedBytes, err = encoder.Bytes([]byte(text))
	if err != nil { // something went wrong, but be lenient and use the fallback
		encodedBytes = []byte(text)
	}
	encodedBits = len(encodedBytes) * 8

	bytes = append(bytes, encodedBytes...)
	bits += encodedBits
	return bytes, bits
}

var leadingOPTA = regexp.MustCompile(`^[A-Za-z ]+#[0-9]{16}`)

func SplitLeadingOPTA(s string) (string, string) {
	opta := leadingOPTA.FindString(s)
	return opta, s[len(opta):]
}

func RemoveLeadingOPTA(s string) string {
	_, result := SplitLeadingOPTA(s)
	return result
}

var trailingITSI = regexp.MustCompile(`((\x1a\x00)|(\x0d\x0d))([0-9]{16})$`)

func SplitTrailingITSI(s string) (string, string) {
	groups := trailingITSI.FindStringSubmatch(s)
	var itsi string
	var matchLen int
	if len(groups) == 0 {
		itsi = ""
		matchLen = 0
	} else {
		itsi = groups[len(groups)-1]
		matchLen = len(groups[0])
	}
	return s[0 : len(s)-matchLen], itsi
}

func RemoveTrailingITSI(s string) string {
	result, _ := SplitTrailingITSI(s)
	return result
}
