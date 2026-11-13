package dvw

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRoundTripExamples(t *testing.T) {
	files, err := filepath.Glob("testdata/*.dvw")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no DVW testdata found")
	}
	for _, name := range files {
		t.Run(filepath.Base(name), func(t *testing.T) {
			want, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			gotFile, err := Parse(want)
			if err != nil {
				t.Fatal(err)
			}
			if scout := gotFile.Section(SectionScout); scout != nil {
				records, err := scout.ScoutRecords()
				if err != nil {
					t.Fatal(err)
				}
				for i, record := range records {
					if _, err := ParseScoutCode(record.Code); err != nil {
						t.Errorf("scouting record %d: %v", i+1, err)
					}
				}
			}
			got, err := gotFile.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			expected := want
			if len(want) > 0 && want[len(want)-1] != '\n' {
				ending := gotFile.Newline
				if ending == "" {
					ending = LineEndingLF
				}
				expected = append(append([]byte(nil), want...), ending...)
			}
			if !bytes.Equal(expected, got) {
				t.Fatalf("round trip mismatch: got %q, want %q", got, expected)
			}
		})
	}
}

func TestDatavolleyReadBaselines(t *testing.T) {
	// These expected values come from datavolley's test_read.R assertions and
	// dv_read() results for its example_data.dvw, PM06.dvw, and truncated copy.
	cases := []struct {
		name, path, date string
		teams            [][3]string
		plays            int
	}{
		{
			name:  "example_data",
			path:  "testdata/mlafin_braslovce_nkbm.dvw",
			date:  "01/25/2015",
			teams: [][3]string{{"BR4", "Braslovče", "3"}, {"MB4", "Nova KBM Branik", "0"}},
			plays: 1031,
		},
		{
			name:  "PM06",
			path:  "testdata/PM06.dvw",
			date:  "12/28/2012",
			teams: [][3]string{{"67", "ACH Volley", "3"}, {"71", "Maribor", "0"}},
			plays: 701,
		},
		{
			name:  "testing_truncated",
			path:  "testdata/testing_truncated.dvw",
			date:  "01/25/2015",
			teams: [][3]string{{"BR4", "Braslovče", "3"}, {"MB4", "Nova KBM Branik", "0"}},
			plays: 1031,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			match, err := f.MatchMetadata()
			if err != nil {
				t.Fatal(err)
			}
			if match.Date != tc.date {
				t.Errorf("match date = %q, want %q", match.Date, tc.date)
			}
			teamRows := f.Section(SectionTeams).Rows()
			if len(teamRows) != len(tc.teams) {
				t.Fatalf("got %d teams; want %d", len(teamRows), len(tc.teams))
			}
			for i, want := range tc.teams {
				if len(teamRows[i]) < 3 || !slices.Equal(teamRows[i][:3], want[:]) {
					t.Errorf("team %d = %#v, want %#v", i+1, teamRows[i], want)
				}
			}
			playedSets := 0
			for _, row := range f.Section(SectionSet).Rows() {
				if len(row) > 1 && row[1] != "" {
					playedSets++
				}
			}
			if playedSets != 3 {
				t.Errorf("got %d played sets; R baseline has 3", playedSets)
			}
			records, err := f.Section(SectionScout).ScoutRecords()
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != tc.plays {
				t.Errorf("got %d scouting rows; dv_read() has %d plays", len(records), tc.plays)
			}
			if tc.name == "testing_truncated" {
				shortRows := 0
				for _, line := range f.Section(SectionScout).Lines {
					if line.Text != "" && len(strings.Split(line.Text, ";")) < ScoutFieldCount {
						shortRows++
					}
				}
				if shortRows == 0 {
					t.Fatal("truncated fixture has no short scouting rows")
				}
			}
		})
	}
}

