# godvw

`godvw` is a small Go package for reading and writing DataVolley `.dvw` files.
It preserves section order, unknown sections, section content bytes, and
trailing empty semicolon fields. Parsed line content and its LF or CRLF
terminator are stored separately. The writer preserves existing line endings
and uses the first line-ending style for lines without a terminator. Serialized
documents with at least one line always end with a newline. Applications can
edit known fields without discarding metadata they do not understand.

Values are returned as byte-preserving Go strings; `MatchMetadata` exposes the
encoding label in `[3MATCH]`, and `DecodeDVUTF8` handles DataVolley's embedded
Unicode tokens. Legacy text is not transcoded, so callers can preserve it
exactly or decode it with a charset package of their choice. Row-writing
helpers reject semicolons and line breaks inside cells. Scouting codes can be
decoded with `ParseScoutCode`; evaluation labels support DataVolley, German,
and VolleyMetrics conventions.

## Scouting record format

Each data row in `[3SCOUT]` has 27 semicolon-separated cells in this order.
Cell numbers below are one-based. Cell 27 is the trailing empty cell, so a
full row normally ends in a semicolon.

| Cells | `ScoutRecord` field | Description |
| --- | --- | --- |
| 1 | `Code` | Scouting code, decoded by `ParseScoutCode`. |
| 2 | `PointPhase` | Point phase (`p` or `s` when set). |
| 3 | `AttackPhase` | Attack phase (`p`, `s`, or `r` when set). |
| 4 | `Reserved1` | Reserved; must be empty for `Validate` to pass. |
| 5 | `StartCoordinate` | Start location; four digits, `-1-1`, or empty. |
| 6 | `MidCoordinate` | Middle location; four digits, `-1-1`, or empty. |
| 7 | `EndCoordinate` | End location; four digits, `-1-1`, or empty. |
| 8 | `Time` | Match time in `HH.MM.SS` format when set. |
| 9 | `SetNumber` | Set number. |
| 10 | `HomeSetterPosition` | Home setter position from `0`–`6`; `5` represents unknown. |
| 11 | `VisitingSetterPosition` | Visiting setter position from `0`–`6`; `5` represents unknown. |
| 12 | `VideoFileNumber` | Video file number. |
| 13 | `VideoTime` | Time associated with the video. |
| 14 | `Reserved2` | Reserved; must be empty for `Validate` to pass. |
| 15–20 | `HomePlayers[0..5]` | Six home-player values. |
| 21–26 | `VisitingPlayers[0..5]` | Six visiting-player values. |
| 27 | `TrailingField` | Must be empty for `Validate` to pass. |

`ParseScoutRow` and `ParseScoutRecord` accept shorter rows and pad omitted
cells with empty strings; they reject rows with more than 27 cells. Parsing
does not imply validation. `ScoutRecord.Validate` checks row fields and code
syntax under the default conventions; `ValidateWithOptions` selects another
code convention.

```go
package main

import dvw "godvw"

func validateScoutingLine(line string) error {
    record, err := dvw.ParseScoutRecord(line)
    if err != nil {
        return err
    }
    return record.Validate()
}
```

### Scouting code syntax

The first `[3SCOUT]` cell contains a compact code for a ball contact or match
event. A regular contact code has this shape:

```text
[team][player][skill][type][evaluation][combination][set type][start zone][end zone][subzone][subtype][players][special][custom suffix]
```

The team marker is `*` for the home team and `a` for the visiting team. It is
followed by the jersey number, then one-character skill, type, and evaluation
codes. The remaining positions are optional but fixed-width: combination or
setter-call code (two characters), then set type, start and end locations,
end subzone, skill subtype, number of players, and special code (one character
each). `~` is a placeholder when later positions must be retained but an
earlier optional value is absent. Text after these positions is kept as the
custom suffix.

For example, `a13AH+X5~47DH2C` means visiting-team player 13 made a high-ball
attack with a positive evaluation using attack combination `X5`. It records
start zone 4, destination zone 7/subzone D, a hard spike, a two-player block,
and special code `C` (block control). The code's positional fields are
independent of the three four-digit coordinates in the `[3SCOUT]` row.

Common skill codes are `S` serve, `R` reception, `E` set, `A` attack, `B`
block, `D` dig, and `F` freeball. For example, `*07SM+` is a home-team serve;
`M` identifies its type and `+` its evaluation. Evaluation meanings depend on
both skill and scouting convention: under the default mapping, `S#` is an
ace, `R#` a perfect pass, and `A#` a winning attack. VolleyMetrics and German
modes change some mappings, particularly for blocks and block cover.

The parser also recognizes non-contact records, including `*p12:09` (point
and score), `*T` (timeout), `*P04` (substitution), `*z3` / `az3` (setter
position), and `**3set` (end of set). Codes containing `>LUp` are recognized as
lineup updates and left for the caller to handle as metadata.

`ParseScoutCode` decodes a code directly:

```go
import (
    "fmt"

    dvw "godvw"
)

func decodeExample(file *dvw.File) error {
    options := file.ScoutCodeOptions(dvw.ScoutCodeStyleDefault)
    code, err := dvw.ParseScoutCodeWithOptions("*07SM+", options)
    if err != nil {
        return err
    }
    fmt.Printf("%s: %s, %s\n", code.Skill, code.SkillType, code.Evaluation)

    issues, err := dvw.ValidateScoutCodeWithOptions("*07SM+", options)
    if err != nil {
        return err
    }
    for _, issue := range issues {
        fmt.Println(issue.Message)
    }
    return nil
}
```

`ScoutCode` exposes the raw code and decoded fields, including team, player
number, skill, type, evaluation, attack or setter-call code, locations, subtype,
player count, special code, and custom suffix. A `$$` player marker sets
`UnknownPlayer`. `ScoutCode.Issues` contains non-fatal diagnostics with a
message and severity; unknown values remain available for inspection. Empty
codes and unsupported styles return errors.

The available styles are `ScoutCodeStyleDefault`,
`ScoutCodeStyleVolleyMetrics`, and `ScoutCodeStyleGerman`; an empty style means
default. `ScoutCodeOptions` also has `Beach` and `Cones` switches for beach
volleyball descriptions and cone-based attack destinations. Non-nil
`AttackCombinations` and `SetterCalls` maps enable metadata lookup and report
unmatched codes. `File.ScoutCodeOptions` builds these maps from the matching
metadata sections. Player names require roster lookup; custom parser callbacks
are application-specific.

```go
package main

import (
    "os"

    "godvw"
)

func copyMatch() error {
    input, err := os.Open("match.dvw")
    if err != nil {
        return err
    }
    defer input.Close()

    f, err := dvw.Read(input)
    if err != nil {
        return err
    }

    output, err := os.Create("copy.dvw")
    if err != nil {
        return err
    }
    defer output.Close()
    return f.Write(output)
}
```

`NewFile` creates an empty section skeleton; teams, rosters, and set metadata
must be populated before relying on the file in DataVolley software.
`MatchMetadata` reads and updates common `[3MATCH]` fields while retaining
other columns. `ScoutRecord` provides a named field for each of the 27
`[3SCOUT]` cells, including reserved and trailing cells. Short scouting lines
are padded when parsed, matching datavolley's truncated-row behavior. Section
methods read and replace record lists. Coordinates can be formatted with
`FormatCoordinate` (four digits), and `-1-1` is supported for missing points
when another coordinate is present.
