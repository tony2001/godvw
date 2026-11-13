package dvw

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ScoutFieldCount is the standard number of semicolon-separated cells in a
// DataVolley scouting row, including the trailing empty cell.
const ScoutFieldCount = 27

const (
	scoutCode = iota
	scoutPointPhase
	scoutAttackPhase
	scoutReserved1
	scoutStartCoordinate
	scoutMidCoordinate
	scoutEndCoordinate
	scoutTime
	scoutSetNumber
	scoutHomeSetterPosition
	scoutVisitingSetterPosition
	scoutVideoFileNumber
	scoutVideoTime
	scoutReserved2
	scoutHomePlayer1
	scoutVisitingPlayer1 = scoutHomePlayer1 + 6
)

// ScoutRow represents the standard 27-cell [3SCOUT] record layout. Its cells
// remain strings so unrecognized values can round-trip without interpretation.
type ScoutRow [ScoutFieldCount]string

// ScoutRecord exposes the standard DataVolley [3SCOUT] columns. Code and
// player values remain strings because DVW permits special and nonnumeric
// values.
type ScoutRecord struct {
	Code                   string
	PointPhase             string
	AttackPhase            string
	Reserved1              string
	StartCoordinate        string
	MidCoordinate          string
	EndCoordinate          string
	Time                   string
	SetNumber              string
	HomeSetterPosition     string
	VisitingSetterPosition string
	VideoFileNumber        string
	VideoTime              string
	Reserved2              string
	HomePlayers            [6]string
	VisitingPlayers        [6]string
	TrailingField          string
}

// ParseScoutRow parses one [3SCOUT] data row. It strips trailing CR/LF
// characters, pads short rows with empty cells, and rejects rows with more
// than ScoutFieldCount cells.
func ParseScoutRow(line string) (ScoutRow, error) {
	var row ScoutRow
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return row, errors.New("scout row is empty")
	}
	cells := strings.Split(line, ";")
	if len(cells) > ScoutFieldCount {
		return row, fmt.Errorf("scout row has %d fields; maximum is %d", len(cells), ScoutFieldCount)
	}
	copy(row[:], cells)
	return row, nil
}

// String serializes the row as a semicolon-delimited DVW line, preserving all
// cells including the trailing empty cell.
func (r ScoutRow) String() string { return strings.Join(r[:], ";") }

// Record converts the positional cells to their named ScoutRecord fields.
func (r ScoutRow) Record() ScoutRecord {
	x := ScoutRecord{
		Code:                   r[scoutCode],
		PointPhase:             r[scoutPointPhase],
		AttackPhase:            r[scoutAttackPhase],
		Reserved1:              r[scoutReserved1],
		StartCoordinate:        r[scoutStartCoordinate],
		MidCoordinate:          r[scoutMidCoordinate],
		EndCoordinate:          r[scoutEndCoordinate],
		Time:                   r[scoutTime],
		SetNumber:              r[scoutSetNumber],
		HomeSetterPosition:     r[scoutHomeSetterPosition],
		VisitingSetterPosition: r[scoutVisitingSetterPosition],
		VideoFileNumber:        r[scoutVideoFileNumber],
		VideoTime:              r[scoutVideoTime],
		Reserved2:              r[scoutReserved2],
		TrailingField:          r[ScoutFieldCount-1],
	}
	copy(x.HomePlayers[:], r[scoutHomePlayer1:scoutVisitingPlayer1])
	copy(x.VisitingPlayers[:], r[scoutVisitingPlayer1:ScoutFieldCount-1])
	return x
}

// Row converts the named fields back to the standard 27-cell layout. The
// reserved cells and trailing cell map to Reserved1, Reserved2, and
// TrailingField.
func (x ScoutRecord) Row() ScoutRow {
	var r ScoutRow
	r[scoutCode] = x.Code
	r[scoutPointPhase] = x.PointPhase
	r[scoutAttackPhase] = x.AttackPhase
	r[scoutReserved1] = x.Reserved1
	r[scoutStartCoordinate] = x.StartCoordinate
	r[scoutMidCoordinate] = x.MidCoordinate
	r[scoutEndCoordinate] = x.EndCoordinate
	r[scoutTime] = x.Time
	r[scoutSetNumber] = x.SetNumber
	r[scoutHomeSetterPosition] = x.HomeSetterPosition
	r[scoutVisitingSetterPosition] = x.VisitingSetterPosition
	r[scoutVideoFileNumber] = x.VideoFileNumber
	r[scoutVideoTime] = x.VideoTime
	r[scoutReserved2] = x.Reserved2
	r[ScoutFieldCount-1] = x.TrailingField
	copy(r[scoutHomePlayer1:scoutVisitingPlayer1], x.HomePlayers[:])
	copy(r[scoutVisitingPlayer1:ScoutFieldCount-1], x.VisitingPlayers[:])
	return r
}

// ParseScoutRecord parses one [3SCOUT] data row into named fields. It pads
// short rows, rejects rows with more than ScoutFieldCount cells, and does not
// call Validate.
func ParseScoutRecord(line string) (ScoutRecord, error) {
	r, err := ParseScoutRow(line)
	if err != nil {
		return ScoutRecord{}, err
	}
	return r.Record(), nil
}

