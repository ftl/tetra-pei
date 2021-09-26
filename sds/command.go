package sds

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ftl/tetra-pei/tetra"
)

type Encoder interface {
	Encode([]byte, int) ([]byte, int)
}

type EncoderFunc func() ([]byte, int)

func (f EncoderFunc) Encode() ([]byte, int) {
	return f()
}

type Requester interface {
	Request(context.Context, string) ([]string, error)
}

type RequesterFunc func(context.Context, string) ([]string, error)

func (f RequesterFunc) Request(ctx context.Context, request string) ([]string, error) {
	return f(ctx, request)
}

const (
	// CRLF line ending for AT commands
	CRLF = "\x0d\x0a"
	// CtrlZ line ending for PDUs
	CtrlZ = "\x1a"

	// SwitchToSDSTL is a short-cut for selecting the SDS-TL AI service with ISSI addressing and E2EE according to [PEI] 6.14.6
	SwitchToSDSTL = "AT+CTSDS=12,0,0,0,1"
	// SwitchToStatus is a short-cut for selecting the status AI service with ISSI addresssing according to [PEI] 6.14.6
	SwitchToStatus = "AT+CTSDS=13,0"
)

// SendMessage according to [PEI] 6.13.2
func SendMessage(destination tetra.Identity, message Encoder) string {
	pdu := make([]byte, 0, 256)
	pduBits := 0
	pdu, pduBits = message.Encode(pdu, pduBits)
	return fmt.Sprintf("AT+CMGS=%s,%d"+CRLF+"%s"+CtrlZ, destination, pduBits, tetra.BinaryToHex(pdu))
}

var sendMessageDescription = regexp.MustCompile(`^\+CMGS: .+\(\d*-(\d*)\)$`)

// RequestMaxMessagePDUBits uses the given RequesterFunc to find out how many bits a message PDU may have (see [PEI] 6.13.2).
func RequestMaxMessagePDUBits(ctx context.Context, requester Requester) (int, error) {
	responses, err := requester.Request(ctx, "AT+CMGS=?")
	if err != nil {
		return 0, err
	}
	if len(responses) < 1 {
		return 0, fmt.Errorf("no response received")
	}
	response := strings.ToUpper(strings.TrimSpace(responses[0]))
	parts := sendMessageDescription.FindStringSubmatch(response)

	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected response: %s", responses[0])
	}

	result, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return result, nil
}
