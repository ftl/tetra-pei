package sds

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ftl/tetra-pei/tetra"
)

// ParseIncomingMessage parses an incoming message with the given header and PDU bytes. The message may
// be part of a concatenated text message with user data header, a simple text message, a text message,
// or a status.
func ParseIncomingMessage(headerString string, pduHex string) (IncomingMessage, error) {
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
		result.Payload, err = ParseSDSTLPDU(pduBytes)
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

type IncomingMessage struct {
	Header  Header
	Payload interface{}
}

// ParseHeader from the given string. The string must include the +CTSDSR: token.
func ParseHeader(s string) (Header, error) {
	if !strings.HasPrefix(s, "+CTSDSR:") {
		return Header{}, fmt.Errorf("invalid header, +CTSDSR expected: %s", s)
	}

	var result Header
	var pduBitCountField string
	headerFields := strings.Split(s[8:], ",")
	// field order according to ETSI TS 100 392-5 V2.7.1
	// +CTSDSR: <AI service>, [<calling party identity>], [<calling party identity type>], <called party identity>, <called party identity type>, <length>, [<end to end encryption>]<CR><LF>user data	
	switch len(headerFields) {
	case 3: // minimum set
		result.AIService = AIService(strings.TrimSpace(headerFields[0]))
		result.Destination = tetra.Identity(strings.TrimSpace(headerFields[1]))
		pduBitCountField = headerFields[2]
	case 4: // minimum set with identity type
		result.AIService = AIService(strings.TrimSpace(headerFields[0]))
		result.Destination = tetra.Identity(strings.TrimSpace(headerFields[1]))
		pduBitCountField = headerFields[3]
	case 6, 7: // with source, with end-to-end encryption
		result.AIService = AIService(strings.TrimSpace(headerFields[0]))
		result.Source = tetra.Identity(strings.TrimSpace(headerFields[1]))
		result.Destination = tetra.Identity(strings.TrimSpace(headerFields[3]))
		pduBitCountField = headerFields[5]
	default:
		return Header{}, fmt.Errorf("invalid header, wrong field count: %s", s)
	}

	var err error
	result.PDUBits, err = strconv.Atoi(strings.TrimSpace(pduBitCountField))
	if err != nil {
		return Header{}, fmt.Errorf("invalid PDU bit count %s: %v", pduBitCountField, err)
	}

	return result, nil
}

// Header represents the information provided with the AT+CTSDSR unsolicited response indicating an incoming SDS.
// see [PEI] 6.13.3
type Header struct {
	AIService   AIService
	Source      tetra.Identity
	Destination tetra.Identity
	PDUBits     int
}

// PDUBytes returns the size of the following PDU in bytes.
func (h Header) PDUBytes() int {
	result := h.PDUBits / 8
	if (h.PDUBits % 8) != 0 {
		result++
	}
	return result
}

// AIService enum according to [PEI] 6.17.3
type AIService string

// All AI services relevant for SDS handling, according to [PEI] 6.17.3
const (
	SDS1Service   AIService = "9"
	SDS2Service   AIService = "10"
	SDS3Service   AIService = "11"
	SDSTLService  AIService = "12"
	StatusService AIService = "13"
)

/* General types used in the PDU */

// ProtocolIdentifier enum according to [AI] 29.4.3.9
type ProtocolIdentifier byte

// Encode this protocol identifier
func (p ProtocolIdentifier) Encode(bytes []byte, bits int) ([]byte, int) {
	return append(bytes, byte(p)), bits + 8
}

// Length of this protocol identifier in bytes.
func (p ProtocolIdentifier) Length() int {
	return 1
}

// All protocol identifiers relevant for SDS handling, according to [AI] table 29.21
const (
	SimpleTextMessaging            ProtocolIdentifier = 0x02
	SimpleImmediateTextMessaging   ProtocolIdentifier = 0x09
	SimpleConcatenatedSDSMessaging ProtocolIdentifier = 0x0C
	TextMessaging                  ProtocolIdentifier = 0x82
	ImmediateTextMessaging         ProtocolIdentifier = 0x89
	UserDataHeaderMessaging        ProtocolIdentifier = 0x8A
	ConcatenatedSDSMessaging       ProtocolIdentifier = 0x8C
	Callout                        ProtocolIdentifier = 0xC3
)

/* SDS-TL related types and functions */

// ParseSDSTLPDU parses an SDS-TL PDU from the given bytes according to [AI] 29.4.1.
// This function currently supports only a subset of the possible protocol identifiers:
// Simple text messaging (0x02), simple immediate text messaging (0x09), text messaging (0x82),
// immediate text messaging (0x89), message with user data header (0x8A),
// concatenated SDS messaging (0x8C), and callout (0xC3).
func ParseSDSTLPDU(bytes []byte) (interface{}, error) {
	if len(bytes) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	switch ProtocolIdentifier(bytes[0]) {
	case SimpleTextMessaging, SimpleImmediateTextMessaging:
		return ParseSimpleTextMessage(bytes)
	case TextMessaging, ImmediateTextMessaging, UserDataHeaderMessaging, ConcatenatedSDSMessaging, Callout:
		return parseSDSTLMessage(bytes)
	default:
		return nil, fmt.Errorf("protocol 0x%x not supported", bytes[0])
	}
}

func parseSDSTLMessage(bytes []byte) (interface{}, error) {
	if len(bytes) < 2 {
		return nil, fmt.Errorf("payload too short: %d", len(bytes))
	}

	messageType := SDSTLMessageType(bytes[1] >> 4)
	switch messageType {
	case SDSTransferMessage:
		return ParseSDSTransfer(bytes)
	case SDSReportMessage:
		return ParseSDSReport(bytes)
	case SDSAcknowledgeMessage:
		return ParseSDSAcknowledge(bytes)
	default:
		return nil, fmt.Errorf("SDS-TL message type 0x%x is not supported", messageType)
	}
}

// SDSTLMessageType enum according to [AI] 29.4.3.8
type SDSTLMessageType byte

// All SDS-TL message types according to [AI] table 29.20
const (
	SDSTransferMessage    SDSTLMessageType = 0
	SDSReportMessage      SDSTLMessageType = 1
	SDSAcknowledgeMessage SDSTLMessageType = 2
)

// ParseSDSAcknowledge parses a SDS-ACK PDU from the given bytes
func ParseSDSAcknowledge(bytes []byte) (SDSAcknowledge, error) {
	if len(bytes) < 4 {
		return SDSAcknowledge{}, fmt.Errorf("SDS-ACK PDU too short: %d", len(bytes))
	}

	var result SDSAcknowledge

	result.protocol = ProtocolIdentifier(bytes[0])
	result.DeliveryStatus = DeliveryStatus(bytes[2])
	result.MessageReference = MessageReference(bytes[3])

	return result, nil
}

// SDSAcknowledge represents the SDS-ACK PDU contents as defined in [AI] 29.4.2.1
type SDSAcknowledge struct {
	protocol         ProtocolIdentifier
	DeliveryStatus   DeliveryStatus
	MessageReference MessageReference
}

// ParseSDSReport parses a SDS-REPORT PDU from the given bytes
func ParseSDSReport(bytes []byte) (SDSReport, error) {
	if len(bytes) < 4 {
		return SDSReport{}, fmt.Errorf("SDS-REPORT PDU too short: %d", len(bytes))
	}

	var result SDSReport

	result.protocol = ProtocolIdentifier(bytes[0])
	result.AckRequired = ((bytes[1] & 0x08) != 0)
	storeForwardControl := (bytes[1] & 0x01) != 0
	result.DeliveryStatus = DeliveryStatus(bytes[2])
	result.MessageReference = MessageReference(bytes[3])

	userdataStart := 4
	if storeForwardControl {
		sfc, err := ParseStoreForwardControl(bytes[4:])
		if err != nil {
			return SDSReport{}, err
		}

		result.StoreForwardControl = sfc
		userdataStart += sfc.Length()
	}

	if userdataStart < len(bytes) {
		result.UserData = bytes[userdataStart:]
	}

	return result, nil
}

// NewSDSReport creates a new SDS-REPORT PDU based on the given SDS-TRANSFER PDU without store/forward control information.
func NewSDSReport(sdsTransfer SDSTransfer, ackRequired bool, deliveryStatus DeliveryStatus) SDSReport {
	return SDSReport{
		protocol:         sdsTransfer.protocol,
		AckRequired:      ackRequired,
		DeliveryStatus:   deliveryStatus,
		MessageReference: sdsTransfer.MessageReference,
	}
}

// SDSReport represents the SDS-REPORT PDU contents as defined in [AI] 29.4.2.2
type SDSReport struct {
	protocol            ProtocolIdentifier
	AckRequired         bool
	DeliveryStatus      DeliveryStatus
	MessageReference    MessageReference
	StoreForwardControl StoreForwardControl

	// user data

	UserData []byte
}

// Encode this SDS-REPORT PDU
func (r SDSReport) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes, bits = r.protocol.Encode(bytes, bits)

	var byte1 byte
	byte1 = byte(SDSReportMessage) << 4
	if r.AckRequired {
		byte1 |= 0x08
	}
	bytes = append(bytes, byte1)
	bits += 8

	bytes, bits = r.DeliveryStatus.Encode(bytes, bits)
	bytes, bits = r.MessageReference.Encode(bytes, bits)

	return bytes, bits
}

