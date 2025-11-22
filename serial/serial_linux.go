//go:build linux

package serial

import (
	"strings"

	"github.com/hedhyw/Go-Serial-Detector/pkg/v1/serialdet"
)

func FindRadioPortName() (string, error) {
	devices, err := serialdet.List()
	if err != nil {
		return "", err
	}

	for _, device := range devices {
		description := strings.ToLower(device.Description())
		if strings.Contains(description, "tetra_pei_interface") {
			return device.Path(), nil
		}
	}

	return "", NoPEIFound
}