func TestDVWriteReadPreservesRWriterSemantics(t *testing.T) {
	// This mirrors test_write.R: change match and match-more metadata, write,
	// read again, and ensure unrelated metadata and every scout row survive.
	for _, path := range []string{"testdata/mlafin_braslovce_nkbm.dvw", "testdata/PM06.dvw"} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			before, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			wantRecords, err := before.Section(SectionScout).ScoutRecords()
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := before.MatchMetadata()
			if err != nil {
				t.Fatal(err)
			}
			metadata.HomeAway = "Home"
			metadata.Fields[6], metadata.Fields[7] = "11", "22"
			if err := before.SetMatchMetadata(metadata); err != nil {
				t.Fatal(err)
			}
			more := before.Section(SectionMore)
			moreCells := strings.Split(more.Lines[0].Text, ";")
			moreCells[1], moreCells[2] = "99", "123"
			more.Lines[0].Text = strings.Join(moreCells, ";")

			written, err := before.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			after, err := Parse(written)
			if err != nil {
				t.Fatal(err)
			}
			gotMetadata, err := after.MatchMetadata()
			if err != nil {
				t.Fatal(err)
			}
			if gotMetadata.HomeAway != "Home" || gotMetadata.Fields[6] != "11" || gotMetadata.Fields[7] != "22" {
				t.Errorf("match metadata edits were lost: %#v", gotMetadata)
			}
			gotMore := after.Section(SectionMore).Rows()
			if gotMore[0][1] != "99" || gotMore[0][2] != "123" {
				t.Errorf("match-more edits were lost: %#v", gotMore[0])
			}
			gotRecords, err := after.Section(SectionScout).ScoutRecords()
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(gotRecords, wantRecords) {
				t.Errorf("scouting rows changed after write/read")
			}
			for i, original := range before.Sections {
				if original.Name == SectionMatch || original.Name == SectionMore {
					continue
				}
				wantLines := append([]Line(nil), original.Lines...)
				if original.Name == SectionScout && len(wantLines) > 0 {
					last := len(wantLines) - 1
					if wantLines[last].Ending == "" {
						wantLines[last].Ending = after.Newline
					}
				}
				if !slices.Equal(wantLines, after.Sections[i].Lines) {
					t.Errorf("section %s changed during write/read", original.Name)
				}
			}
		})
	}
}

func TestParsePreservesPreambleUnknownAndDuplicateSections(t *testing.T) {
	input := []byte("preamble\n[UNKNOWN]\nopaque;data;\n[3SCOUT]\na;b;\n[UNKNOWN]\nsecond\n")
	f, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Preamble) != 1 || f.Preamble[0].Text != "preamble" {
		t.Fatalf("unexpected preamble: %#v", f.Preamble)
	}
	if len(f.Sections) != 3 || f.Sections[0].Name != "UNKNOWN" || f.Sections[2].Name != "UNKNOWN" {
		t.Fatalf("sections/order not preserved: %#v", f.Sections)
	}
	if got, err := f.Marshal(); err != nil || !bytes.Equal(got, input) {
		t.Fatalf("round trip mismatch: got %q, err=%v", got, err)
	}
}