// ParseSDSShortReport parses a SDS-SHORT-REPORT PDU from the given bytes
func ParseSDSShortReport(bytes []byte) (SDSShortReport, error) {
	if len(bytes) != 2 {
		return SDSShortReport{}, fmt.Errorf("SDS-SHORT-REPORT PDU invalid length %d", len(bytes))
	}
	if (bytes[0] & SDSShortReportPDUIdentifier) != SDSShortReportPDUIdentifier {
		return SDSShortReport{}, fmt.Errorf("SDS-SHORT-REPORT PDU invalid PDU identifier 0x%x", bytes[0]&SDSShortReportPDUIdentifier)
	}

	var result SDSShortReport

	result.ReportType = ShortReportType(bytes[0] & 0x03)
	result.MessageReference = MessageReference(bytes[1])

	return result, nil
}

// SDSShortReportPDUIdentifier for SDS-SHORT-REPORT PDUs
const SDSShortReportPDUIdentifier byte = 0x7A

// SDSShortReport represents the SDS-SHORT-REPORT PDU contents as defined in [AI] 29.4.2.3
type SDSShortReport struct {
	ReportType       ShortReportType
	MessageReference MessageReference
}

// Encode this SDS-SHORT-REPORT PDU
func (r SDSShortReport) Encode(bytes []byte, bits int) ([]byte, int) {
	byte0 := byte(0x7C) | byte(r.ReportType)
	bytes = append(bytes, byte0)
	bits += 8

	bytes, bits = r.MessageReference.Encode(bytes, bits)

	return bytes, bits
}

// ParseSDSTransfer parses a SDS-TRANSFER PDU from the given bytes
func ParseSDSTransfer(bytes []byte) (SDSTransfer, error) {
	if len(bytes) < 4 {
		return SDSTransfer{}, fmt.Errorf("SDS-TRANSFER PDU too short: %d", len(bytes))
	}

	var result SDSTransfer

	result.protocol = ProtocolIdentifier(bytes[0])
	result.DeliveryReportRequest = DeliveryReportRequest((bytes[1] & 0x0C) >> 2)
	result.ServiceSelectionShortFormReport = (bytes[1] & 0x02) == 0
	storeForwardControl := (bytes[1] & 0x01) != 0
	result.MessageReference = MessageReference(bytes[2])

	userdataStart := 3
	if storeForwardControl {
		sfc, err := ParseStoreForwardControl(bytes[3:])
		if err != nil {
			return SDSTransfer{}, err
		}

		result.StoreForwardControl = sfc
		userdataStart += sfc.Length()
	}

	var sdu interface{}
	var err error

	switch result.protocol {
	case TextMessaging, ImmediateTextMessaging:
		sdu, err = ParseTextSDU(bytes[userdataStart:])
	case UserDataHeaderMessaging:
		sdu, err = ParseConcatenatedTextSDU(bytes[userdataStart:])
	case ConcatenatedSDSMessaging:
		sdu, err = ParseConcatenatedSDSMessageSDU(bytes[userdataStart:])
	case Callout:
		sdu, err = ParseCalloutSDU(bytes[userdataStart:])
	default:
		return SDSTransfer{}, fmt.Errorf("protocol 0x%x is not supported as SDS-TRANSFER content", bytes[0])
	}

	if err != nil {
		return SDSTransfer{}, err
	}
	result.UserData = sdu

	return result, nil
}

// NewTextMessageTransfer returns a new SDS-TRANSFER PDU for text messaging with the given parameters
func NewTextMessageTransfer(messageReference MessageReference, immediate bool, deliveryReport DeliveryReportRequest, encoding TextEncoding, text string) SDSTransfer {
	var protocol ProtocolIdentifier
	if immediate {
		protocol = ImmediateTextMessaging
	} else {
		protocol = TextMessaging
	}

	return SDSTransfer{
		protocol:              protocol,
		MessageReference:      messageReference,
		DeliveryReportRequest: deliveryReport,
		UserData: TextSDU{
			TextHeader: TextHeader{
				Encoding: encoding,
			},
			Text: text,
		},
	}
}

