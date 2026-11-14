// Package dvw reads and writes DataVolley DVW files while preserving section
// order, content bytes, and unrecognized sections.
package dvw

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Standard DVW section names.
const (
	SectionDataVolleyScout   = "3DATAVOLLEYSCOUT"
	SectionMatch             = "3MATCH"
	SectionTeams             = "3TEAMS"
	SectionMore              = "3MORE"
	SectionComments          = "3COMMENTS"
	SectionSet               = "3SET"
	SectionPlayersHome       = "3PLAYERS-H"
	SectionPlayersVisiting   = "3PLAYERS-V"
	SectionAttackCombination = "3ATTACKCOMBINATION"
	SectionSetterCall        = "3SETTERCALL"
	SectionWinningSymbols    = "3WINNINGSYMBOLS"
	SectionReserve           = "3RESERVE"
	SectionVideo             = "3VIDEO"
	SectionScout             = "3SCOUT"
)

// LineEnding is a line terminator supported by DVW text files.
type LineEnding string

const (
	// LineEndingLF is a line feed terminator.
	LineEndingLF LineEnding = "\n"
	// LineEndingCRLF is a carriage return followed by a line feed.
	LineEndingCRLF LineEnding = "\r\n"
)

// Line stores text separately from its line terminator. An empty Ending uses
// the file's default newline when the line is serialized.
type Line struct {
	// Text contains the line content without its terminator.
	Text string
	// Ending is the original terminator, or empty to use the file default.
	Ending LineEnding
}

// File is an ordered collection of DVW sections and any lines before the first
// section. Strings in the file are byte-preserving; no character decoding is
// performed.
type File struct {
	// Preamble contains lines before the first section.
	Preamble []Line
	// Sections contains the document's sections in file order.
	Sections []*Section
	// BOM controls whether Marshal writes a UTF-8 byte-order mark.
	BOM bool
	// Newline is the default terminator for lines without an Ending value.
	Newline LineEnding
}

var _ io.WriterTo = (*File)(nil)

// Section is a named DVW section. Each line stores its content and terminator
// separately so callers never need to handle carriage returns themselves.
type Section struct {
	// Name is the section name without the surrounding brackets.
	Name string
	// Lines contains the section's data lines, excluding its header.
	Lines        []Line
	headerEnding LineEnding
}

// SectionLineError identifies the one-based line within a section where
// parsing failed. It unwraps to the underlying parse error.
type SectionLineError struct {
	// Section is the section where parsing failed.
	Section string
	// Line is the one-based line number within Section.
	Line int
	// Err is the parse error that occurred on that line.
	Err error
}

// Error formats the section name, line number, and underlying parse error.
func (e *SectionLineError) Error() string {
	return fmt.Sprintf("section %q, line %d: %v", e.Section, e.Line, e.Err)
}

// Unwrap returns the parse error so errors.Is and errors.As can inspect it.
func (e *SectionLineError) Unwrap() error { return e.Err }

// NewFile creates an indoor DVW skeleton with the standard section order and
// default [3MATCH] rows. Populate team, roster, and set metadata before using
// the file in DataVolley software.
func NewFile() *File {
	names := []string{
		SectionDataVolleyScout,
		SectionMatch,
		SectionTeams,
		SectionMore,
		SectionComments,
		SectionSet,
		SectionPlayersHome,
		SectionPlayersVisiting,
		SectionAttackCombination,
		SectionSetterCall,
		SectionWinningSymbols,
		SectionReserve,
		SectionVideo,
		SectionScout,
	}
	f := &File{}
	for _, name := range names {
		f.AddSection(name)
	}
	f.Section(SectionDataVolleyScout).Lines = []Line{{Text: "FILEFORMAT: 2.0"}}
	f.Section(SectionMatch).Lines = []Line{{Text: ";;;;;;;;UTF-8;1;Z;0;"}, {Text: ";;12345;;;;;;"}}
	return f
}