func TestReadParsesReader(t *testing.T) {
	f, err := Read(strings.NewReader("[3MATCH]\n;;;;;;;;UTF-8;1;Z;0;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f.Section(SectionMatch) == nil {
		t.Fatal("Read did not parse the match section")
	}
}

func TestRoundTripMixedLineEndingsAndBareTrailingCR(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "mixed line endings",
			input: []byte("[A]\nvalue\r\n[B]\r\nx\n"),
			want:  []byte("[A]\nvalue\r\n[B]\r\nx\n"),
		},
		{
			name:  "bare trailing carriage return",
			input: []byte("[A]\r\nvalue\r"),
			want:  []byte("[A]\r\nvalue\r\r\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("Marshal() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetadataEditsPreserveExistingLineEndings(t *testing.T) {
	input := []byte("[META]\n" +
		"KEY: old\r\n" +
		"[3MATCH]\n" +
		"01/02/2020;10.20.30;season;league;venue;home;opaque6;opaque7;UTF-8;1;Z;0;\r\n" +
		";;12345;;;;;;\n")
	f, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Section("META").SetHeader("KEY", "new"); err != nil {
		t.Fatal(err)
	}
	m, err := f.MatchMetadata()
	if err != nil {
		t.Fatal(err)
	}
	m.League = "updated"
	if err := f.SetMatchMetadata(m); err != nil {
		t.Fatal(err)
	}
	got, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("[META]\n" +
		"KEY: new\r\n" +
		"[3MATCH]\n" +
		"01/02/2020;10.20.30;season;updated;venue;home;opaque6;opaque7;UTF-8;1;Z;0;\r\n" +
		";;12345;;;;;;\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("metadata edit changed line endings: got %q, want %q", got, want)
	}
}

func TestDecodeDVUTF8(t *testing.T) {
	got, err := DecodeDVUTF8("\x0f2E9")
	if err != nil {
		t.Fatal(err)
	}
	if got != "é" {
		t.Fatalf("got %q", got)
	}
	got, err = DecodeDVUTF8("\x0f40041")
	if err != nil {
		t.Fatal(err)
	}
	if got != "A" {
		t.Fatalf("got %q", got)
	}
	got, err = DecodeDVUTF8("\x0f420AC")
	if err != nil || got != "€" {
		t.Fatalf("four-digit code point decoded to %q, err=%v", got, err)
	}
	if got, err = DecodeDVUTF8("plain\x0f2E9"); err != nil || got != "plain\x0f2E9" {
		t.Fatalf("unmarked text changed to %q, err=%v", got, err)
	}
	for _, malformed := range []string{"\x0f", "\x0f3ABC", "\x0f2", "\x0f2G1", "\x0f4D800", "\x0f4DC00"} {
		if _, err := DecodeDVUTF8(malformed); err == nil {
			t.Errorf("DecodeDVUTF8(%q) unexpectedly succeeded", malformed)
		}
	}
}

func TestScoutRecordFormattingAndValidation(t *testing.T) {
	row := ScoutRecord{
		Code:                   "*01SH+",
		PointPhase:             "p",
		AttackPhase:            "r",
		StartCoordinate:        "0123",
		MidCoordinate:          "-1-1",
		EndCoordinate:          "0456",
		Time:                   "12.34.56",
		SetNumber:              "1",
		HomeSetterPosition:     "5",
		VisitingSetterPosition: "",
	}.Row()
	r, err := ParseScoutRecord(row.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatCoordinate(42)
	if err != nil || formatted != "0042" {
		t.Fatalf("coordinate=%q err=%v", formatted, err)
	}
}

func TestNewFileMatchAndScoutWrite(t *testing.T) {
	f := NewFile()
	m, err := f.MatchMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if m.TextEncoding != "UTF-8" || m.ZonesOrCones != "Z" {
		t.Fatalf("unexpected defaults: %#v", m)
	}
	m.Date, m.Time, m.League = "13/11/2026", "18.30.00", "Test League"
	if err := f.SetMatchMetadata(m); err != nil {
		t.Fatal(err)
	}
	var row ScoutRow
	row[0] = "*P01>LUp"
	if err := f.Section(SectionScout).SetScoutRecords([]ScoutRecord{row.Record()}); err != nil {
		t.Fatal(err)
	}
	b, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := parsed.MatchMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if got.Date != m.Date || got.League != m.League {
		t.Fatalf("metadata mismatch: %#v", got)
	}
	records, err := parsed.Section(SectionScout).ScoutRecords()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Code != row[0] {
		t.Fatalf("records mismatch: %#v", records)
	}
}

func TestMatchMetadataErrors(t *testing.T) {
	if _, err := (&File{}).MatchMetadata(); err == nil {
		t.Fatal("expected missing-section error")
	}
	f, err := Parse([]byte("[3MATCH]\n;;12345;;;;;;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.MatchMetadata(); err == nil {
		t.Fatal("expected missing-metadata-row error")
	}
}

func TestSetMatchMetadataPreservesOpaqueCellsAndFollowingRows(t *testing.T) {
	input := []byte("[3MATCH]\n" +
		"01/02/2020;10.20.30;season;league;venue;home;opaque6;opaque7;UTF-8;1;Z;0;opaque13;\n" +
		";;12345;;;;;;\n")
	f, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.MatchMetadata()
	if err != nil {
		t.Fatal(err)
	}
	m.League = "new league"
	if err := f.SetMatchMetadata(m); err != nil {
		t.Fatal(err)
	}
	got := f.Section(SectionMatch).Lines
	if len(got) != 2 {
		t.Fatalf("expected metadata and auxiliary rows, got %#v", got)
	}
	cells := strings.Split(got[0].Text, ";")
	if cells[3] != "new league" || cells[6] != "opaque6" || cells[7] != "opaque7" || cells[12] != "opaque13" {
		t.Fatalf("opaque metadata cells changed: %#v", cells)
	}
	if got[1].Text != ";;12345;;;;;;" {
		t.Fatalf("auxiliary row changed: %q", got[1].Text)
	}
}

func TestSetMatchMetadataInsertsRowWhenMissing(t *testing.T) {
	f, err := Parse([]byte("[3MATCH]\n;;12345;;;;;;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.SetMatchMetadata(MatchMetadata{Date: "01/02/2020", TextEncoding: "UTF-8"}); err != nil {
		t.Fatal(err)
	}
	if len(f.Section(SectionMatch).Lines) != 2 {
		t.Fatalf("expected metadata row to be inserted, got %#v", f.Section(SectionMatch).Lines)
	}
	m, err := f.MatchMetadata()
	if err != nil || m.Date != "01/02/2020" || m.TextEncoding != "UTF-8" {
		t.Fatalf("metadata mismatch: %#v, err=%v", m, err)
	}
}

func TestEncodeDVUTF8(t *testing.T) {
	if got, err := EncodeDVUTF8(""); err != nil || got != "" {
		t.Fatalf("empty string encoded as %q, err=%v", got, err)
	}
	for _, tc := range []struct{ input, want string }{
		{"é", "\x0f2E9"},
		{"München", "\x0f24DFC6E6368656E"},
		{"€", "\x0f420AC"},
	} {
		got, err := EncodeDVUTF8(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("EncodeDVUTF8(%q) = %q, %v; want %q", tc.input, got, err, tc.want)
		}
	}
	input := "Team München €"
	encoded, err := EncodeDVUTF8(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeDVUTF8(encoded)
	if err != nil || got != input {
		t.Fatalf("round trip=%q err=%v", got, err)
	}
	if _, err := EncodeDVUTF8(string([]byte{0xff})); err == nil {
		t.Fatal("expected invalid UTF-8 input to fail")
	}
	if _, err := EncodeDVUTF8("😀"); err == nil {
		t.Fatal("expected unsupported supplementary code point to fail")
	}
}

func TestLineEndingsAndFinalNewlineNormalization(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "preserve CRLF",
			input: []byte("[3SCOUT]\r\nabc;\r\n"),
			want:  []byte("[3SCOUT]\r\nabc;\r\n"),
		},
		{
			name:  "add missing final LF",
			input: []byte("[3SCOUT]\nabc;"),
			want:  []byte("[3SCOUT]\nabc;\n"),
		},
		{
			name:  "preserve BOM",
			input: []byte{0xef, 0xbb, 0xbf, '[', 'X', ']', '\n'},
			want:  []byte{0xef, 0xbb, 0xbf, '[', 'X', ']', '\n'},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("Marshal() = %q, want %q", got, tt.want)
			}
			if bytes.HasPrefix(tt.input, []byte{0xef, 0xbb, 0xbf}) && f.Section("X") == nil {
				t.Fatal("BOM prevented section recognition")
			}
		})
	}
}

func TestParsedLineSeparatesTextFromTerminator(t *testing.T) {
	f, err := Parse([]byte("[3SCOUT]\r\nserve;row\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	line := f.Section(SectionScout).Lines[0]
	if line.Text != "serve;row" || line.Ending != LineEndingCRLF {
		t.Fatalf("parsed line = %#v; want text and CRLF stored separately", line)
	}
}

func TestEmptyFileRoundTrip(t *testing.T) {
	f, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Marshal()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty round trip returned %q, err=%v", got, err)
	}
}

func TestMarshalRejectsNilFileAndSection(t *testing.T) {
	var nilFile *File
	if _, err := nilFile.Marshal(); err == nil {
		t.Fatal("expected nil-file error")
	}
	if _, err := (&File{Sections: []*Section{nil}}).Marshal(); err == nil {
		t.Fatal("expected nil-section error")
	}
}

func TestWriteDetectsShortWrite(t *testing.T) {
	err := NewFile().Write(shortWriter{})
	if err != io.ErrShortWrite {
		t.Fatalf("got %v; want %v", err, io.ErrShortWrite)
	}
}

func TestWriteSerializesFile(t *testing.T) {
	f := NewFile()
	want, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := f.Write(&got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("Write produced %q; want %q", got.Bytes(), want)
	}
}

func TestWriteReturnsUnderlyingError(t *testing.T) {
	want := errors.New("write failed")
	if err := NewFile().Write(errorWriter{err: want}); !errors.Is(err, want) {
		t.Fatalf("got %v; want wrapped %v", err, want)
	}
}

func TestWriteTo(t *testing.T) {
	f := NewFile()
	want, err := f.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	n, err := f.WriteTo(&got)
	if err != nil {
		t.Fatal(err)
	}
	if n != int64(len(want)) || !bytes.Equal(got.Bytes(), want) {
		t.Fatalf("WriteTo wrote %d bytes %q; want %d bytes %q", n, got.Bytes(), len(want), want)
	}
}

func TestWriteToPropagatesWriterFailures(t *testing.T) {
	invalid := &File{
		Sections: []*Section{{
			Name:  "3SCOUT",
			Lines: []Line{{Text: "record", Ending: LineEnding("invalid")}},
		}},
	}
	if n, err := invalid.WriteTo(&bytes.Buffer{}); err == nil || n != 0 {
		t.Fatalf("WriteTo with invalid file returned %d bytes and error %v", n, err)
	}

	if _, err := NewFile().WriteTo(shortWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write returned %v", err)
	}
	want := errors.New("write failed")
	if _, err := NewFile().WriteTo(errorWriter{err: want}); !errors.Is(err, want) {
		t.Fatalf("writer error returned %v; want %v", err, want)
	}
}

func TestMarshalRejectsInvalidSectionAndLineEnding(t *testing.T) {
	f := NewFile()
	f.Newline = LineEnding("\r")
	if _, err := f.Marshal(); err == nil {
		t.Fatal("expected invalid line ending error")
	}
	f.Newline = LineEndingLF
	f.Sections = []*Section{{
		Name:  "3SCOUT",
		Lines: []Line{{Text: "record", Ending: LineEnding("invalid")}},
	}}
	if _, err := f.Marshal(); err == nil {
		t.Fatal("expected invalid line ending on section line")
	}

	f.Sections = nil
	f.AddSection("bad]name")
	if _, err := f.Marshal(); err == nil {
		t.Fatal("expected invalid section name error")
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

func TestReadReturnsUnderlyingError(t *testing.T) {
	want := errors.New("read failed")
	if _, err := Read(errorReader{err: want}); !errors.Is(err, want) {
		t.Fatalf("got %v; want %v", err, want)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestRowsPreserveTrailingCells(t *testing.T) {
	s := &Section{Lines: []Line{{Text: "one;two;"}}}
	rows := s.Rows()
	if len(rows) != 1 || len(rows[0]) != 3 || rows[0][2] != "" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestNilSectionHelpers(t *testing.T) {
	var section *Section
	if rows := section.Rows(); rows != nil {
		t.Fatalf("Rows on nil section = %#v, want nil", rows)
	}
	if value, ok := section.Header("key"); value != "" || ok {
		t.Fatalf("Header on nil section = %q, %t", value, ok)
	}
	if err := section.AppendRow(nil); err == nil {
		t.Fatal("AppendRow on nil section succeeded")
	}
	if _, err := section.ScoutRecords(); err == nil {
		t.Fatal("ScoutRecords on nil section succeeded")
	}
	if err := section.SetScoutRecords(nil); err == nil {
		t.Fatal("SetScoutRecords on nil section succeeded")
	}
	if err := section.SetHeader("key", "value"); err == nil {
		t.Fatal("SetHeader on nil section succeeded")
	}
	if got := (*File)(nil).Section(SectionScout); got != nil {
		t.Fatalf("Section on nil file = %#v, want nil", got)
	}
}

func TestRowsStripCRLFTerminator(t *testing.T) {
	s := &Section{Lines: []Line{{Text: "one;two;", Ending: LineEndingCRLF}}}
	rows := s.Rows()
	if len(rows) != 1 || len(rows[0]) != 3 || rows[0][1] != "two" || rows[0][2] != "" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestAppendRowRejectsDelimiterInjection(t *testing.T) {
	s := &Section{Name: SectionTeams}
	if err := s.AppendRow([]string{"home", "A;B"}); err == nil {
		t.Fatal("expected embedded semicolon to be rejected")
	}
	if len(s.Lines) != 0 {
		t.Fatalf("invalid row was appended: %#v", s.Lines)
	}
	if err := s.AppendRow([]string{"home", "Team A"}); err != nil {
		t.Fatal(err)
	}
}

func TestSetHeaderRejectsNewlines(t *testing.T) {
	s := &Section{Name: SectionDataVolleyScout}
	if err := s.SetHeader("KEY", "value\n[3SCOUT]"); err == nil {
		t.Fatal("expected newline to be rejected")
	}
	if len(s.Lines) != 0 {
		t.Fatalf("invalid header was appended: %#v", s.Lines)
	}
}

func TestHeaderLookupAndSet(t *testing.T) {
	s := &Section{Name: SectionDataVolleyScout, Lines: []Line{{Text: "FILEFORMAT: 2.0", Ending: LineEndingCRLF}}}
	if got, ok := s.Header("fileformat"); !ok || got != "2.0" {
		t.Fatalf("Header() = %q, %v", got, ok)
	}
	if _, ok := s.Header("missing"); ok {
		t.Fatal("Header unexpectedly found missing key")
	}
	if err := s.SetHeader("FILEFORMAT", "3.0"); err != nil {
		t.Fatal(err)
	}
	if s.Lines[0].Text != "FILEFORMAT: 3.0" || s.Lines[0].Ending != LineEndingCRLF {
		t.Fatalf("SetHeader changed line: %#v", s.Lines[0])
	}
	if err := s.SetHeader("GENERATOR-IDP", "DVW"); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Header("GENERATOR-IDP"); !ok || got != "DVW" {
		t.Fatalf("appended header = %q, %v", got, ok)
	}
}

func TestSetMatchMetadataRejectsDelimiterInjection(t *testing.T) {
	f := NewFile()
	m, err := f.MatchMetadata()
	if err != nil {
		t.Fatal(err)
	}
	m.League = "league;unexpected"
	if err := f.SetMatchMetadata(m); err == nil {
		t.Fatal("expected semicolon to be rejected")
	}
}

func TestSetMatchMetadataRequiresMatchSection(t *testing.T) {
	if err := (&File{}).SetMatchMetadata(MatchMetadata{}); err == nil {
		t.Fatal("expected missing-section error")
	}
}

func TestSetScoutRecordsRejectsDelimiterInjection(t *testing.T) {
	s := &Section{Name: SectionScout, Lines: []Line{{Text: "existing row"}}}
	record := ScoutRecord{Code: "bad;code"}
	if err := s.SetScoutRecords([]ScoutRecord{record}); err == nil {
		t.Fatal("expected semicolon to be rejected")
	}
	if len(s.Lines) != 1 || s.Lines[0].Text != "existing row" {
		t.Fatalf("failed replacement changed existing rows: %#v", s.Lines)
	}
}

func TestScoutRow(t *testing.T) {
	line := "a;b;c;d;e;f;g;h;i;j;k;l;m;n;o;p;q;r;s;t;u;v;w;x;y;z;"
	r, err := ParseScoutRow(line)
	if err != nil {
		t.Fatal(err)
	}
	if r.String() != line {
		t.Fatalf("got %q", r.String())
	}
	if got := r.Record().Row().String(); got != line {
		t.Fatalf("named record round trip changed row to %q", got)
	}
}

func TestParseScoutRecordReportsMalformedRow(t *testing.T) {
	if _, err := ParseScoutRecord(strings.Repeat("x;", ScoutFieldCount)); err == nil {
		t.Fatal("expected malformed-row error")
	}
}

func TestFormatCoordinateBounds(t *testing.T) {
	for _, n := range []int{-1, 10000} {
		if _, err := FormatCoordinate(n); err == nil {
			t.Errorf("FormatCoordinate(%d) unexpectedly succeeded", n)
		}
	}
}

func TestParseScoutRowRejectsMalformedFieldCount(t *testing.T) {
	for _, empty := range []string{"", "\r\n"} {
		if _, err := ParseScoutRow(empty); err == nil {
			t.Errorf("ParseScoutRow(%q) unexpectedly accepted an empty row", empty)
		}
	}
	for _, line := range []string{"a;b;c;d;e;f;g;h;i;j;k;l;m;n;o;p;q;r;s;t;u;v;w;x;y;z;extra;"} {
		if _, err := ParseScoutRow(line); err == nil {
			t.Errorf("ParseScoutRow(%q) unexpectedly succeeded", line)
		}
	}
	short, err := ParseScoutRow("a;b")
	if err != nil || short[0] != "a" || short[1] != "b" || short[2] != "" {
		t.Fatalf("short row not padded: %#v, err=%v", short, err)
	}
	if _, err := ParseScoutRow("a;b;c;d;e;f;g;h;i;j;k;l;m;n;o;p;q;r;s;t;u;v;w;x;y;z;\r\n"); err != nil {
		t.Fatalf("valid CRLF row rejected: %v", err)
	}
}

func TestScoutRecordsSkipsNULPaddingAndRejectsWrongSection(t *testing.T) {
	rows := []Line{
		{Text: "a;b;c;d;e;f;g;h;i;j;k;l;m;n;o;p;q;r;s;t;u;v;w;x;y;z;\x00"},
		{Text: "\x00\x00"},
	}
	s := &Section{Name: SectionScout, Lines: rows}
	records, err := s.ScoutRecords()
	if err != nil || len(records) != 1 || records[0].Code != "a" {
		t.Fatalf("records=%#v err=%v", records, err)
	}
	if _, err := (&Section{Name: SectionMatch}).ScoutRecords(); err == nil {
		t.Fatal("expected wrong-section error")
	}
	wrong := &Section{Name: SectionMatch, Lines: []Line{{Text: "keep"}}}
	if err := wrong.SetScoutRecords(nil); err == nil {
		t.Fatal("expected wrong-section error from setter")
	}
	if len(wrong.Lines) != 1 || wrong.Lines[0].Text != "keep" {
		t.Fatalf("wrong-section setter mutated data: %#v", wrong.Lines)
	}
	malformed := &Section{Name: SectionScout, Lines: []Line{{Text: strings.Repeat("x;", ScoutFieldCount)}}}
	if _, err := malformed.ScoutRecords(); err == nil {
		t.Fatal("expected malformed-row error")
	} else {
		var lineErr *SectionLineError
		if !errors.As(err, &lineErr) || lineErr.Section != SectionScout || lineErr.Line != 1 {
			t.Fatalf("expected structured source-line context, got %#v", err)
		}
	}
}

func TestSectionLineError(t *testing.T) {
	cause := errors.New("invalid row")
	err := &SectionLineError{
		Section: SectionScout,
		Line:    2,
		Err:     cause,
	}

	if got, want := err.Error(), `section "3SCOUT", line 2: invalid row`; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("Unwrap() did not preserve cause %v", cause)
	}
}

func TestScoutRecordValidationFailures(t *testing.T) {
	tests := []struct {
		name   string
		change func(*ScoutRecord)
	}{
		{"missing code", func(r *ScoutRecord) { r.Code = "" }},
		{"point phase", func(r *ScoutRecord) { r.PointPhase = "x" }},
		{"attack phase", func(r *ScoutRecord) { r.AttackPhase = "x" }},
		{"clock time", func(r *ScoutRecord) { r.Time = "25.00.00" }},
		{"coordinate", func(r *ScoutRecord) { r.StartCoordinate = "12x4" }},
		{"setter position out of range", func(r *ScoutRecord) { r.HomeSetterPosition = "7" }},
		{"nonnumeric setter position", func(r *ScoutRecord) { r.HomeSetterPosition = "unknown" }},
		{"trailing field", func(r *ScoutRecord) { r.TrailingField = "unexpected" }},
		{"reserved field", func(r *ScoutRecord) { r.Reserved1 = "unexpected" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ScoutRecord{
				Code:               "*01SH+",
				PointPhase:         "p",
				AttackPhase:        "r",
				Time:               "12.34.56",
				StartCoordinate:    "0123",
				HomeSetterPosition: "5",
			}
			tt.change(&r)
			if err := r.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