// NewConcatenatedMessageTransfer returns a set of SDS_TRANSFER PDUs for that make up the given text using concatenated text messages with a UDH.
func NewConcatenatedMessageTransfer(messageReference MessageReference, deliveryReport DeliveryReportRequest, encoding TextEncoding, maxPDUBits int, text string) []SDSTransfer {
	blueprint := SDSTransfer{
		protocol:              UserDataHeaderMessaging,
		MessageReference:      messageReference,
		DeliveryReportRequest: deliveryReport,
		UserData: ConcatenatedTextSDU{
			TextSDU: TextSDU{
				TextHeader: TextHeader{
					Encoding: encoding,
				},
				Text: "",
			},
			UserDataHeader: ConcatenatedTextUDH{
				ElementID:        ConcatenatedTextMessageWithShortReference,
				MessageReference: uint16(messageReference),
				TotalNumber:      0,
				SequenceNumber:   0,
			},
		},
	}
	blueprintBits := blueprint.Length() * 8

	textParts := SplitToMaxBits(encoding, maxPDUBits-blueprintBits, text)

	if len(textParts) == 1 {
		return []SDSTransfer{{
			protocol:              TextMessaging,
			MessageReference:      messageReference,
			DeliveryReportRequest: deliveryReport,
			UserData: TextSDU{
				TextHeader: TextHeader{
					Encoding: encoding,
				},
				Text: text,
			},
		}}
	}

	result := make([]SDSTransfer, len(textParts))
	for i, textPart := range textParts {
		result[i] = SDSTransfer{
			protocol:                        UserDataHeaderMessaging,
			ServiceSelectionShortFormReport: true,
			MessageReference:                messageReference + MessageReference(i),
			DeliveryReportRequest:           deliveryReport,
			UserData: ConcatenatedTextSDU{
				TextSDU: TextSDU{
					TextHeader: TextHeader{
						Encoding: encoding,
					},
					Text: textPart,
				},
				UserDataHeader: ConcatenatedTextUDH{
					ElementID:        ConcatenatedTextMessageWithShortReference,
					MessageReference: uint16(messageReference),
					TotalNumber:      byte(len(textParts)),
					SequenceNumber:   byte(i + 1),
				},
			},
		}
	}

	return result
}

// SDSTransfer represents the SDS-TRANSFER PDU contents as defined in [AI] 29.4.2.4
type SDSTransfer struct {
	protocol                        ProtocolIdentifier
	DeliveryReportRequest           DeliveryReportRequest
	ServiceSelectionShortFormReport bool
	MessageReference                MessageReference
	StoreForwardControl             StoreForwardControl
	UserData                        interface{}
}

// Encode this SDS-TRANSFER PDU
func (m SDSTransfer) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes, bits = m.protocol.Encode(bytes, bits)

	var byte1 byte
	byte1 = byte(SDSTransferMessage) << 4
	byte1 |= byte(m.DeliveryReportRequest) << 2
	if !m.ServiceSelectionShortFormReport {
		byte1 |= 0x02
	}
	bytes = append(bytes, byte1)
	bits += 8

	bytes, bits = m.MessageReference.Encode(bytes, bits)

	switch sdu := m.UserData.(type) {
	case TextSDU:
		bytes, bits = sdu.Encode(bytes, bits)
	case ConcatenatedTextSDU:
		bytes, bits = sdu.Encode(bytes, bits)
	}

	return bytes, bits
}

// Length of this SDS-TRANSFER in bytes.
func (m SDSTransfer) Length() int {
	var result int
	result += m.protocol.Length()
	result++ // byte1
	result++ // message reference
	switch sdu := m.UserData.(type) {
	case TextSDU:
		result += sdu.Length()
	case ConcatenatedTextSDU:
		result += sdu.Length()
	}
	return result
}

// ReceivedReportRequested indicates if for this SDS-TRANSFER PDU a delivery report is requested for receipt
func (m SDSTransfer) ReceivedReportRequested() bool {
	return m.DeliveryReportRequest == MessageReceivedReportRequested ||
		m.DeliveryReportRequest == MessageReceivedAndConsumedReportRequested
}

// ConsumedReportRequested indicates if for this SDS-TRANSFER PDU a delivery report is requested for consumation
func (m SDSTransfer) ConsumedReportRequested() bool {
	return m.DeliveryReportRequest == MessageConsumedReportRequested ||
		m.DeliveryReportRequest == MessageReceivedAndConsumedReportRequested
}

// Immediate indiciates if this message should be displayed/handled immediately by the TE.
func (m SDSTransfer) Immediate() bool {
	return m.protocol == ImmediateTextMessaging
}

// MessageReference according to [AI] 29.4.3.7
type MessageReference byte

// Encode this message reference
func (m MessageReference) Encode(bytes []byte, bits int) ([]byte, int) {
	return append(bytes, byte(m)), bits + 8
}

// DeliveryStatus according to [AI] 29.4.3.2
type DeliveryStatus byte

// Encode this delivery status
func (s DeliveryStatus) Encode(bytes []byte, bits int) ([]byte, int) {
	return append(bytes, byte(s)), bits + 8
}

// Success indicates if this status represents a success (see [AI] table 29.16).
func (s DeliveryStatus) Success() bool {
	return (s & 0xE0) == 0x00
}

// TemporaryError indicates if this status represents a temporary error (see [AI] table 29.16).
func (s DeliveryStatus) TemporaryError() bool {
	return (s & 0xE0) == 0x20
}

// DataDeliveryFailed indicates if this status represents a data transfer failure (see [AI] table 29.16).
func (s DeliveryStatus) DataDeliveryFailed() bool {
	return (s & 0xE0) == 0x40
}

// FlowControl indicates if this status represents flow control information (see [AI] table 29.16).
func (s DeliveryStatus) FlowControl() bool {
	return (s & 0xE0) == 0x06
}

// EndToEndControl indicates if this status represents end to end control information (see [AI] table 29.16).
func (s DeliveryStatus) EndToEndControl() bool {
	return (s & 0xE0) == 0x80
}

