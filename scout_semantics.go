package dvw

import "strings"

// Scout team markers used by scouting-code records.
const (
	ScoutTeamHome     = "*"
	ScoutTeamVisiting = "a"
)

// Scout skill codes used by DataVolley scouting records.
const (
	ScoutSkillServe     = "S"
	ScoutSkillReception = "R"
	ScoutSkillAttack    = "A"
	ScoutSkillBlock     = "B"
	ScoutSkillDig       = "D"
	ScoutSkillSet       = "E"
	ScoutSkillFreeball  = "F"
)

// Scout evaluation codes used by DataVolley scouting records.
const (
	ScoutEvaluationCodeError    = "="
	ScoutEvaluationCodeBlocked  = "/"
	ScoutEvaluationCodePoor     = "-"
	ScoutEvaluationCodePositive = "+"
	ScoutEvaluationCodeWinner   = "#"
	ScoutEvaluationCodeOK       = "!"
)

// IsSkill reports whether this contact uses skill.
func (c ScoutCode) IsSkill(skill string) bool { return c.SkillCode == skill }

// IsContact reports whether the code contains a contact, including an unknown skill.
func (c ScoutCode) IsContact() bool { return c.SkillCode != "" }

// HasKnownSkill reports whether the parser recognizes the skill code.
func (c ScoutCode) HasKnownSkill() bool { return scoutSkillNames[c.SkillCode] != "" }

// HasKnownEvaluation reports whether the parser recognizes the evaluation for
// this skill.
func (c ScoutCode) HasKnownEvaluation() bool {
	if c.SkillCode == "" || c.EvaluationCode == "" {
		return false
	}
	return evaluationDescriptions[c.SkillCode][c.EvaluationCode] != ""
}

// IsTeam reports whether this record belongs to team.
func (c ScoutCode) IsTeam(team string) bool { return c.Team == team }

// IsAce reports a serve evaluated as an ace.
func (c ScoutCode) IsAce() bool {
	return c.IsSkill(ScoutSkillServe) && c.EvaluationCode == ScoutEvaluationCodeWinner
}

// IsWinningAttack reports an attack that directly scores.
func (c ScoutCode) IsWinningAttack() bool {
	return c.IsSkill(ScoutSkillAttack) && c.EvaluationCode == ScoutEvaluationCodeWinner
}

// IsWinningBlock reports a block that directly scores.
func (c ScoutCode) IsWinningBlock() bool {
	return c.IsSkill(ScoutSkillBlock) && c.EvaluationCode == ScoutEvaluationCodeWinner
}

// IsError reports an error evaluation, including a block invasion.
func (c ScoutCode) IsError() bool {
	return c.IsEvaluationError() ||
		c.IsSkill(ScoutSkillBlock) && c.EvaluationCode == ScoutEvaluationCodeBlocked
}

// IsEvaluationError reports an evaluation code defined as an error for this
// skill and DataVolley mapping.
func (c ScoutCode) IsEvaluationError() bool {
	if c.RotationError {
		return true
	}
	return scoutSkillNames[c.SkillCode] != "" && c.EvaluationCode == ScoutEvaluationCodeError
}

// IsChartError reports an error included in score-chart error marks.
func (c ScoutCode) IsChartError() bool {
	if c.RotationError {
		return true
	}
	switch c.SkillCode {
	case ScoutSkillServe, ScoutSkillAttack, ScoutSkillSet, ScoutSkillFreeball:
		return c.EvaluationCode == ScoutEvaluationCodeError
	case ScoutSkillBlock:
		return c.EvaluationCode == ScoutEvaluationCodeError ||
			c.EvaluationCode == ScoutEvaluationCodeBlocked
	default:
		return false
	}
}

// IsBlockedAttack reports an attack that was blocked and stayed in play.
func (c ScoutCode) IsBlockedAttack() bool {
	return c.IsSkill(ScoutSkillAttack) && c.EvaluationCode == ScoutEvaluationCodeBlocked
}

// Evaluation-code sets classify outcomes independently of their display text.
var positiveEvaluationCodes = map[string]string{
	ScoutSkillServe:     "/+#",
	ScoutSkillReception: "+#",
	ScoutSkillAttack:    "+#",
	ScoutSkillBlock:     "+#",
	ScoutSkillDig:       "+#",
	ScoutSkillSet:       "+#",
	ScoutSkillFreeball:  "+#",
}

var negativeEvaluationCodes = map[string]string{
	ScoutSkillServe:     "-",
	ScoutSkillReception: "-",
}

var poorEvaluationCodes = map[string]string{
	ScoutSkillReception: "/",
	ScoutSkillAttack:    "-",
	ScoutSkillBlock:     "-!",
	ScoutSkillSet:       "-/",
	ScoutSkillFreeball:  "/",
}

// IsPositiveEvaluation reports whether the skill and evaluation codes
// represent a positive result under DataVolley mappings.
func (c ScoutCode) IsPositiveEvaluation() bool {
	return hasEvaluationCode(positiveEvaluationCodes[c.SkillCode], c.EvaluationCode)
}

// IsNegativeEvaluation reports a negative skill evaluation.
func (c ScoutCode) IsNegativeEvaluation() bool {
	return hasEvaluationCode(negativeEvaluationCodes[c.SkillCode], c.EvaluationCode)
}

// IsPoorEvaluation reports a poor skill evaluation.
func (c ScoutCode) IsPoorEvaluation() bool {
	return !c.IsEvaluationError() &&
		hasEvaluationCode(poorEvaluationCodes[c.SkillCode], c.EvaluationCode)
}

func hasEvaluationCode(codes, code string) bool {
	return code != "" && strings.Contains(codes, code)
}

// IsPerfectReception reports a reception evaluated as perfect.
func (c ScoutCode) IsPerfectReception() bool {
	return c.IsSkill(ScoutSkillReception) && c.EvaluationCode == ScoutEvaluationCodeWinner
}

// IsPositiveReception reports a positive or perfect reception.
func (c ScoutCode) IsPositiveReception() bool {
	return c.IsSkill(ScoutSkillReception) &&
		(c.EvaluationCode == ScoutEvaluationCodePositive || c.IsPerfectReception())
}

// IsTransitionReception reports a reception that does not permit a first-tempo attack.
func (c ScoutCode) IsTransitionReception() bool {
	return c.IsSkill(ScoutSkillReception) &&
		(c.IsPoorEvaluation() || c.EvaluationCode == ScoutEvaluationCodePoor ||
			c.EvaluationCode == ScoutEvaluationCodeOK)
}

// SubstitutionPlayers returns outgoing and incoming jersey numbers when known.
func (c ScoutCode) SubstitutionPlayers() (outgoing, incoming int, ok bool) {
	return c.SubstitutionOut, c.SubstitutionIn, c.SubstitutionOut > 0 && c.SubstitutionIn > 0
}
