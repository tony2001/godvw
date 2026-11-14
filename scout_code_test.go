package dvw

import (
	"strings"
	"testing"
)

func TestParseScoutCodeSkillAndEvaluation(t *testing.T) {
	code, err := ParseScoutCode("*01SH+")
	if err != nil {
		t.Fatal(err)
	}
	got := struct {
		team, skill, skillType, evaluation string
		player                             int
	}{code.Team, code.Skill, code.SkillType, code.Evaluation, code.PlayerNumber}
	want := struct {
		team, skill, skillType, evaluation string
		player                             int
	}{"*", "Serve", "Float serve", "Positive, opponent some attack", 1}
	if got != want {
		t.Fatalf("unexpected decoded serve: %#v", code)
	}
	if len(code.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", code.Issues)
	}
}

func TestParseScoutCodeDataVolleyEvaluations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		raw     string
		want    string
		isError bool
		isPoor  bool
	}{
		{name: "block invasion", raw: "*01BH/", want: "Invasion", isError: true},
		{name: "block error", raw: "*01BH=", want: "Error", isError: true},
		{name: "poor set", raw: "*01EH/", want: "Poor", isPoor: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := ParseScoutCode(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if code.Evaluation != tc.want {
				t.Errorf("evaluation = %q, want %q", code.Evaluation, tc.want)
			}
			if code.IsError() != tc.isError || code.IsPoorEvaluation() != tc.isPoor {
				t.Errorf("unexpected DataVolley semantics: %+v", code)
			}
		})
	}
}

func TestParseScoutCodeAutomaticRecords(t *testing.T) {
	for _, tc := range []struct {
		code  string
		check func(ScoutCode) bool
	}{
		{code: "*p12:09", check: func(c ScoutCode) bool { return c.Point && c.HomeScore == 12 && c.VisitingScore == 9 }},
		{code: "*T", check: func(c ScoutCode) bool { return c.Timeout && c.Skill == "Timeout" }},
		{code: "*P04", check: func(c ScoutCode) bool { return c.Substitution }},
		{code: "az3", check: func(c ScoutCode) bool { return c.SetterTeam == "a" && c.SetterPosition == 3 }},
		{code: "**3set", check: func(c ScoutCode) bool { return c.EndOfSet }},
	} {
		t.Run(tc.code, func(t *testing.T) {
			code, err := ParseScoutCode(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.check(code) {
				t.Errorf("unexpected decoded automatic code: %#v", code)
			}
		})
	}
}

func TestParseScoutCodeAutomaticRecordsRequireTeamMarker(t *testing.T) {
	for _, raw := range []string{"?p12:09", "x04>LUp"} {
		t.Run(raw, func(t *testing.T) {
			code, err := ParseScoutCode(raw)
			if err != nil {
				t.Fatal(err)
			}
			if len(code.Issues) == 0 || !strings.Contains(code.Issues[0].Message, "Team entries") {
				t.Fatalf("expected missing team diagnostic, got %#v", code.Issues)
			}
		})
	}
}

func TestParseScoutCodeAutomaticRecordsRejectTrailingJunk(t *testing.T) {
	for _, raw := range []string{"*p12:09extra", "*z3extra", "**3setextra", "xp12:09"} {
		t.Run(raw, func(t *testing.T) {
			code, err := ParseScoutCode(raw)
			if err != nil {
				t.Fatal(err)
			}
			if code.Point || code.SetterPosition != 0 || code.EndOfSet {
				t.Fatalf("malformed automatic code was accepted: %#v", code)
			}
			if len(code.Issues) == 0 {
				t.Fatalf("malformed automatic code produced no diagnostic: %#v", code)
			}
		})
	}
}

func TestParseScoutCodeMetadataAllowsEmptyDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		code    string
		options ScoutCodeOptions
	}{
		{name: "attack", code: "*01AH#X5", options: ScoutCodeOptions{AttackCombinations: map[string]string{"X5": ""}}},
		{name: "setter call", code: "*01EH#K1", options: ScoutCodeOptions{SetterCalls: map[string]string{"K1": ""}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseScoutCodeWithOptions(tc.code, tc.options)
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Issues) != 0 {
				t.Fatalf("known metadata code reported issues: %#v", parsed.Issues)
			}
		})
	}
}

func TestParseScoutCodeSpecialDescriptionsDependOnEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{name: "serve in net is let", code: "*01SH+~~~~~~~~N", want: "Let"},
		{name: "attack in net on winning attack is let", code: "*01AH#~~~~~~~~N", want: "Let"},
		{name: "net contact on winning attack", code: "*01AH#~~~~~~~~I", want: "Net contact"},
		{name: "fixture attack with net contact", code: "*01AH#V3~34DP1I", want: "Net contact"},
		{name: "invalid special is not decoded", code: "*01AH/~~~~~~~~I", want: "Unexpected I"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseScoutCode(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Special != tc.want {
				t.Errorf("special = %q, want %q", parsed.Special, tc.want)
			}
			if strings.HasPrefix(tc.want, "Unexpected ") && len(parsed.Issues) == 0 {
				t.Error("unexpected special code produced no issue")
			}
			if !strings.HasPrefix(tc.want, "Unexpected ") && len(parsed.Issues) != 0 {
				t.Errorf("valid special code reported issues: %#v", parsed.Issues)
			}
		})
	}
}

