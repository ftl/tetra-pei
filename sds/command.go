package sds

import (
	"fmt"

	"github.com/ftl/tetra-pei/tetra"
)

type Encoder interface {
	Encode([]byte, int) ([]byte, int)
}

type EncoderFunc func() ([]byte, int)

func (f EncoderFunc) Encode() ([]byte, int) {
	return f()
}

const (
	// SwitchToSDSTL is a short-cut for selecting the SDS-TL AI service according to [PEI] 6.14.6
	SwitchToSDSTL = "AT+CTSDS=12,0,0,0,1"
	// SwitchToStatus is a short-cut for selecting the status AI service according to [PEI] 6.14.6
	SwitchToStatus = "AT+CTSDS=13,0"
)

// SendMessage according to [PEI] 6.13.2
func SendMessage(destination tetra.Identity, sds Encoder) string {
	pdu := make([]byte, 0, 2000) // TODO use the maximum size allowed
	pduBits := 0
	pdu, pduBits = sds.Encode(pdu, pduBits)
	return fmt.Sprintf("AT+CMGS=%s,%d\x0d\x0a%s\x1a", destination, pduBits, tetra.BinaryToHex(pdu))
}
