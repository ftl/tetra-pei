package ctrl

import (
	"fmt"
	"strings"
)

// AIModeByName returns the AIMode with the given name
func AIModeByName(name string) (AIMode, error) {
	sanitized := strings.ToUpper(strings.TrimSpace(name))
	result, ok := AIModesByName[sanitized]
	if !ok {
		return 0, fmt.Errorf("invalid operating mode %s", name)
	}
	return result, nil
}

// AIMode represents an operating mode according to [PEI] 6.17.4
type AIMode byte

func (m AIMode) String() string {
	for k, v := range AIModesByName {
		if v == m {
			return k
		}
	}
	return "UNKNOWN"
}

// All supported operating modes
const (
	TMO AIMode = iota
	DMO
)

// AIModesByName maps all supported operating modes by their string representation
var AIModesByName = map[string]AIMode{
	"TMO": TMO,
	"DMO": DMO,
}