// All DeliveryStatus values according to [AI] table 29.16
const (
	// Success

	ReceiptAckByDestination                  DeliveryStatus = 0x00
	ReceiptReportAck                         DeliveryStatus = 0x01
	ConsumedByDestination                    DeliveryStatus = 0x02
	ConsumedReportAck                        DeliveryStatus = 0x03
	MessageForwardedToExternalNetwork        DeliveryStatus = 0x04
	SentToGroupAckPresented                  DeliveryStatus = 0x05
	ConcatenationPartReceiptAckByDestination DeliveryStatus = 0x06

	// Temporary Error

	Congestion                           DeliveryStatus = 0x20
	MessageStored                        DeliveryStatus = 0x21
	DestinationNotReachableMessageStored DeliveryStatus = 0x22

	// Data Transfer Failed

	NetworkOverload                          DeliveryStatus = 0x40
	ServicePermanentlyNotAvailable           DeliveryStatus = 0x41
	ServiceTemporaryNotAvailable             DeliveryStatus = 0x42
	SourceNotAuthorized                      DeliveryStatus = 0x43
	DestinationNotAuthorzied                 DeliveryStatus = 0x44
	UnknownDestGatewayServiceAddress         DeliveryStatus = 0x45
	UnknownForwardAddress                    DeliveryStatus = 0x46
	GroupAddressWithIndividualService        DeliveryStatus = 0x47
	ValidityPeriodExpiredNotReceived         DeliveryStatus = 0x48
	ValidityPeriodExpiredNotConsumed         DeliveryStatus = 0x49
	DeliveryFailed                           DeliveryStatus = 0x4A
	DestinationNotRegistered                 DeliveryStatus = 0x4B
	DestinationQueueFull                     DeliveryStatus = 0x4C
	MessageTooLong                           DeliveryStatus = 0x4D
	DestinationDoesNotSupportSDSTL           DeliveryStatus = 0x4E
	DestinationHostNotConnected              DeliveryStatus = 0x4F
	ProtocolNotSupported                     DeliveryStatus = 0x50
	DataCodingSchemeNotSupported             DeliveryStatus = 0x51
	DestinationMemoryFullMessageDiscarded    DeliveryStatus = 0x52
	DestinationNotAcceptingSDS               DeliveryStatus = 0x53
	ConcatednatedMessageTooLong              DeliveryStatus = 0x54
	DestinationAddressProhibited             DeliveryStatus = 0x56
	CannotRouteToExternalNetwork             DeliveryStatus = 0x57
	UnknownExternalSubscriberNumber          DeliveryStatus = 0x58
	NegativeReportAcknowledgement            DeliveryStatus = 0x59
	DestinationNotReachable                  DeliveryStatus = 0x5A
	TextDistributionError                    DeliveryStatus = 0x5B
	CorruptInformationElement                DeliveryStatus = 0x5C
	NotAllConcatenationPartsReceived         DeliveryStatus = 0x5D
	DestinationEngagedInAnotherServiceBySwMI DeliveryStatus = 0x5E
	DestinationEngagedInAnotherServiceByDest DeliveryStatus = 0x5F

	// Flow Control

	DestinationMemoryFull      DeliveryStatus = 0x60
	DestinationMemoryAvailable DeliveryStatus = 0x61
	StartPendingMessages       DeliveryStatus = 0x62
	NoPendingMessages          DeliveryStatus = 0x63

	// End to End Control

	StopSending  DeliveryStatus = 0x80
	StartSending DeliveryStatus = 0x81
)

// ShortReportType enum according to [AI] 29.4.3.10
type ShortReportType byte

// All short report type values accoring to [AI] table 29.22
const (
	ProtocolOrEncodingNotSupportedShort ShortReportType = 0x00
	DestinationMemoryFullShort          ShortReportType = 0x01
	MessageReceivedShort                ShortReportType = 0x02
	MessageConsumedShort                ShortReportType = 0x03
)

// DeliveryReportRequest enum according to [AI] 29.4.3.3
type DeliveryReportRequest byte

// All delivery report requests according to [AI] table 29.17
const (
	NoReportRequested                         DeliveryReportRequest = 0x00
	MessageReceivedReportRequested            DeliveryReportRequest = 0x01
	MessageConsumedReportRequested            DeliveryReportRequest = 0x02
	MessageReceivedAndConsumedReportRequested DeliveryReportRequest = 0x03
)

// ParseStoreForwardControl from the given bytes.
func ParseStoreForwardControl(bytes []byte) (StoreForwardControl, error) {
	if len(bytes) < 1 {
		return StoreForwardControl{}, fmt.Errorf("store forward control too short: %d", len(bytes))
	}
	var result StoreForwardControl

	result.Valid = true
	result.ValidityPeriod = ParseValidityPeriod(bytes[0] >> 3)
	result.ForwardAddressType = ForwardAddressType(bytes[0] & 3)

	switch result.ForwardAddressType {
	case ForwardToSNA:
		if len(bytes) < 2 {
			return StoreForwardControl{}, fmt.Errorf("store forward control with SNA too short: %d", len(bytes))
		}
		result.ForwardAddressSNA = ForwardAddressSNA(bytes[1])
	case ForwardToSSI:
		if len(bytes) < 4 {
			return StoreForwardControl{}, fmt.Errorf("store forward control with SSI too short: %d", len(bytes))
		}
		copy(result.ForwardAddressSSI[:], bytes[1:4])
	case ForwardToTSI:
		if len(bytes) < 4 {
			return StoreForwardControl{}, fmt.Errorf("store forward control with TSI too short: %d", len(bytes))
		}
		copy(result.ForwardAddressSSI[:], bytes[1:4])
	case ForwardToExternalSubscriberNumber:
		if len(bytes) < 2 {
			return StoreForwardControl{}, fmt.Errorf("store forward control with external subscriber number too short: %d", len(bytes))
		}
		l := int(bytes[1])
		bl := l / 2
		if l%2 > 0 {
			bl++
		}
		if len(bytes) < 2+bl {
			return StoreForwardControl{}, fmt.Errorf("store forward control with external subscriber number too short: %d", len(bytes))
		}

		result.ExternalSubscriberNumber = make(ExternalSubscriberNumber, 0, l)
		d := 0
		for i := 0; i < bl; i++ {
			result.ExternalSubscriberNumber[d] = ExternalSubscriberNumberDigit(bytes[i] >> 4)
			d++
			if d < l {
				result.ExternalSubscriberNumber[d+1] = ExternalSubscriberNumberDigit(bytes[i] & 0x0F)
				d++
			}
		}
	}

	return result, nil
}

