//go:build !linux

package serial

func FindRadioPortName() (string, error) {
	// no-op for other OSes
	return "", NoPEIFound
}
