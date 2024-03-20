package ctrl

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ftl/tetra-pei/tetra"
)

// SetOperatingMode according to [PEI] 6.14.7.2
func SetOperatingMode(mode AIMode) string {
	return fmt.Sprintf("AT+CTOM=%d", mode)
}

const operatingModeRequest = "AT+CTOM?"

var operatingModeResponse = regexp.MustCompile(`^\+CTOM: (\d+)$`)

// RequestOperatingMode reads the current operating mode according to [PEI] 6.14.7.4
func RequestOperatingMode(ctx context.Context, requester tetra.Requester) (AIMode, error) {
	parts, err := requestWithSingleLineResponse(ctx, requester, operatingModeRequest, operatingModeResponse, 2)
	if err != nil {
		return 0, err
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

const talkgroupRequest = "AT+CTGS?"

var talkgroupResponse = regexp.MustCompile(`^\+CTGS: .*,(\d+)$`)

// RequestTalkgroup reads the current talkgroup according to [PEI] 6.15.6.4
func RequestTalkgroup(ctx context.Context, requester tetra.Requester) (string, error) {
	parts, err := requestWithSingleLineResponse(ctx, requester, talkgroupRequest, talkgroupResponse, 2)
	if err != nil {
		return "", err
	}

	return parts[1], nil
}

const (
	talkgroupRangeRequest    = "AT+CNUM%s=?"
	talkgroupsPrepareRequest = "AT+CNUM%s=0,%d,%d"
	talkgroupsReadRequest    = "AT+CNUM%s?"
)

type TalkgroupKind string

const (
	TalkgroupFixed   TalkgroupKind = "F"
	TalkgroupStatic  TalkgroupKind = "S"
	TalkgroupDynamic TalkgroupKind = "D"
)

type TalkgroupRange struct {
	Min int
	Max int
}

type TalkgroupInfo struct {
	GTSI string
	Name string
}

// RequestTalkgroups reads all available static talkgroups from the device, see [PEI] 6.11.5.2
func RequestTalkgroups(ctx context.Context, requester tetra.Requester, kind TalkgroupKind, result []TalkgroupInfo) ([]TalkgroupInfo, error) {
	rng, err := RequestTalkgroupRange(ctx, requester, kind)
	if err != nil {
		return nil, err
	}

	prepareRequest := fmt.Sprintf(talkgroupsPrepareRequest, kind, rng.Min, rng.Max)
	_, err = requester.Request(ctx, prepareRequest)
	if err != nil {
		return nil, err
	}

	readRequest := fmt.Sprintf(talkgroupsReadRequest, kind)
	responses, err := requester.Request(ctx, readRequest)
	if err != nil {
		return nil, err
	}
	if len(responses) < 1 {
		return nil, fmt.Errorf("no response received")
	}

	for _, line := range responses {
		info, err := parseTalkgroupInfo(line)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

var talkgroupInfoLine = regexp.MustCompile(`^(\+CNUM(S|D): )?(\d+),(\d+),(.+)`)

func parseTalkgroupInfo(line string) (TalkgroupInfo, error) {
	parts := talkgroupInfoLine.FindStringSubmatch(line)
	if len(parts) != 6 {
		return TalkgroupInfo{}, fmt.Errorf("invalid talkgroup info: %s", line)
	}
	return TalkgroupInfo{
		GTSI: parts[4],
		Name: parts[5],
	}, nil
}

var talkgroupRangeResponse = regexp.MustCompile(`^\+CNUM(S|D): \(.*\),\((\d+)-(\d+)\),\((\d+)-(\d+)\)`)

func RequestTalkgroupRange(ctx context.Context, requester tetra.Requester, kind TalkgroupKind) (TalkgroupRange, error) {
	cmd := fmt.Sprintf("AT+CNUM%s=?", kind)
	parts, err := requestWithSingleLineResponse(ctx, requester, cmd, talkgroupRangeResponse, 6)
	if err != nil {
		return TalkgroupRange{}, err
	}

	min, err := strconv.Atoi(parts[2])
	if err != nil {
		return TalkgroupRange{}, fmt.Errorf("cannot parse range minimum: %v", err)
	}
	max, err := strconv.Atoi(parts[5])
	if err != nil {
		return TalkgroupRange{}, fmt.Errorf("cannot parse range maximum: %v", err)
	}

	return TalkgroupRange{Min: min, Max: max}, nil
}

const batteryChargeRequest = "AT+CBC?"

var batteryChargeResponse = regexp.MustCompile(`^\+CBC: .*,(\d+)$`)

// RequestBatteryCharge reads the current battery charge status according to [PEI] 6.9
func RequestBatteryCharge(ctx context.Context, requester tetra.Requester) (int, error) {
	parts, err := requestWithSingleLineResponse(ctx, requester, batteryChargeRequest, batteryChargeResponse, 2)
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(parts[1])
}

const signalStrengthRequest = "AT+CSQ?"

var signalStrengthResponse = regexp.MustCompile(`^\+CSQ: (\d+),(\d+)$`)

// RequestSignalStrength reads the current signal strength in dBm according to [PEI] 6.9
func RequestSignalStrength(ctx context.Context, requester tetra.Requester) (int, error) {
	parts, err := requestWithSingleLineResponse(ctx, requester, signalStrengthRequest, signalStrengthResponse, 3)
	if err != nil {
		return 0, err
	}

	value, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid signal strength: %v", err)
	}
	if value == 99 {
		return 0, fmt.Errorf("no signal strength available")
	}

	return -113 + (value * 2), err
}

const gpsPositionRequest = "AT+GPSPOS?"

var gpsPositionResponse = regexp.MustCompile(`^\+GPSPOS: (\d{2}):(\d{2}):(\d{2}),(N|S): (\d{2})_(\d{2}.\d{4}),(W|E): (\d{3})_(\d{2}.\d{4}),(\d+)$`)

// RequestGPSPosition reads the current GPS position, number of satellites, and time in UTC
func RequestGPSPosition(ctx context.Context, requester tetra.Requester) (float64, float64, int, time.Time, error) {
	parts, err := requestWithSingleLineResponse(ctx, requester, gpsPositionRequest, gpsPositionResponse, 11)
	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}

	var (
		hours, minutes, seconds int
		latDegrees, lonDegrees  float64
		latMinutes, lonMinutes  float64
		satellites              int
	)

	if err == nil {
		hours, err = strconv.Atoi(parts[1])
	}
	if err == nil {
		minutes, err = strconv.Atoi(parts[2])
	}
	if err == nil {
		seconds, err = strconv.Atoi(parts[3])
	}

	if err == nil {
		latDegrees, err = strconv.ParseFloat(parts[5], 64)
	}
	if err == nil {
		latMinutes, err = strconv.ParseFloat(parts[6], 64)
	}
	lat := degreesMinutesToDecimalDegrees(parts[4], latDegrees, latMinutes)

	if err == nil {
		lonDegrees, err = strconv.ParseFloat(parts[8], 64)
	}
	if err == nil {
		lonMinutes, err = strconv.ParseFloat(parts[9], 64)
	}
	lon := degreesMinutesToDecimalDegrees(parts[7], lonDegrees, lonMinutes)

	if err == nil {
		satellites, err = strconv.Atoi(parts[10])
	}

	if err != nil {
		return 0, 0, 0, time.Time{}, err
	}

	now := time.Now()
	gpsTime := time.Date(now.Year(), now.Month(), now.Day(), hours, minutes, seconds, 0, time.UTC)

	return lat, lon, satellites, gpsTime, nil
}

func degreesMinutesToDecimalDegrees(direction string, degrees float64, minutes float64) float64 {
	var sign float64
	switch direction {
	case "N", "E":
		sign = 1
	case "S", "W":
		sign = -1
	}

	return sign * (degrees + minutes/60)
}

func requestWithSingleLineResponse(ctx context.Context, requester tetra.Requester, request string, re *regexp.Regexp, partsCount int) ([]string, error) {
	responses, err := requester.Request(ctx, request)
	if err != nil {
		return nil, err
	}
	if len(responses) < 1 {
		return nil, fmt.Errorf("no response received")
	}
	response := strings.ToUpper(strings.TrimSpace(responses[0]))
	parts := re.FindStringSubmatch(response)

	if len(parts) != partsCount {
		return nil, fmt.Errorf("unexpected response: %s", responses[0])
	}

	return parts, nil
}
