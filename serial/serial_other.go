//go:build !linux

package serial

func ListDevices() ([]SerialDevice, error) {
	// no-op for other OSes
	return []SerialDevice{}, nil
}

func FindRadioPortName() (string, error) {
	// no-op for other OSes
	return "", NoPEIFound
}
