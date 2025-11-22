package serial

import (
	"errors"
	"io"

	"github.com/jacobsa/go-serial/serial"

	"github.com/ftl/tetra-pei/com"
)

var (
	NoPEIFound = errors.New("no active PEI device found")
)

type SerialDevice struct {
	Description string
	Filename    string
}

func Open(portName string) (*com.COM, error) {
	device, err := openSerial(portName)
	if err != nil {
		return nil, err
	}

	return com.New(device), nil
}

func OpenWithTrace(portName string, tracePEIWriter io.Writer) (*com.COM, error) {
	device, err := openSerial(portName)
	if err != nil {
		return nil, err
	}

	return com.NewWithTrace(device, tracePEIWriter), nil
}

func openSerial(portName string) (io.ReadWriteCloser, error) {
	portConfig := serial.OpenOptions{
		PortName:              portName,
		BaudRate:              38400,
		DataBits:              8,
		StopBits:              1,
		ParityMode:            serial.PARITY_NONE,
		RTSCTSFlowControl:     true,
		MinimumReadSize:       4,
		InterCharacterTimeout: 100,
	}

	return serial.Open(portConfig)
}