// StoreForwardControl represents the optional store and forward control information contained in the SDS-REPORT and SDS-TRANSFER PDUs.
type StoreForwardControl struct {
	// Valid indicates if this StoreForwardControl instance contains valid data. Valid is false if store and forward control is not used with this message.
	Valid                    bool
	ValidityPeriod           ValidityPeriod
	ForwardAddressType       ForwardAddressType
	ForwardAddressSNA        ForwardAddressSNA
	ForwardAddressSSI        ForwardAddressSSI
	ForwardAddressExtension  ForwardAddressExtension
	ExternalSubscriberNumber ExternalSubscriberNumber
}

// Length returns the length of this encoded store forward control in bytes.
func (s StoreForwardControl) Length() int {
	switch s.ForwardAddressType {
	case ForwardToSNA:
		return 2
	case ForwardToSSI:
		return 4
	case ForwardToTSI:
		return 4
	case ForwardToExternalSubscriberNumber:
		l := len(s.ExternalSubscriberNumber) / 2
		if len(s.ExternalSubscriberNumber)%2 > 0 {
			l++
		}
		return 2 + l
	case NoForwardAddressPresent:
		return 1
	default:
		return 1
	}
}

// ValidityPeriod according to [AI] 29.4.3.14
type ValidityPeriod time.Duration

// InfinitelyValid represents the infinite validity period (31).
const InfinitelyValid ValidityPeriod = -1

// DecodeValidityPeriod from a 5 bits value according to [AI] table 29.25
func ParseValidityPeriod(b byte) ValidityPeriod {
	switch {
	case b == 0:
		return 0
	case b <= 6:
		return ValidityPeriod(time.Duration(b) * 10 * time.Second)
	case b <= 10:
		return ValidityPeriod(time.Duration(b-5) * time.Minute)
	case b <= 16:
		return ValidityPeriod(time.Duration(b-10) * 10 * time.Minute)
	case b <= 21:
		return ValidityPeriod(time.Duration(b-15) * time.Hour)
	case b <= 24:
		return ValidityPeriod(time.Duration(b-20) * 6 * time.Hour)
	case b <= 30:
		return ValidityPeriod(time.Duration(b-24) * 48 * time.Hour)
	default:
		return InfinitelyValid
	}
}

// Encode the validity period into 5 bits, according to [AI] table 29.25
func (p ValidityPeriod) Encode() ([]byte, int) {
	d := time.Duration(p)
	var result byte
	incIfRemainder := func(resultDuration time.Duration) {
		remainder := d - resultDuration
		if remainder > 0 {
			result++
		}
	}

	switch {
	case d == 0:
		return []byte{0}, 8
	case d <= time.Minute:
		result = byte(int(d.Truncate(time.Second).Seconds() / 10))
		incIfRemainder(time.Duration(result) * 10 * time.Second)
		return []byte{result}, 8
	case d <= 5*time.Minute:
		result = byte(int(d.Truncate(time.Minute).Minutes()))
		incIfRemainder(time.Duration(result) * time.Minute)
		return []byte{result + 5}, 8
	case d <= time.Hour:
		result = byte(int(d.Truncate(time.Minute).Minutes() / 10))
		incIfRemainder(time.Duration(result) * 10 * time.Minute)
		return []byte{result + 10}, 8
	case d <= 6*time.Hour:
		result = byte(int(d.Truncate(time.Hour).Hours()))
		incIfRemainder(time.Duration(result) * time.Hour)
		return []byte{result + 15}, 8
	case d <= 24*time.Hour:
		result = byte(int(d.Truncate(time.Hour).Hours() / 6))
		incIfRemainder(time.Duration(result) * 6 * time.Hour)
		return []byte{result + 20}, 8
	case d <= 12*24*time.Hour:
		result = byte(int(d.Truncate(time.Hour).Hours() / 48))
		incIfRemainder(time.Duration(result) * 48 * time.Hour)
		return []byte{result + 24}, 8
	default:
		return []byte{31}, 8 // infinite
	}
}

// ForwardAddressType enum according to [AI] 29.4.3.5
type ForwardAddressType byte

// All forward address type values according to [AI] table 29.18
const (
	ForwardToSNA                      ForwardAddressType = 0x00
	ForwardToSSI                      ForwardAddressType = 0x01
	ForwardToTSI                      ForwardAddressType = 0x02
	ForwardToExternalSubscriberNumber ForwardAddressType = 0x03
	NoForwardAddressPresent           ForwardAddressType = 0x07
)

// ForwardAddressSNA according to [AI] 29.4.3.6
type ForwardAddressSNA byte

// ForwardAddressSSI according to [AI] 29.4.3.6
type ForwardAddressSSI [3]byte

// ForwardAddressExtendsion according to [AI] 29.4.3.6
type ForwardAddressExtension [3]byte

// ExternalSubscriberNumber according to [AI] 29.4.3.6, contains an arbitrary number of digits.
type ExternalSubscriberNumber []ExternalSubscriberNumberDigit

// ExternalSubscriberNumberDigit represents one digit in the ExternalSubscriberNumber
type ExternalSubscriberNumberDigit byte // its only 4 bits per digit

/* Simple Text Messaging related types and functions */

// ParseSimpleTextMessage parses a simple text message PDU
func ParseSimpleTextMessage(bytes []byte) (SimpleTextMessage, error) {
	if len(bytes) < 2 {
		return SimpleTextMessage{}, fmt.Errorf("simple text message PDU too short: %d", len(bytes))
	}

	var result SimpleTextMessage
	result.protocol = ProtocolIdentifier(bytes[0])
	result.Encoding = TextEncoding(bytes[1] & 0x7F)

	text, err := DecodePayloadText(result.Encoding, bytes[2:])
	if err != nil {
		return SimpleTextMessage{}, err
	}
	result.Text = text

	return result, nil
}

// NewSimpleTextMessage returns a new simple text message PDU according to the given parameters
func NewSimpleTextMessage(immediate bool, encoding TextEncoding, text string) SimpleTextMessage {
	var protocol ProtocolIdentifier
	if immediate {
		protocol = ImmediateTextMessaging
	} else {
		protocol = TextMessaging
	}

	return SimpleTextMessage{
		protocol: protocol,
		Encoding: encoding,
		Text:     text,
	}
}

// SimpleTextMessage represents the data of a simple text messaging PDU, according to [AI] 29.5.2.3
type SimpleTextMessage struct {
	protocol ProtocolIdentifier
	Encoding TextEncoding
	Text     string
}

// Immediate indiciates if this message should be displayed/handled immediately by the TE.
func (m SimpleTextMessage) Immediate() bool {
	return m.protocol == SimpleImmediateTextMessaging
}

