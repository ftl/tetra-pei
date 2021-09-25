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
	headerFields := strings.Split(s[8:], ",")
	switch len(headerFields) {
	case 3, 4: // minimum set
		result.AIService = AIService(strings.TrimSpace(headerFields[0]))
		result.Destination = tetra.Identity(strings.TrimSpace(headerFields[1]))
	case 6, 7: // with source, with end-to-end encryption
		result.AIService = AIService(strings.TrimSpace(headerFields[0]))
		result.Source = tetra.Identity(strings.TrimSpace(headerFields[1]))
		result.Destination = tetra.Identity(strings.TrimSpace(headerFields[3]))
	default:
		return Header{}, fmt.Errorf("invalid header, wrong field count: %s", s)
	}

	pduBitCountField := headerFields[len(headerFields)-1]
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

// All protocol identifiers relevant for SDS handling, according to [AI] table 29.21
const (
	SimpleTextMessaging            ProtocolIdentifier = 0x02
	SimpleImmediateTextMessaging   ProtocolIdentifier = 0x09
	SimpleConcatenatedSDSMessaging ProtocolIdentifier = 0x0C
	TextMessaging                  ProtocolIdentifier = 0x82
	ImmediateTextMessaging         ProtocolIdentifier = 0x89
	UserDataHeaderMessaging        ProtocolIdentifier = 0x8A
	ConcatenatedSDSMessaging       ProtocolIdentifier = 0x8C
)

/* SDS-TL related types and functions */

// ParseSDSTLPDU parses an SDS-TL PDU from the given bytes according to [AI] 29.4.1.
// This function currently supports only a subset of the possible protocol identifiers:
// Simple text messaging (0x02), simple immediate text messaging (0x09), text messaging (0x82),
// immediate text messaging (0x89), message with user data header (0x8A)
func ParseSDSTLPDU(bytes []byte) (interface{}, error) {
	if len(bytes) == 0 {
		return nil, fmt.Errorf("empty payload")
	}

	switch ProtocolIdentifier(bytes[0]) {
	case SimpleTextMessaging, SimpleImmediateTextMessaging:
		return ParseSimpleTextMessage(bytes)
	case TextMessaging, ImmediateTextMessaging, UserDataHeaderMessaging:
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
		// bytes, bits = sdu.Encode(bytes, bits)
	}

	return bytes, bits
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
		8D: Timestamp Used[1] (yes), Text Encoding Scheme[7] (ISO8859-15/Latin 0 ???)
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

// Length returns the length of this header in bytes.
func (h ConcatenatedTextUDH) Length() int {
	return int(h.HeaderLength) + 1 // the HeaderLength byte itself
}

// UDHInformationElementID enum according to [AI] 29.5.9.4.1
type UDHInformationElementID byte

// The relevant UDHInformationElementID values for concatenated text according to [AI] table 29.47.
const (
	ConcatenatedTextMessageWithShortReference UDHInformationElementID = 0x00
	ConcatenatedTextMessageWithLongReference  UDHInformationElementID = 0x08
)

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