// Read parses a DVW document from r without imposing a line-size limit. It
// returns an error if reading from r fails.
func Read(r io.Reader) (*File, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Parse parses a DVW document and preserves section order, unknown content,
// the UTF-8 BOM, and line endings present in the source. It does not validate
// section contents or scouting codes.
func Parse(data []byte) (*File, error) {
	var bom bool
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		bom = true
		data = data[3:]
	}
	lines := splitLines(data)
	f := &File{BOM: bom, Newline: detectNewline(lines)}
	var current *Section
	for _, line := range lines {
		if name, ok := sectionName(line.Text); ok {
			current = &Section{Name: name, headerEnding: line.Ending}
			f.Sections = append(f.Sections, current)
			continue
		}
		if current == nil {
			f.Preamble = append(f.Preamble, line)
		} else {
			current.Lines = append(current.Lines, line)
		}
	}
	return f, nil
}

func detectNewline(lines []Line) LineEnding {
	for _, line := range lines {
		if line.Ending != "" {
			return line.Ending
		}
	}
	return LineEndingLF
}

func splitLines(data []byte) []Line {
	var lines []Line
	for len(data) > 0 {
		newline := bytes.IndexByte(data, '\n')
		if newline < 0 {
			lines = append(lines, Line{Text: string(data)})
			break
		}

		text := data[:newline]
		ending := LineEndingLF
		if len(text) > 0 && text[len(text)-1] == '\r' {
			text = text[:len(text)-1]
			ending = LineEndingCRLF
		}
		lines = append(lines, Line{Text: string(text), Ending: ending})
		data = data[newline+1:]
	}
	return lines
}

func sectionName(line string) (string, bool) {
	if len(line) < 3 || line[0] != '[' || line[len(line)-1] != ']' {
		return "", false
	}
	name := line[1 : len(line)-1]
	if strings.TrimSpace(name) != name || name == "" || strings.ContainsAny(name, "[]") {
		return "", false
	}
	return name, true
}

// Marshal serializes the document in section order and always ends a nonempty
// document with a line terminator. Existing terminators are preserved; lines
// with no terminator use the first line-ending style found in the source. It
// returns an error for a nil file, nil sections, invalid section names, or
// unsupported line endings.
func (f *File) Marshal() ([]byte, error) {
	if f == nil {
		return nil, errors.New("nil DVW file")
	}
	lines := append([]Line(nil), f.Preamble...)
	for _, s := range f.Sections {
		if s == nil {
			return nil, errors.New("nil section")
		}
		if _, ok := sectionName("[" + s.Name + "]"); !ok {
			return nil, fmt.Errorf("invalid section name %q", s.Name)
		}
		header := Line{Text: "[" + s.Name + "]", Ending: s.headerEnding}
		lines = append(lines, header)
		lines = append(lines, s.Lines...)
	}
	sep := f.Newline
	if sep == "" {
		sep = LineEndingLF
	}
	if !validLineEnding(sep) {
		return nil, fmt.Errorf("invalid line ending %q", sep)
	}
	var b bytes.Buffer
	if f.BOM {
		b.Write([]byte{0xef, 0xbb, 0xbf})
	}
	for _, line := range lines {
		ending := line.Ending
		if ending == "" {
			ending = sep
		}
		if !validLineEnding(ending) {
			return nil, fmt.Errorf("invalid line ending %q", ending)
		}
		b.WriteString(line.Text)
		b.WriteString(string(ending))
	}
	return b.Bytes(), nil
}

func validLineEnding(ending LineEnding) bool {
	return ending == LineEndingLF || ending == LineEndingCRLF
}

// Write serializes f to w. It returns serialization errors and errors from w,
// including io.ErrShortWrite when w writes fewer bytes than requested.
func (f *File) Write(w io.Writer) error {
	_, err := f.WriteTo(w)
	return err
}

// Section returns the first section with a case-insensitive name match, or nil
// if the file does not contain that section.
func (f *File) Section(name string) *Section {
	if f == nil {
		return nil
	}
	for _, s := range f.Sections {
		if s != nil && strings.EqualFold(s.Name, name) {
			return s
		}
	}
	return nil
}

