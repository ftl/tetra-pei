package ctrl

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ftl/tetra-pei/tetra"
)

// SetOperatingMode according to [PEI] 6.14.7.2
func SetOperatingMode(mode AIMode) string {
	return fmt.Sprintf("AT+CTOM=%d", mode)
}

var requestOperatingModeResponse = regexp.MustCompile(`^\+CTOM: (\d+)$`)

// RequestOperatingMode reads the current operating mode according to [PEI] 6.14.7.4
func RequestOperatingMode(ctx context.Context, requester tetra.Requester) (AIMode, error) {
	responses, err := requester.Request(ctx, "AT+CTOM?")
	if err != nil {
		return 0, err
	}
	if len(responses) < 1 {
		return 0, fmt.Errorf("no response received")
	}
	response := strings.ToUpper(strings.TrimSpace(responses[0]))
	parts := requestOperatingModeResponse.FindStringSubmatch(response)

	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected response: %s", responses[0])
	}

	result, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}

	return AIMode(result), nil
}

// SetTalkgroup according to [PEI] 6.15.6.2
func SetTalkgroup(gtsi string) string {
	return fmt.Sprintf("AT+CTGS=1,%s", gtsi)
}

var requestTalkgroupResponse = regexp.MustCompile(`^\+CTGS: .*,(\d+)$`)

// RequestTalkgroup reads the current talkgroup according to [PEI] 6.15.6.4
func RequestTalkgroup(ctx context.Context, requester tetra.Requester) (string, error) {
	responses, err := requester.Request(ctx, "AT+CTGS?")
	if err != nil {
		return "", err
	}
	if len(responses) < 1 {
		return "", fmt.Errorf("no response received")
	}
	response := strings.ToUpper(strings.TrimSpace(responses[0]))
	parts := requestTalkgroupResponse.FindStringSubmatch(response)

	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected response: %s", responses[0])
	}

	return parts[1], nil
}