func TestParseScoutCodeFields(t *testing.T) {
	code, err := ParseScoutCode("a13AT+X5~47DH2C")
	if err != nil {
		t.Fatal(err)
	}
	got := struct {
		team, skill, attackCode, startZone, endZone string
		subtype, players, special                   string
		player                                      int
	}{
		team: code.Team, skill: code.Skill, attackCode: code.AttackCode,
		startZone: code.StartZone, endZone: code.EndZone,
		subtype: code.SkillSubtype, players: code.NumberPlayersLabel,
		special: code.Special, player: code.PlayerNumber,
	}
	want := struct {
		team, skill, attackCode, startZone, endZone string
		subtype, players, special                   string
		player                                      int
	}{"a", "Attack", "X5", "4", "7", "Hard spike", "2 player block", "Block control", 13}
	if got != want {
		t.Fatalf("unexpected decoded attack: %#v", code)
	}
}

func TestParseScoutCodeSetAndLocationOptions(t *testing.T) {
	set, err := ParseScoutCode("a10EH#KEF")
	if err != nil {
		t.Fatal(err)
	}
	if set.SetCode != "KE" || set.SetType != "F" {
		t.Fatalf("set code/type = %q/%q, want KE/F", set.SetCode, set.SetType)
	}

	attack, err := ParseScoutCodeWithOptions("*01AH+X5~47DH2C", ScoutCodeOptions{Cones: true})
	if err != nil {
		t.Fatal(err)
	}
	if attack.EndCone != "7" || attack.EndZone != "" {
		t.Fatalf("cone destination = %q, zone = %q", attack.EndCone, attack.EndZone)
	}

	beach, err := ParseScoutCode("*01SQ#")
	if err != nil {
		t.Fatal(err)
	}
	if beach.SkillType != "Jump serve" {
		t.Errorf("beach serve type = %q, want Jump serve", beach.SkillType)
	}
}

func TestParseScoutCodeValidatesFreeballType(t *testing.T) {
	for _, tc := range []struct {
		name       string
		code       string
		wantIssues bool
	}{
		{name: "known type", code: "*01FF+"},
		{name: "unknown type", code: "*01FX+", wantIssues: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseScoutCode(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			if gotIssues := len(parsed.Issues) != 0; gotIssues != tc.wantIssues {
				t.Errorf("issues = %#v, want issue presence %t", parsed.Issues, tc.wantIssues)
			}
		})
	}
}

func TestFileScoutCodeOptionsReadDescriptions(t *testing.T) {
	file := &File{}
	file.AddSection(SectionAttackCombination).Lines = []Line{{Text: "X5;4;R;H;High set in 4;;;"}}
	file.AddSection(SectionSetterCall).Lines = []Line{{Text: "K1;;Front Quick;"}}
	options := file.ScoutCodeOptions()

	attack, err := ParseScoutCodeWithOptions("*01AH#X5", options)
	if err != nil {
		t.Fatal(err)
	}
	if attack.AttackDescription != "High set in 4" {
		t.Errorf("attack description = %q", attack.AttackDescription)
	}
	set, err := ParseScoutCodeWithOptions("*01EH#K1", options)
	if err != nil {
		t.Fatal(err)
	}
	if set.SetDescription != "Front Quick" {
		t.Errorf("setter-call description = %q", set.SetDescription)
	}
}

func TestValidateScoutCodeReportsMalformedCode(t *testing.T) {
	issues, err := ValidateScoutCode("*01S")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) == 0 {
		t.Fatal("expected a missing-field issue")
	}
	issues, err = ValidateScoutCodeWithOptions("*01BH/", ScoutCodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("valid option-based code reported issues: %#v", issues)
	}
}

func TestParseScoutCodeReportsNumberOverflow(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want string
	}{
		{name: "player number", code: "*999999999999999999999SH+", want: "player number"},
		{name: "point score", code: "*p999999999999999999999:0", want: "home score"},
		{name: "setter position", code: "az999999999999999999999", want: "setter position"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := ParseScoutCode(tc.code)
			if err != nil {
				t.Fatal(err)
			}
			for _, issue := range parsed.Issues {
				if strings.Contains(issue.Message, tc.want) {
					return
				}
			}
			t.Fatalf("missing %q diagnostic: %#v", tc.want, parsed.Issues)
		})
	}
}