// ScoutCodeOptions builds decoder options with attack and setter-call
// descriptions from this file's metadata sections. Missing sections are
// represented by nil maps, which disables lookup for that code category.
func (f *File) ScoutCodeOptions() ScoutCodeOptions {
	var options ScoutCodeOptions
	if f == nil {
		return options
	}
	if section := f.Section(SectionAttackCombination); section != nil {
		options.AttackCombinations = make(map[string]string)
		for _, row := range section.Rows() {
			if len(row) > 4 && row[0] != "" {
				options.AttackCombinations[row[0]] = row[4]
			}
		}
	}
	if section := f.Section(SectionSetterCall); section != nil {
		options.SetterCalls = make(map[string]string)
		for _, row := range section.Rows() {
			if len(row) > 2 && row[0] != "" {
				options.SetterCalls[row[0]] = row[2]
			}
		}
	}
	return options
}

// AddSection appends a section with name and returns it for population. The
// name is checked by Marshal, so invalid names cause serialization to fail.
func (f *File) AddSection(name string) *Section {
	s := &Section{Name: name}
	f.Sections = append(f.Sections, s)
	return s
}

// Rows splits each data line into semicolon-delimited cells. Trailing empty
// cells are preserved. Lines without semicolons return one-cell rows.
func (s *Section) Rows() [][]string {
	if s == nil {
		return nil
	}
	rows := make([][]string, 0, len(s.Lines))
	for _, line := range s.Lines {
		rows = append(rows, strings.Split(line.Text, ";"))
	}
	return rows
}

// AppendRow appends cells as a semicolon-delimited row. Empty final cells are
// retained. It returns an error if any cell contains a semicolon or line break.
func (s *Section) AppendRow(row []string) error {
	if s == nil {
		return errors.New("cannot append a row to a nil section")
	}
	if err := validateRowCells(row); err != nil {
		return err
	}
	s.Lines = append(s.Lines, Line{Text: strings.Join(row, ";")})
	return nil
}

func validateRowCells(cells []string) error {
	for i, cell := range cells {
		if strings.ContainsAny(cell, ";\r\n") {
			return fmt.Errorf("field %d contains a row delimiter", i+1)
		}
	}
	return nil
}

// ScoutRecords parses each nonempty, non-NUL-padding line in a [3SCOUT]
// section. It does not call ScoutRecord.Validate. The returned
// SectionLineError identifies the one-based section line for malformed rows.
func (s *Section) ScoutRecords() ([]ScoutRecord, error) {
	if s == nil {
		return nil, errors.New("cannot read scouting records from a nil section")
	}
	if !strings.EqualFold(s.Name, SectionScout) {
		return nil, fmt.Errorf("section %q is not %q", s.Name, SectionScout)
	}
	out := make([]ScoutRecord, 0, len(s.Lines))
	for i, line := range s.Lines {
		text := strings.TrimRight(line.Text, "\x00")
		if text == "" {
			continue
		}
		record, err := ParseScoutRecord(text)
		if err != nil {
			return nil, &SectionLineError{Section: s.Name, Line: i + 1, Err: err}
		}
		out = append(out, record)
	}
	return out, nil
}

// SetScoutRecords replaces the section's lines with the supplied records. It
// rejects cells containing row delimiters but does not call Validate.
func (s *Section) SetScoutRecords(records []ScoutRecord) error {
	if s == nil {
		return errors.New("cannot set scouting records on a nil section")
	}
	if !strings.EqualFold(s.Name, SectionScout) {
		return fmt.Errorf("section %q is not %q", s.Name, SectionScout)
	}
	lines := make([]Line, len(records))
	for i, record := range records {
		row := record.Row()
		if err := validateRowCells(row[:]); err != nil {
			return fmt.Errorf("scouting record %d: %w", i+1, err)
		}
		lines[i] = Line{Text: row.String()}
	}
	s.Lines = lines
	return nil
}

