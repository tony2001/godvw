package dvw_test

import (
	"fmt"
	"strings"

	dvw "github.com/tony2001/godvw"
)

func ExampleRead() {
	input := strings.NewReader("[3SCOUT]\n*01SH+;p;r;;0123\n")
	file, err := dvw.Read(input)
	if err != nil {
		fmt.Println("read failed:", err)
		return
	}

	records, err := file.Section(dvw.SectionScout).ScoutRecords()
	if err != nil {
		fmt.Println("parse records failed:", err)
		return
	}
	if err := records[0].Validate(); err != nil {
		fmt.Println("invalid record:", err)
		return
	}

	fmt.Printf("%s starts at %s\n", records[0].Code, records[0].StartCoordinate)
	// Output: *01SH+ starts at 0123
}

func ExampleFile_SetMatchMetadata() {
	file := dvw.NewFile()
	metadata, err := file.MatchMetadata()
	if err != nil {
		fmt.Println("read metadata failed:", err)
		return
	}
	metadata.Date = "11/13/2026"
	metadata.League = "Example League"
	if err := file.SetMatchMetadata(metadata); err != nil {
		fmt.Println("update metadata failed:", err)
		return
	}

	data, err := file.Marshal()
	if err != nil {
		fmt.Println("marshal failed:", err)
		return
	}
	parsed, err := dvw.Parse(data)
	if err != nil {
		fmt.Println("parse failed:", err)
		return
	}
	metadata, err = parsed.MatchMetadata()
	if err != nil {
		fmt.Println("read updated metadata failed:", err)
		return
	}
	fmt.Printf("%s: %s\n", metadata.Date, metadata.League)
	// Output: 11/13/2026: Example League
}