// Encode this simple text message
func (m SimpleTextMessage) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes, bits = m.protocol.Encode(bytes, bits)
	bytes = append(bytes, byte(m.Encoding))
	bits += 8
	bytes, bits = AppendEncodedPayloadText(bytes, bits, m.Text, m.Encoding)

	return bytes, bits
}

/* Text messaging related types and functions */

// ParseTextSDU parses the user data of a text message.
func ParseTextSDU(bytes []byte) (TextSDU, error) {
	textHeader, err := ParseTextHeader(bytes)
	if err != nil {
		return TextSDU{}, err
	}
	textPayloadStart := textHeader.Length()
	text, err := DecodePayloadText(textHeader.Encoding, bytes[textPayloadStart:])
	if err != nil {
		return TextSDU{}, err
	}

	return TextSDU{
		TextHeader: textHeader,
		Text:       text,
	}, nil
}

// TextSDU according to [AI] 29.5.3.3
type TextSDU struct {
	TextHeader
	Text string
}

// Encode this text SDU
func (t TextSDU) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes, bits = t.TextHeader.Encode(bytes, bits)
	bytes, bits = AppendEncodedPayloadText(bytes, bits, t.Text, t.TextHeader.Encoding)

	return bytes, bits
}

// Length returns the length of this encoded text SDU in bytes.
func (t TextSDU) Length() int {
	return t.TextHeader.Length() + TextBytes(t.Encoding, len(t.Text))
}

/* Concatenated text messageing related types and functions */

// ParseConcatenatedTextSDU parses the user data of a message with user data header.
func ParseConcatenatedTextSDU(bytes []byte) (ConcatenatedTextSDU, error) {
	/*
		Example PDU with User Data Header: 8A00C98D045A8F050003C90201

		8A: Protocol Identifier[8]
		00: Message Type[4], Delivery Report Request[2], Service Selection/Short form report[1], Store/forward control[1]
		C9: Message Reference[8] (0xC9) <-- This one is incremented for each part of the concatenated message
		<no store forward control information>
		8D: Timestamp Used[1] (yes), Text Encoding Scheme[7] (ISO8859-15/Latin 0)
		04 5A 8F: Timestamp[24]
		05: User Data Header length[8] (5)
		00: UDH Information Element ID[8] (0)
		03: UDH Information Element Length[8] (3)
		C9: Message Reference[8] (0xC9) <-- This is always the message reference of the first part
		02: Total number of parts[8] (2)
		01: Sequence number of current part[8] (1) <-- 1-based, first part == 1

		and then comes the text data
	*/

	textHeader, err := ParseTextHeader(bytes)
	if err != nil {
		return ConcatenatedTextSDU{}, err
	}

	udhStart := textHeader.Length()
	udh, err := ParseConcatenatedTextUDH(bytes[udhStart:])
	if err != nil {
		return ConcatenatedTextSDU{}, err
	}

	textPayloadStart := udhStart + udh.Length()
	text, err := DecodePayloadText(textHeader.Encoding, bytes[textPayloadStart:])
	if err != nil {
		return ConcatenatedTextSDU{}, err
	}

	return ConcatenatedTextSDU{
		TextSDU: TextSDU{
			TextHeader: textHeader,
			Text:       text,
		},
		UserDataHeader: udh,
	}, nil
}

// ConcatenatedTextSDU according to [AI] 29.5.10.3
type ConcatenatedTextSDU struct {
	TextSDU
	UserDataHeader ConcatenatedTextUDH
}

// Encode this concatenated text SDU
func (t ConcatenatedTextSDU) Encode(bytes []byte, bits int) ([]byte, int) {
	bytes, bits = t.TextHeader.Encode(bytes, bits)
	bytes, bits = t.UserDataHeader.Encode(bytes, bits)
	bytes, bits = AppendEncodedPayloadText(bytes, bits, t.Text, t.TextHeader.Encoding)

	return bytes, bits
}

// Length returns the length of this encoded concatenated text SDU in bytes.
func (t ConcatenatedTextSDU) Length() int {
	return t.TextSDU.Length() + t.UserDataHeader.Length()
}

// ParseConcatenatedTextUDH according to [AI] table 29.48
func ParseConcatenatedTextUDH(bytes []byte) (ConcatenatedTextUDH, error) {
	if len(bytes) < 6 {
		return ConcatenatedTextUDH{}, fmt.Errorf("concatenated text UDH too short: %d", len(bytes))
	}

	var result ConcatenatedTextUDH

	result.HeaderLength = bytes[0]
	result.ElementID = UDHInformationElementID(bytes[1])
	result.ElementLength = bytes[2]
	numbersStart := 4
	if result.ElementID == ConcatenatedTextMessageWithShortReference {
		if result.ElementLength != 3 {
			return ConcatenatedTextUDH{}, fmt.Errorf("UDH information element length invalid, got %d but expected 3", result.ElementLength)
		}
		result.MessageReference = uint16(bytes[3])
	} else {
		if result.ElementLength != 4 {
			return ConcatenatedTextUDH{}, fmt.Errorf("UDH information element length invalid, got %d but expected 4", result.ElementLength)
		}
		if len(bytes) < 7 {
			return ConcatenatedTextUDH{}, fmt.Errorf("concatenated text UDH with long reference too short: %d", len(bytes))
		}
		numbersStart = 5
		result.MessageReference = (uint16(bytes[4]) << 8) | uint16(bytes[3])
	}
	result.TotalNumber = bytes[numbersStart]
	result.SequenceNumber = bytes[numbersStart+1]

	return result, nil
}

// ConcatenatedTextUDH contents according to [AI] 29.5.10.3
type ConcatenatedTextUDH struct {
	HeaderLength     byte
	ElementID        UDHInformationElementID
	ElementLength    byte
	MessageReference uint16
	TotalNumber      byte
	SequenceNumber   byte
}

// Encode this concatenated text UDH
func (h ConcatenatedTextUDH) Encode(bytes []byte, bits int) ([]byte, int) {
	headerLengthIndex := len(bytes)
	bytes = append(bytes, 0)
	bits += 8

	bytes = append(bytes, byte(h.ElementID))
	bits += 8

	elementLengthIndex := len(bytes)
	bytes = append(bytes, 0)
	bits += 8

	bytes = append(bytes, byte(h.MessageReference))
	bits += 8
	if h.ElementID == ConcatenatedTextMessageWithLongReference {
		bytes = append(bytes, byte(h.MessageReference>>8))
		bits += 8
	}

	bytes = append(bytes, h.TotalNumber)
	bits += 8
	bytes = append(bytes, h.SequenceNumber)
	bits += 8

	bytes[headerLengthIndex] = byte(len(bytes) - headerLengthIndex - 1)
	bytes[elementLengthIndex] = byte(len(bytes) - elementLengthIndex - 1)

	return bytes, bits
}