// Validate checks row structure and scalar conventions, and reports
// non-fatal scouting-code syntax issues using the default code conventions.
// Optional phase, time, coordinate, and setter fields may be empty.
func (x ScoutRecord) Validate() error {
	return x.ValidateWithOptions(ScoutCodeOptions{})
}

// ValidateWithOptions checks row structure and scalar conventions, and
// validates the code using the requested scouting conventions.
func (x ScoutRecord) ValidateWithOptions(options ScoutCodeOptions) error {
	if x.Code == "" {
		return errors.New("scouting code is empty")
	}
	if x.Reserved1 != "" || x.Reserved2 != "" {
		return errors.New("reserved scouting row fields must be empty")
	}
	if x.TrailingField != "" {
		return errors.New("scouting row must end with an empty field")
	}
	parsedCode, err := ParseScoutCodeWithOptions(x.Code, options)
	if err != nil {
		return err
	}
	if len(parsedCode.Issues) != 0 {
		issues := make([]error, 0, len(parsedCode.Issues))
		for _, issue := range parsedCode.Issues {
			issues = append(issues, fmt.Errorf("scouting code: %s", issue.Message))
		}
		return errors.Join(issues...)
	}
	switch x.PointPhase {
	case "", "p", "s":
	default:
		return fmt.Errorf("invalid point phase %q", x.PointPhase)
	}
	switch x.AttackPhase {
	case "", "p", "s", "r":
	default:
		return fmt.Errorf("invalid attack phase %q", x.AttackPhase)
	}
	if x.Time != "" {
		if _, err := time.Parse("15.04.05", x.Time); err != nil {
			return fmt.Errorf("invalid time %q; want HH.MM.SS", x.Time)
		}
	}
	coordinates := []struct {
		name  string
		value string
	}{
		{name: "start coordinate", value: x.StartCoordinate},
		{name: "middle coordinate", value: x.MidCoordinate},
		{name: "end coordinate", value: x.EndCoordinate},
	}
	for _, coordinate := range coordinates {
		if coordinate.value == "" || coordinate.value == "-1-1" {
			continue
		}
		hasFourDigits := len(coordinate.value) == 4 && digits(coordinate.value)
		if !hasFourDigits {
			return fmt.Errorf("invalid %s %q; want four digits or -1-1", coordinate.name, coordinate.value)
		}
	}
	for _, value := range []string{x.HomeSetterPosition, x.VisitingSetterPosition} {
		if value == "" {
			continue
		}
		position, err := strconv.Atoi(value)
		if err != nil || position < 0 || position > 6 {
			return fmt.Errorf("invalid setter position %q", value)
		}
	}
	return nil
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FormatCoordinate formats a coordinate as four zero-padded digits. It
// returns an error when n is outside 0 through 9999. Use -1-1 in a record to
// represent a missing point when another coordinate is present.
func FormatCoordinate(n int) (string, error) {
	if n < 0 || n > 9999 {
		return "", fmt.Errorf("coordinate %d is outside 0..9999", n)
	}
	return fmt.Sprintf("%04d", n), nil
}

// DecodeDVUTF8 decodes DataVolley's embedded Unicode form. A leading 0x0f
// marker must be followed by 2 or 4, which sets the number of hex digits per
// code point. Unmarked text is returned unchanged. Invalid markers, payloads,
// and Unicode scalar values return an error.
func DecodeDVUTF8(s string) (string, error) {
	if !strings.HasPrefix(s, "\x0f") {
		return s, nil
	}
	if len(s) < 2 {
		return "", errors.New("incomplete DataVolley Unicode marker")
	}
	width := 0
	switch s[1] {
	case '2':
		width = 2
	case '4':
		width = 4
	default:
		return "", fmt.Errorf("unknown DataVolley Unicode width %q", s[1])
	}
	token := s[2:]
	if token == "" || len(token)%width != 0 {
		return "", errors.New("invalid DataVolley Unicode payload length")
	}
	var out strings.Builder
	for i := 0; i < len(token); i += width {
		codepoint, err := strconv.ParseUint(token[i:i+width], 16, 16)
		if err != nil || !validUnicodeScalar(rune(codepoint)) {
			return "", fmt.Errorf("invalid DataVolley Unicode code point %q", token[i:i+width])
		}
		out.WriteRune(rune(codepoint))
	}
	return out.String(), nil
}

// EncodeDVUTF8 encodes text using DataVolley's embedded Unicode form. It uses
// two hex digits per code point when possible and four otherwise. Invalid
// UTF-8 and supplementary Unicode code points return an error.
func EncodeDVUTF8(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", errors.New("input is not valid UTF-8")
	}
	if s == "" {
		return "", nil
	}
	width := 2
	for _, r := range s {
		if r > 0xffff {
			return "", fmt.Errorf("DataVolley Unicode encoding does not support code point U+%X", r)
		}
		if r > 0xff {
			width = 4
		}
	}
	var out strings.Builder
	out.WriteByte(0x0f)
	out.WriteByte(byte('0' + width))
	for _, r := range s {
		fmt.Fprintf(&out, "%0*X", width, r)
	}
	return out.String(), nil
}

func validUnicodeScalar(r rune) bool {
	return r >= 0 && r <= utf8.MaxRune && (r < 0xd800 || r > 0xdfff)
}