// MatchMetadata contains commonly used fields from a [3MATCH] metadata row.
// Fields retains the complete row so updates can preserve columns this
// package does not interpret.
type MatchMetadata struct {
	Date         string
	Time         string
	Season       string
	League       string
	Venue        string
	HomeAway     string
	TextEncoding string
	Regulation   string
	ZonesOrCones string
	// Fields contains the complete source row, including fields not exposed above.
	Fields []string
}

// MatchMetadata returns the first [3MATCH] row with at least 12 cells. Fields
// contains all cells in that row. It returns an error if the section or a
// metadata row is missing.
func (f *File) MatchMetadata() (MatchMetadata, error) {
	s := f.Section(SectionMatch)
	if s == nil {
		return MatchMetadata{}, fmt.Errorf("missing %q section", SectionMatch)
	}
	for _, line := range s.Lines {
		cells := strings.Split(line.Text, ";")
		if len(cells) < 12 {
			continue
		}
		m := MatchMetadata{Fields: cells}
		m.Date, m.Time, m.Season, m.League = cells[0], cells[1], cells[2], cells[3]
		m.Venue, m.HomeAway, m.TextEncoding, m.Regulation, m.ZonesOrCones = cells[4], cells[5], cells[8], cells[9], cells[10]
		return m, nil
	}
	return MatchMetadata{}, fmt.Errorf("%q section has no metadata row", SectionMatch)
}

// SetMatchMetadata replaces the first metadata row, retaining unrecognized
// cells from m.Fields. If Fields is too short, missing cells are added. It
// returns an error if [3MATCH] is absent or a cell contains a row delimiter.
func (f *File) SetMatchMetadata(m MatchMetadata) error {
	s := f.Section(SectionMatch)
	if s == nil {
		return fmt.Errorf("missing %q section", SectionMatch)
	}
	cells := append([]string(nil), m.Fields...)
	if len(cells) < 13 {
		cells = append(cells, make([]string, 13-len(cells))...)
	}
	cells[0], cells[1], cells[2], cells[3] = m.Date, m.Time, m.Season, m.League
	cells[4], cells[5], cells[8], cells[9], cells[10] = m.Venue, m.HomeAway, m.TextEncoding, m.Regulation, m.ZonesOrCones
	if err := validateRowCells(cells); err != nil {
		return fmt.Errorf("%q metadata: %w", SectionMatch, err)
	}
	for i, line := range s.Lines {
		if len(strings.Split(line.Text, ";")) >= 12 {
			s.Lines[i].Text = strings.Join(cells, ";")
			return nil
		}
	}
	s.Lines = append([]Line{{Text: strings.Join(cells, ";")}}, s.Lines...)
	return nil
}

// Header returns the value of a KEY: VALUE line with a case-insensitive key
// match. It returns false if no matching line exists.
func (s *Section) Header(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	for _, line := range s.Lines {
		k, v, ok := strings.Cut(line.Text, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// SetHeader replaces a matching KEY: VALUE line or appends one if absent. It
// returns an error if key or value contains a line break, or if key contains
// a colon.
func (s *Section) SetHeader(key, value string) error {
	if s == nil {
		return errors.New("cannot set a header on a nil section")
	}
	if key == "" || strings.ContainsAny(key, ":\r\n") || strings.ContainsAny(value, "\r\n") {
		return errors.New("header key or value contains an invalid delimiter")
	}
	for i, line := range s.Lines {
		k, _, ok := strings.Cut(line.Text, ":")
		if ok && strings.EqualFold(strings.TrimSpace(k), key) {
			s.Lines[i].Text = key + ": " + value
			return nil
		}
	}
	s.Lines = append(s.Lines, Line{Text: key + ": " + value})
	return nil
}

// WriteTo writes the serialized file to w and implements io.WriterTo. It
// returns the number of bytes written and any serialization or writer error,
// including io.ErrShortWrite when w writes fewer bytes without an error.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	b, err := f.Marshal()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return int64(n), err
}