// Length returns the length of this header in bytes.
func (h ConcatenatedTextUDH) Length() int {
	result := 6
	if h.ElementID == ConcatenatedTextMessageWithLongReference {
		result++
	}

	return result
}

// UDHInformationElementID enum according to [AI] 29.5.9.4.1
type UDHInformationElementID byte

// The relevant UDHInformationElementID values for concatenated text according to [AI] table 29.47.
const (
	ConcatenatedTextMessageWithShortReference UDHInformationElementID = 0x00
	ConcatenatedTextMessageWithLongReference  UDHInformationElementID = 0x08
)

/* Concatenated SDS messaging related types and functions */

func ParseConcatenatedSDSMessageSDU(bytes []byte) (ConcatenatedSDSMessageSDU, error) {
	/*
		Parses the structure of a Concatenated SDS message (PID 0x8C) :

		1. Concatenation Control Header (4 bits)
			Bits:   7   6   5  | 4 |  3   2   1   0
				----------- --- ---------------
				PDU Type     | Reference Extension Present

			- PDU Type (bits 7–5): Must be 0b000 (Concatenation Transfer)
			- Reference Extension Present (bit 4):
				- 0 = only short reference used (4 bits total)
				- 1 = extension present (12-bit reference total)

			the next 4 bits (bits 3-0) will either be the upper 4 bits of the reference
			(if extension is present) or the short reference (if extension is not present)

		2. Concatenation Reference Extension (optional, 8 bits)
			- Present only if Reference Extension Present == 1
			- Contains the upper 8 bits of the 12-bit reference value

		3. Short Reference (4 bits):
			- Always present, even when extension is used
			- Forms the lower 4 bits of the reference number

		4. Total Number of Concatenation Parts (1 byte)
			- Range: 2–255
			- All parts of a single concatenated message share the same reference and total

		5. Sequence Number of Current Part (1 byte)
			- Range: 1–255
			- Indicates the position of this fragment (1 = first)

		6. Payload Protocol Identifier (optional, 1 byte)
			- Present only if Sequence Number == 1
			- Identifies the actual SDS PID of the original (unfragmented) message
			- Not present in parts 2 and onward

		7. Payload Data (remaining bytes)
			- The actual SDS application payload fragment for this message part
			- May be empty (e.g. padding or protocol artifacts)
	*/

	var result ConcatenatedSDSMessageSDU

	offset := 0
	if len(bytes) < offset+1 {
		return result, fmt.Errorf("data too short for concatenation control header")
	}

	ctrlByte := bytes[offset]

	// Extract the PDU type from bits 7–5 (should be 0b000 for "Concatenation Transfer")
	pduType := (ctrlByte & 0xE0) >> 5
	if pduType != 0b000 {
		return result, fmt.Errorf("unsupported PDU type: %03b", pduType)
	}

	// Bit 4 indicates whether the reference extension is present
	refType := (ctrlByte & 0x10) >> 4

	offset++ // move past the control byte

	if refType == 1 {
		// Reference extension is present — need to read the next byte
		if len(bytes) < offset+1 {
			return result, fmt.Errorf("missing reference extension byte")
		}

		extByte := bytes[offset]
		offset++

		// Combine into 12-bit reference:
		// ctrlByte bits 3-0 (upper 4 ext bits) go to positions 11-8
		// extByte bits 7-0 (lower 4 ext bits + short ref) go to positions 7-0
		result.ConcatenationReference = (uint16(ctrlByte&0x0F) << 8) | uint16(extByte)
	} else {
		// No extension — 4-bit reference only from bits 3-0 of control byte
		result.ConcatenationReference = uint16(ctrlByte & 0x0F)
	}

	// TotalNumber and SequenceNumber
	if len(bytes) < offset+2 {
		return result, fmt.Errorf("missing total number or sequence number")
	}
	result.TotalNumber = bytes[offset]
	offset++
	result.SequenceNumber = bytes[offset]
	offset++

	if result.SequenceNumber == 1 {
		if len(bytes) < offset+1 {
			return result, fmt.Errorf("missing payload PID")
		}
		result.PayloadPID = ProtocolIdentifier(bytes[offset])
		offset++
	}

	// PayloadData
	if offset > len(bytes) {
		return result, fmt.Errorf("offset exceeds data length")
	}
	result.PayloadData = bytes[offset:]

	return result, nil
}

// ConcatenatedSDSMessageSDU according to [AI] 29.5.14.12
type ConcatenatedSDSMessageSDU struct {
	ConcatenationReference uint16
	TotalNumber            byte
	SequenceNumber         byte
	PayloadPID             ProtocolIdentifier
	PayloadData            []byte
}

/* Status related types and functions */

// ParseStatus from the given bytes.
func ParseStatus(bytes []byte) (interface{}, error) {
	if len(bytes) < 2 {
		return 0, fmt.Errorf("status value too short: %v", bytes)
	}

	if (bytes[0] & SDSShortReportPDUIdentifier) == SDSShortReportPDUIdentifier {
		return ParseSDSShortReport(bytes)
	}

	var result Status
	result = (Status(bytes[0]) << 8) | Status(bytes[1])

	return result, nil
}

// Status represents a pre-coded status according to [AI] 14.8.34
type Status uint16

// Bytes returns this status as byte slice.
func (s Status) Bytes() []byte {
	return []byte{
		byte(s >> 8),
		byte(s),
	}
}

// Encode this status
func (s Status) Encode(bytes []byte, bits int) ([]byte, int) {
	return append(bytes, s.Bytes()...), bits + 16
}

// Length returns the length of this encoded status in bytes.
func (s Status) Length() int {
	return 2
}

// Some relevant status values
const (
	// requests

	Status0 Status = 0x8002
	Status1 Status = 0x8003
	Status2 Status = 0x8004
	Status3 Status = 0x8005
	Status4 Status = 0x8006
	Status5 Status = 0x8007
	Status6 Status = 0x8008
	Status7 Status = 0x8009
	Status8 Status = 0x800A
	Status9 Status = 0x800B

	// responses

	StatusA Status = 0x80F2
	StatusE Status = 0x80F3
	StatusC Status = 0x80F4
	StatusF Status = 0x80F5
	StatusH Status = 0x80F6
	StatusJ Status = 0x80F7
	StatusL Status = 0x80F8
	StatusP Status = 0x80F9
	Statusd Status = 0x80FC
	Statush Status = 0x80FD
	Statuso Status = 0x80FE
	Statusu Status = 0x80FF
)

