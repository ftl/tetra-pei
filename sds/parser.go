package sds

import (
	"fmt"
	"log"

	"github.com/ftl/tetra-pei/tetra"
)

type PayloadParserFunc func([]byte) (any, error)

type Parser struct {
	parsers map[ProtocolIdentifier]PayloadParserFunc
}

// NewParser returns a new SDS parser that uses the default payload parsers for
// simple text messaging (0x02), simple immediate text messaging (0x09),
// text messaging (0x82), immediate text messaging (0x89), user data header (0x8A)
func NewParser() *Parser {
	return &Parser{
		parsers: map[ProtocolIdentifier]PayloadParserFunc{
			SimpleTextMessaging:          ParseSimpleTextMessage,
			SimpleImmediateTextMessaging: ParseSimpleTextMessage,
			TextMessaging:                ParseSDSTLMessage,
			ImmediateTextMessaging:       ParseSDSTLMessage,
			UserDataHeaderMessaging:      ParseSDSTLMessage,
		},
	}
}

// Set a individual payload parser for the given protocol identifier.
func (p *Parser) Set(protocol ProtocolIdentifier, parser PayloadParserFunc) {
	p.parsers[protocol] = parser
}

// ParseIncomingMessage parses an incoming message with the given header and PDU bytes. The message may
// be part of a concatenated text message with user data header, a simple text message, a text message,
// or a status.
func (p *Parser) ParseIncomingMessage(headerString string, pduHex string) (IncomingMessage, error) {
	header, err := ParseHeader(headerString)
	if err != nil {
		return IncomingMessage{}, err
	}

	pduBytes, err := tetra.HexToBinary(pduHex)
	if err != nil {
		return IncomingMessage{}, fmt.Errorf("cannot decode hex PDU data: %w", err)
	}
	if len(pduBytes) != header.PDUBytes() {
		log.Printf("got different count of pdu bytes, expected %d, but got %d", len(pduBytes), header.PDUBytes())
	}
	if len(pduBytes) > header.PDUBytes() {
		pduBytes = pduBytes[0:header.PDUBytes()]
	}

	var result IncomingMessage
	result.Header = header
	switch header.AIService {
	case SDSTLService:
		result.Payload, err = p.parseSDSTLPDU(pduBytes)
	case StatusService:
		result.Payload, err = ParseStatus(pduBytes)
	default:
		return IncomingMessage{}, fmt.Errorf("AI service %s is not supported", header.AIService)
	}

	if err != nil {
		return IncomingMessage{}, err
	}
	return result, nil
}

func (p *Parser) parseSDSTLPDU(bytes []byte) (any, error) {
	if len(bytes) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	protocolIdentifier := ProtocolIdentifier(bytes[0])

	payloadParser, ok := p.parsers[protocolIdentifier]
	if !ok {
		return nil, fmt.Errorf("no SDS payload parser registered for protocol 0x%x", protocolIdentifier)
	}

	return payloadParser(bytes)
}
