package dvw

import "testing"

func TestScoutCodeSemanticHelpers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		raw               string
		team              string
		skill             string
		ace               bool
		block             bool
		blocked           bool
		chartError        bool
		positive          bool
		positiveReception bool
		negative          bool
		poor              bool
		transition        bool
	}{
		{name: "ace", raw: "*01SQ#", team: ScoutTeamHome, skill: ScoutSkillServe, ace: true, positive: true},
		{name: "perfect pass", raw: "a02RH#", team: ScoutTeamVisiting, skill: ScoutSkillReception, positive: true, positiveReception: true},
		{name: "winning attack", raw: "*01AH#", team: ScoutTeamHome, skill: ScoutSkillAttack, positive: true},
		{name: "block kill", raw: "a02BH#", team: ScoutTeamVisiting, skill: ScoutSkillBlock, block: true, positive: true},
		{name: "blocked attack", raw: "*01AH/", team: ScoutTeamHome, skill: ScoutSkillAttack, blocked: true},
		{name: "serve error", raw: "*01SQ=", team: ScoutTeamHome, skill: ScoutSkillServe, chartError: true},
		{name: "reception error excluded", raw: "a02RH=", team: ScoutTeamVisiting, skill: ScoutSkillReception},
		{name: "invasion", raw: "*01BH/", team: ScoutTeamHome, skill: ScoutSkillBlock, chartError: true},
		{name: "positive serve", raw: "*01SH+", team: ScoutTeamHome, skill: ScoutSkillServe, positive: true},
		{name: "good dig", raw: "*01DH+", team: ScoutTeamHome, skill: ScoutSkillDig, positive: true},
		{name: "negative serve", raw: "*01SH-", team: ScoutTeamHome, skill: ScoutSkillServe, negative: true},
		{name: "poor attack", raw: "*01AH-", team: ScoutTeamHome, skill: ScoutSkillAttack, poor: true},
		{name: "positive reception", raw: "a02RH+", team: ScoutTeamVisiting, skill: ScoutSkillReception, positive: true, positiveReception: true},
		{name: "poor reception", raw: "a02RH/", team: ScoutTeamVisiting, skill: ScoutSkillReception, poor: true, transition: true},
		{name: "transition reception", raw: "a02RH-", team: ScoutTeamVisiting, skill: ScoutSkillReception, negative: true, transition: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, err := ParseScoutCode(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if !code.IsContact() || !code.HasKnownSkill() || !code.HasKnownEvaluation() ||
				!code.IsSkill(tc.skill) || !code.IsTeam(tc.team) {
				t.Fatalf("unexpected code semantics: %+v", code)
			}
			if code.IsAce() != tc.ace || code.IsWinningBlock() != tc.block ||
				code.IsBlockedAttack() != tc.blocked || code.IsChartError() != tc.chartError ||
				code.IsPositiveEvaluation() != tc.positive || code.IsPositiveReception() != tc.positiveReception ||
				code.IsNegativeEvaluation() != tc.negative ||
				code.IsPoorEvaluation() != tc.poor ||
				code.IsTransitionReception() != tc.transition {
				t.Fatalf("semantic predicate mismatch for %+v", code)
			}
		})
	}
}

func TestScoutCodeIsChartError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		skill    string
		eval     string
		rotation bool
		want     bool
	}{
		{name: "serve error", skill: ScoutSkillServe, eval: "=", want: true},
		{name: "attack error", skill: ScoutSkillAttack, eval: "=", want: true},
		{name: "set error", skill: ScoutSkillSet, eval: "=", want: true},
		{name: "freeball error", skill: ScoutSkillFreeball, eval: "=", want: true},
		{name: "default block error", skill: ScoutSkillBlock, eval: "=", want: true},
		{name: "default block invasion", skill: ScoutSkillBlock, eval: "/", want: true},
		{name: "reception error", skill: ScoutSkillReception, eval: "="},
		{name: "dig error", skill: ScoutSkillDig, eval: "="},
		{name: "rotation error", rotation: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := ScoutCode{
				SkillCode:      tc.skill,
				EvaluationCode: tc.eval,
				RotationError:  tc.rotation,
			}
			if got := code.IsChartError(); got != tc.want {
				t.Fatalf("IsChartError() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestScoutCodeContactPredicates(t *testing.T) {
	t.Parallel()

	unknownSkill, err := ParseScoutCode("*01XH+")
	if err != nil {
		t.Fatal(err)
	}
	if !unknownSkill.IsContact() || unknownSkill.HasKnownSkill() || unknownSkill.HasKnownEvaluation() {
		t.Fatalf("unexpected unknown-skill semantics: %+v", unknownSkill)
	}

	unknownEvaluation, err := ParseScoutCode("*01SH?")
	if err != nil {
		t.Fatal(err)
	}
	if !unknownEvaluation.IsContact() || !unknownEvaluation.HasKnownSkill() || unknownEvaluation.HasKnownEvaluation() {
		t.Fatalf("unexpected unknown-evaluation semantics: %+v", unknownEvaluation)
	}

	point, err := ParseScoutCode("*p1:0")
	if err != nil {
		t.Fatal(err)
	}
	if point.IsContact() {
		t.Fatalf("point record was classified as a contact: %+v", point)
	}
}

func TestZeroScoutCodeHasNoEvaluationSemantics(t *testing.T) {
	t.Parallel()
	var code ScoutCode
	if code.IsAce() || code.IsWinningAttack() || code.IsWinningBlock() || code.IsError() ||
		code.IsEvaluationError() || code.IsChartError() || code.IsBlockedAttack() ||
		code.IsPositiveEvaluation() || code.IsNegativeEvaluation() || code.IsPoorEvaluation() ||
		code.IsPerfectReception() || code.IsPositiveReception() || code.IsTransitionReception() {
		t.Fatalf("zero ScoutCode unexpectedly has evaluation semantics: %+v", code)
	}
}

func TestScoutCodeSubstitutionPlayers(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"*c01:02", "aC15.08"} {
		code, err := ParseScoutCode(raw)
		if err != nil {
			t.Fatal(err)
		}
		outgoing, incoming, ok := code.SubstitutionPlayers()
		if !code.Substitution || !ok || outgoing <= 0 || incoming <= 0 {
			t.Fatalf("substitution %q decoded as %d, %d, %t", raw, outgoing, incoming, ok)
		}
	}

	for _, raw := range []string{"*c", "*c01", "*c01:0", "*c01::02", "*c01.02junk", "*c-1:02"} {
		t.Run("invalid/"+raw, func(t *testing.T) {
			code, err := ParseScoutCode(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !code.Substitution || len(code.Issues) == 0 {
				t.Fatalf("malformed substitution %q was not diagnosed: %+v", raw, code)
			}
		})
	}
}

func TestScoutRecordDecodedCodeSemantics(t *testing.T) {
	t.Parallel()
	record, err := ParseScoutRecord("*01AH#V3~34DP1I")
	if err != nil {
		t.Fatal(err)
	}

	code, err := record.DecodedCode()
	if err != nil {
		t.Fatal(err)
	}
	if !code.IsWinningAttack() || code.Special != "Net contact" {
		t.Fatalf("decoded scouting semantics = %+v", code)
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("record with supported attack net-contact code did not validate: %v", err)
	}
}