// DecodeTimestamp according to [AI] 29.5.4.4
func DecodeTimestamp(bytes []byte) (time.Time, error) {
	if len(bytes) != 3 {
		return time.Now(), fmt.Errorf("a timestamp must be 3 bytes long")
	}

	locations := []*time.Location{time.Local, time.UTC, time.Local, time.Local}
	location := locations[(bytes[0]&0xC0)>>6]
	year := time.Now().Year()
	month := bytes[0] & 0x0F
	day := int((bytes[1] & 0xF8) >> 3)
	hour := int(((bytes[1] & 0x07) << 2) | ((bytes[2] & 0xC0) >> 6))
	minute := int(bytes[2] & 0x3F)

	return time.Date(year, time.Month(month), day, hour, minute, 0, 0, location), nil
}

// EncodeTimestampUTC according to [AI] 29.5.4.4, always using timeframe type UTC
func EncodeTimestampUTC(timestamp time.Time) []byte {
	result := make([]byte, 3)
	utc := timestamp.UTC()

	result[0] = 0x40 // always use timeframe type UTC
	result[0] |= byte(utc.Month()) & 0x0F
	result[1] = (byte(utc.Day()) << 3) & 0xF8
	result[1] |= (byte(utc.Hour()) >> 2) & 0x07
	result[2] = (byte(utc.Hour()) << 6) & 0xC0
	result[2] |= byte(utc.Minute()) & 0x3F

	return result
}

/* Callout related types and functions */

// Simple Callout Service as defined in TTR001-21 V2.1.1 (2014-11):
// TETRA Interoperability Profile (TIP) Part 21: Callout
func ParseCalloutSDU(bytes []byte) (CalloutAlert, error) {
	/*
		[TLV: 0x0D + packed callout number + priority]
		               └─ Packed byte: [Length:4bits][CalloutMSB:4bits] + remaining bytes
		[Sender Address: 2 bytes]
		[Receiver Count: 1 byte]
		[Receiver Addresses: 2 bytes each]
		[Separator: 0xFF]
		[Text: remaining bytes, ISO8859-1]
	*/

	if len(bytes) < 4 { // Minimum size: PID(2) + GroupControl(2)
		return CalloutAlert{}, fmt.Errorf("callout alert PDU too short: %d bytes", len(bytes))
	}

	var result CalloutAlert
	offset := 0

	// Parse TLV fields
	tlvLoop:
	for offset < len(bytes) {
		if offset >= len(bytes) {
			break
		}

		// Check type field (8 bits = 1 byte)
		typeField := bytes[offset]

		switch typeField {
		case 0x0D: // Callout Number follows
			offset += 1 // Skip type field
			if offset >= len(bytes) {
				return CalloutAlert{}, fmt.Errorf("insufficient bytes for packed fields")
			}

			// Parse packed byte containing length (upper 4 bits) and first part of callout number (lower 4 bits)
			packedByte := bytes[offset]
			lengthInBytes := int(packedByte >> 4)

			if offset + lengthInBytes >= len(bytes) {
				return CalloutAlert{}, fmt.Errorf("insufficient bytes for callout number and priority")
			}

			// Convert the callout number bytes to a single integer
			var calloutNumber uint32 = 0

			// First byte: constructed from packed byte and next byte
			firstByte := (packedByte & 0x0F) << 4 | (bytes[offset+1] >> 4)
			calloutNumber = uint32(firstByte)

			// Add remaining bytes (if any) to build the complete integer
			for i := 1; i < lengthInBytes; i++ {
				calloutNumber = (calloutNumber << 8) | uint32(bytes[offset+i+1])
			}

			result.CalloutNumber = calloutNumber

			// Priority is in the lower 4 bits after the complete callout number
			priorityByteOffset := offset + lengthInBytes
			result.Priority = bytes[priorityByteOffset] & 0x0F

			offset = priorityByteOffset + 1

		default:
			break tlvLoop // Not a known TLV type, assume we've reached the fixed fields
		}
	}

	// Parse remaining fixed fields: SenderSubAddress + ReceiverSubAddrLen + ReceiverSubAddresses + Text
	if offset+3 > len(bytes) { // SenderAddr(2) + RecvAddrLen(1)
		return CalloutAlert{}, fmt.Errorf("insufficient bytes for fixed fields")
	}

	// Parse Sender Sub Address (2 bytes)
	result.SenderSubAddress = uint16((uint32(bytes[offset]) << 8) | uint32(bytes[offset+1]))
	offset += 2

	// Parse Receiver Sub Address Length (1 byte) - number of bytes for receiver addresses
	ReceiverSubAddrLen := uint8(bytes[offset])
	offset += 1

	// Calculate how many bytes the receiver sub-addresses take up
	receiverSubAddrBytes := int(ReceiverSubAddrLen) // Direct byte count

	if len(bytes) < offset+receiverSubAddrBytes {
		return result, fmt.Errorf("callout alert PDU too short for receiver sub-addresses: %d bytes", len(bytes))
	}

	// Parse Receiver Sub Addresses (2 bytes each)
	numAddresses := receiverSubAddrBytes / 2
	result.ReceiverSubAddresses = make([]SubAddress, numAddresses)
	for i := 0; i < numAddresses; i++ {
		addrOffset := offset + (i * 2)
		result.ReceiverSubAddresses[i] = SubAddress((uint16(bytes[addrOffset]) << 8) | uint16(bytes[addrOffset+1]))
	}
	offset += receiverSubAddrBytes

	// Skip separator (1 byte - should be 0xFF) - we don't store it, just verify it's there
	if len(bytes) < offset+1 {
		return CalloutAlert{}, fmt.Errorf("callout alert PDU missing separator")
	}
	separator := bytes[offset]
	if separator != 0xFF {
		return CalloutAlert{}, fmt.Errorf("invalid separator: expected 0xFF, got 0x%02X", separator)
	}
	offset += 1

	// Parse Text (rest of the message)
	if offset < len(bytes) {
		// For simple callout messages, encoding is fixed to Latin-1
		text, err := DecodePayloadText(ISO8859_1, bytes[offset:])
		if err != nil {
			return CalloutAlert{}, fmt.Errorf("failed to decode callout text: %w", err)
		}
		result.Text = text
	}

	return result, nil
}

type SubAddress uint16

// CalloutAlert represents a callout alert PDU (PID 0xC3) for paging sub-addresses with text display
type CalloutAlert struct {
	CalloutNumber        uint32
	Priority             uint8
	SenderSubAddress     uint16
	ReceiverSubAddresses []SubAddress
	Text                 string
}
