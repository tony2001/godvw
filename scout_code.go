package dvw

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ScoutCodeStyle selects the evaluation and skill-type conventions used when
// decoding a scouting code.
type ScoutCodeStyle string

const (
	// ScoutCodeStyleDefault follows the standard DataVolley conventions.
	ScoutCodeStyleDefault ScoutCodeStyle = "default"
	// ScoutCodeStyleVolleyMetrics follows VolleyMetrics evaluation conventions.
	ScoutCodeStyleVolleyMetrics ScoutCodeStyle = "volleymetrics"
	// ScoutCodeStyleGerman follows the German DVV conventions.
	ScoutCodeStyleGerman ScoutCodeStyle = "german"
)

// ScoutCodeOptions controls convention-dependent scouting-code decoding.
type ScoutCodeOptions struct {
	// Style controls evaluation and serve-type labels. Empty means default.
	Style ScoutCodeStyle
	// Beach selects beach-volleyball serve and block-count descriptions.
	Beach bool
	// Cones interprets an attack's destination digit as a cone instead of a zone.
	Cones bool
	// AttackCombinations maps attack codes to their descriptions. A nil map skips
	// metadata lookup; a non-nil map reports unmatched attack codes.
	AttackCombinations map[string]string
	// SetterCalls maps setter-call codes to their descriptions. A nil map skips
	// metadata lookup; a non-nil map reports unmatched setter-call codes.
	SetterCalls map[string]string
}

// ScoutCodeIssue describes a non-fatal syntax or convention issue. The R
// datavolley parser reports many such conditions while retaining decoded data.
type ScoutCodeIssue struct {
	// Message explains the parser diagnostic.
	Message string
	// Severity is 1 for a warning and 2 for a stronger diagnostic.
	Severity int
}

// ScoutCode contains the fields decoded from one [3SCOUT] Code cell. Empty
// optional fields remain empty; unrecognized values are preserved and
// reported in Issues.
type ScoutCode struct {
	// Raw is the original code string.
	Raw string
	// Team is the code's team marker: * for home or a for visiting.
	Team string
	// PlayerNumber is the jersey number, or zero when unavailable.
	PlayerNumber int
	// UnknownPlayer reports the $$ unknown-player marker.
	UnknownPlayer bool
	// SkillCode is the one-character skill code from the string.
	SkillCode string
	// Skill is the decoded skill name.
	Skill string
	// SkillTypeCode is the one-character skill-type code.
	SkillTypeCode string
	// SkillType is the decoded skill-type label.
	SkillType string
	// EvaluationCode is the one-character evaluation code.
	EvaluationCode string
	// Evaluation is the label decoded under the selected convention.
	Evaluation string
	// AttackCode is the attack-combination code for an attack.
	AttackCode string
	// AttackDescription is the matching file metadata description, if available.
	AttackDescription string
	// SetCode is the setter-call code for a set.
	SetCode string
	// SetDescription is the matching file metadata description, if available.
	SetDescription string
	// SetType is the one-character target type on a set.
	SetType string
	// StartZone is the decoded starting court zone.
	StartZone string
	// EndZone is the destination zone unless Cones was selected.
	EndZone string
	// EndCone is the destination cone when Cones was selected.
	EndCone string
	// EndSubzone is the destination subzone code.
	EndSubzone string
	// SkillSubtypeCode is the one-character subtype code.
	SkillSubtypeCode string
	// SkillSubtype is the decoded subtype label.
	SkillSubtype string
	// NumberOfPlayers is the raw player-count/formation code.
	NumberOfPlayers string
	// NumberPlayersLabel is the decoded player-count/formation label.
	NumberPlayersLabel string
	// SpecialCode is the one-character special-event code.
	SpecialCode string
	// Special is the decoded special-event label.
	Special string
	// CustomCode contains text after the fixed-position code fields.
	CustomCode string
	// Point reports a point-score record.
	Point bool
	// HomeScore is the home score on a point-score record.
	HomeScore int
	// VisitingScore is the visiting score on a point-score record.
	VisitingScore int
	// Timeout reports a timeout record.
	Timeout bool
	// Substitution reports a substitution record.
	Substitution bool
	// EndOfSet reports an end-of-set record.
	EndOfSet bool
	// RotationError reports a rotation-error record.
	RotationError bool
	// SetterPosition is the setter's court position in a setter-position record.
	SetterPosition int
	// SetterTeam is the team marker in a setter-position record.
	SetterTeam string
	// Issues contains diagnostics found while interpreting the code.
	Issues []ScoutCodeIssue
}

var (
	endOfSetCode = regexp.MustCompile(`(?i)^\*\*\dset`)
	pointCode    = regexp.MustCompile(`^.p(\d+):(\d+)`)
	setterCode   = regexp.MustCompile(`^([*a])z(\d+)`)
	playerCode   = regexp.MustCompile(`^([*a])(\d+)`)
)

var attackWinningSpecialDescriptions = map[string]string{
	"S": "Block out - side",
	"O": "Block out - long",
	"F": "Block on floor",
	"X": "Direct on floor",
}

var scoutSkillNames = map[string]string{
	"S": "Serve",
	"R": "Reception",
	"A": "Attack",
	"B": "Block",
	"D": "Dig",
	"E": "Set",
	"F": "Freeball",
}

var evaluationSkillNames = map[string]string{
	"S": "serve",
	"R": "pass",
	"A": "attack",
	"B": "block",
	"D": "dig",
	"E": "set",
	"F": "freeball",
}

var skillTypeDescriptions = map[string]string{
	"H": "High ball",
	"M": "Half ball",
	"Q": "Quick ball",
	"T": "Head ball",
	"U": "Super ball",
	"F": "Fast ball",
	"N": "Slide ball",
	"O": "Other",
}

var indoorServeTypeDescriptions = map[ScoutCodeStyle]map[string]string{
	ScoutCodeStyleDefault:       {"H": "Float", "M": "Jump-float", "Q": "Jump", "T": "Topspin"},
	ScoutCodeStyleVolleyMetrics: {"H": "Float", "M": "Jump-float", "Q": "Jump", "T": "Float"},
	ScoutCodeStyleGerman:        {"H": "Float", "M": "Jump-float", "Q": "Jump", "T": "Jump"},
}

var beachServeTypeDescriptions = map[string]string{
	"H": "Standing",
	"Q": "Jump",
	"T": "Jump-float",
}

var evaluationDescriptions = map[string]map[string]string{
	"S": {
		"=": "Error", "/": "Positive, no attack", "-": "Negative, opponent free attack",
		"+": "Positive, opponent some attack", "#": "Ace", "!": "OK, no first tempo possible",
	},
	"R": {
		"=": "Error", "/": "Poor, no attack", "-": "Negative, limited attack",
		"+": "Positive, attack", "#": "Perfect pass", "!": "OK, no first tempo possible",
	},
	"A": {
		"=": "Error", "/": "Blocked", "-": "Poor, easily dug", "!": "Blocked for reattack",
		"+": "Positive, good attack", "#": "Winning attack",
	},
	"B": {
		"=": "Error", "/": "Invasion", "-": "Poor, opposition to replay",
		"+": "Positive, block touch", "#": "Winning block", "!": "Poor, opposition to replay",
	},
	"D": {
		"=": "Error", "/": "Ball directly back over net", "-": "No structured attack possible",
		"#": "Perfect dig", "+": "Good dig", "!": "OK, no first tempo possible",
	},
	"E": {"=": "Error", "-": "Poor", "/": "Poor", "+": "Positive", "#": "Perfect", "!": "OK"},
	"F": {
		"=": "Error", "/": "Poor", "!": "OK, no first tempo possible",
		"-": "OK, only high set possible", "+": "Good", "#": "Perfect",
	},
}

var volleyMetricsEvaluationDescriptions = map[string]map[string]string{
	"B": {
		"/": "Poor, opposition to replay", "!": "Poor, blocking team cannot recover",
		"-": "Poor block", "+": "Positive block",
	},
	"D": {"/": "Positive block cover", "!": "Poor block cover"},
	"E": {"/": "Error"},
}

var germanEvaluationDescriptions = map[string]map[string]string{
	"B": {"/": "Error", "=": "Invasion", "-": "Poor block"},
}

var evaluationDescriptionsByStyle = map[ScoutCodeStyle]map[string]map[string]string{
	ScoutCodeStyleVolleyMetrics: volleyMetricsEvaluationDescriptions,
	ScoutCodeStyleGerman:        germanEvaluationDescriptions,
}

var skillSubtypeDescriptions = map[string]map[string]string{
	"A": {"H": "Hard spike", "P": "Soft spike/topspin", "T": "Tip"},
	"B": {"A": "Block assist", "T": "Block attempt", "P": "Block on soft spike"},
	"R": {"L": "On left", "R": "On right", "W": "Low", "O": "Overhand", "M": "Middle line"},
	"E": {
		"1": "1 hand set", "2": "2 hands set", "3": "Bump set", "4": "Other set",
		"5": "Underhand set", "O": "Hand set", "U": "Bump set",
	},
	"D": {"S": "On spike", "C": "Spike cover", "B": "After block", "E": "Emergency", "T": "Tip", "P": "Soft spike"},
}

var blockCountDescriptions = map[string][]string{
	"indoor": {"No block", "1 player block", "2 player block", "3 player block", "Hole block"},
	"beach":  {"No block", "Line block", "Crosscourt block", "Block jumps to line", "Block jumps to crosscourt"},
}

var receptionFormationDescriptions = map[string]string{
	"1": "Two players receiving, the player on left receives",
	"2": "Two players receiving, the player on right receives",
	"3": "Three players receiving, the player on left receives",
	"4": "Three players receiving, the player in center receives",
	"5": "Three players receiving, the player on right receives",
	"6": "Four players receiving, the player on left receives",
	"7": "Four players receiving, the player on center-left receives",
	"8": "Four players receiving, the player on center-right receives",
	"9": "Four players receiving, the player on right receives",
}

// ParseScoutCode decodes a scouting-code string using default conventions.
// Unusual or custom syntax is retained and described in ScoutCode.Issues.
func ParseScoutCode(raw string) (ScoutCode, error) {
	return ParseScoutCodeWithOptions(raw, ScoutCodeOptions{})
}

// ParseScoutCodeWithOptions decodes a scouting-code string using the supplied
// conventions and metadata. Empty Style selects the default DataVolley style.
// It returns an error for an empty code or unsupported style.
func ParseScoutCodeWithOptions(raw string, options ScoutCodeOptions) (ScoutCode, error) {
	if raw == "" {
		return ScoutCode{}, errors.New("scouting code is empty")
	}
	opt, err := normalizeScoutCodeOptions(options)
	if err != nil {
		return ScoutCode{}, err
	}
	out := ScoutCode{Raw: raw}
	if parseAutomaticScoutCode(raw, &out) {
		return out, nil
	}
	body, ok := parseScoutPlayer(raw, &out)
	if !ok {
		return out, nil
	}
	decodeContactCode(body, opt, &out)
	return out, nil
}

func normalizeScoutCodeOptions(opt ScoutCodeOptions) (ScoutCodeOptions, error) {
	if opt.Style == "" {
		opt.Style = ScoutCodeStyleDefault
	}
	switch opt.Style {
	case ScoutCodeStyleDefault, ScoutCodeStyleVolleyMetrics, ScoutCodeStyleGerman:
		return opt, nil
	default:
		return ScoutCodeOptions{}, fmt.Errorf("unsupported scouting-code style %q", opt.Style)
	}
}

func parseAutomaticScoutCode(raw string, code *ScoutCode) bool {
	switch {
	case endOfSetCode.MatchString(raw):
		code.EndOfSet = true
	case strings.HasPrefix(raw, ">"):
		if strings.Contains(strings.ToUpper(raw), "ROT") {
			code.RotationError = true
			code.Skill = "Rotation error"
			code.Evaluation = "Error"
		}
	case strings.Contains(strings.ToUpper(raw), ">LUP"):
	setTeamFromCode(raw, code)
	case pointCode.MatchString(raw):
		match := pointCode.FindStringSubmatch(raw)
		setTeamFromCode(raw, code)
		code.Point = true
		code.HomeScore = parseCodeNumber(match[1], "home score", code)
		code.VisitingScore = parseCodeNumber(match[2], "visiting score", code)
	case setterCode.MatchString(raw):
		match := setterCode.FindStringSubmatch(raw)
		code.SetterTeam = match[1]
		code.SetterPosition = parseCodeNumber(match[2], "setter position", code)
	case hasTeamPrefix(raw) && (raw[1] == 'P' || raw[1] == 'c'):
		code.Team = raw[:1]
		code.Substitution = true
	case hasTeamPrefix(raw) && raw[1] == 'T':
		code.Team = raw[:1]
		code.Timeout = true
		code.Skill = "Timeout"
	case strings.HasPrefix(raw, "*$$&") || strings.HasPrefix(raw, "a$$&"):
		code.Team = raw[:1]
	default:
		return false
	}
	return true
}

func hasTeamPrefix(raw string) bool {
	return len(raw) >= 2 && (raw[0] == '*' || raw[0] == 'a')
}

func setTeamFromCode(raw string, code *ScoutCode) {
	code.Team = teamPrefix(raw)
	if code.Team == "" {
		code.addIssue("Team entries not starting with * or a", 2)
	}
}

func parseScoutPlayer(raw string, code *ScoutCode) (string, bool) {
	code.Team = teamPrefix(raw)
	if code.Team == "" {
		code.addIssue("Team entries not starting with * or a", 2)
	}
	if hasTeamPrefix(raw) && strings.HasPrefix(raw[1:], "$$") {
		code.UnknownPlayer = true
		return raw[3:], true
	}
	match := playerCode.FindStringSubmatch(raw)
	if match == nil {
		code.addIssue("Player number should start at the second character", 2)
		return "", false
	}
	code.PlayerNumber = parseCodeNumber(match[2], "player number", code)
	return raw[len(match[0]):], true
}

func parseCodeNumber(value, field string, code *ScoutCode) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		code.addIssue("Could not read numeric "+field, 2)
		return 0
	}
	return number
}

func decodeContactCode(body string, opt ScoutCodeOptions, code *ScoutCode) {
	if len(body) == 0 {
		code.addIssue("Missing skill code", 1)
		return
	}
	code.SkillCode = body[:1]
	code.Skill = skillName(code.SkillCode)
	if code.Skill == "" {
		code.Skill = "Unknown skill: " + code.SkillCode
		code.addIssue("Unexpected skill: "+code.SkillCode, 2)
	}
	if len(body) > 1 {
		code.SkillTypeCode = body[1:2]
		code.SkillType = decodeSkillType(code.SkillCode, code.SkillTypeCode, opt)
		if !validType(code.SkillCode, code.SkillTypeCode) {
			code.addIssue("Unexpected skill type: "+code.SkillTypeCode+" for skill: "+code.SkillCode, 1)
		}
	} else {
		code.addIssue("Missing skill type", 1)
	}
	if len(body) > 2 {
		code.EvaluationCode = body[2:3]
		code.Evaluation = decodeEvaluation(code.SkillCode, code.EvaluationCode, opt.Style)
		if strings.HasPrefix(strings.ToLower(code.Evaluation), "unknown") {
			code.addIssue(strings.ToUpper(code.Evaluation[:1])+code.Evaluation[1:]+": "+code.EvaluationCode, 2)
		}
	} else {
		code.addIssue("Missing evaluation code", 1)
	}
	decodeContactFields(body, opt, code)
	validateContactCode(code, opt)
	decodeSubFields(code, opt)
	decodeSpecial(code)
}

func decodeContactFields(body string, opt ScoutCodeOptions, code *ScoutCode) {
	if len(body) >= 5 {
		combination := body[3:5]
		switch code.SkillCode {
		case "A":
			code.AttackCode = combination
		case "E":
			code.SetCode = combination
		default:
			if hasCodeValue(combination) {
				code.addIssue("Unexpected non-null attack code: "+combination+" in non-attack code", 1)
			}
		}
	}
	code.SetType = codeCharAt(body, 5)
	code.StartZone = codeCharAt(body, 6)
	end := codeCharAt(body, 7)
	if code.SkillCode == "A" && opt.Cones {
		code.EndCone = end
	} else {
		code.EndZone = end
	}
	code.EndSubzone = codeCharAt(body, 8)
	code.SkillSubtypeCode = codeCharAt(body, 9)
	code.NumberOfPlayers = codeCharAt(body, 10)
	code.SpecialCode = codeCharAt(body, 11)
	if len(body) > 12 {
		code.CustomCode = body[12:]
	}
}

func codeCharAt(body string, index int) string {
	if index >= len(body) {
		return ""
	}
	return body[index : index+1]
}

func validateContactCode(code *ScoutCode, opt ScoutCodeOptions) {
	if code.Skill == "Set" && hasCodeValue(code.SetType) && !strings.Contains("FBCPS", code.SetType) {
		code.addIssue("Set type (attack target) should be F, B, C, P, or S, but is: "+code.SetType, 1)
	}
	if hasCodeValue(code.AttackCode) && opt.AttackCombinations != nil {
		var ok bool
		code.AttackDescription, ok = opt.AttackCombinations[code.AttackCode]
		if !ok {
			code.addIssue("Unmatched attack code: "+code.AttackCode, 1)
		}
	}
	if hasCodeValue(code.SetCode) && opt.SetterCalls != nil {
		var ok bool
		code.SetDescription, ok = opt.SetterCalls[code.SetCode]
		if !ok {
			code.addIssue("Unmatched set code: "+code.SetCode, 1)
		}
	}
	isServeOrReception := code.SkillCode == "S" || code.SkillCode == "R"
	if isServeOrReception && hasCodeValue(code.StartZone) && !strings.Contains("19675", code.StartZone) {
		code.addIssue("Unexpected serve/reception start zone: "+code.StartZone, 2)
	}
	if code.SkillCode == "B" && hasCodeValue(code.EndZone) && !strings.Contains("234", code.EndZone) {
		code.addIssue("Unexpected block end zone: "+code.EndZone, 2)
	}
	if hasCodeValue(code.EndSubzone) && !strings.Contains("ABCD", code.EndSubzone) {
		code.addIssue("Unexpected end subzone: "+code.EndSubzone, 1)
	}
}

func hasCodeValue(value string) bool {
	return value != "" && value != "~" && value != "~~"
}

// ValidateScoutCode parses a code using default conventions and returns its
// non-fatal syntax issues. An error indicates an empty code.
func ValidateScoutCode(raw string) ([]ScoutCodeIssue, error) {
	parsed, err := ParseScoutCode(raw)
	if err != nil {
		return nil, err
	}
	return parsed.Issues, nil
}

// ValidateScoutCodeWithOptions validates a code using the requested
// conventions and returns its non-fatal syntax issues.
func ValidateScoutCodeWithOptions(raw string, options ScoutCodeOptions) ([]ScoutCodeIssue, error) {
	parsed, err := ParseScoutCodeWithOptions(raw, options)
	if err != nil {
		return nil, err
	}
	return parsed.Issues, nil
}

func (c *ScoutCode) addIssue(message string, severity int) {
	c.Issues = append(c.Issues, ScoutCodeIssue{Message: message, Severity: severity})
}

func teamPrefix(raw string) string {
	if len(raw) > 0 && (raw[0] == '*' || raw[0] == 'a') {
		return raw[:1]
	}
	return ""
}

func skillName(skill string) string {
	return scoutSkillNames[skill]
}

func validType(skill, typ string) bool {
	if skill == "S" || skill == "R" {
		return strings.Contains("HMQT", typ)
	}
	if skill == "F" {
		return true
	}
	return strings.Contains("HMQTFUON", typ)
}

func decodeSkillType(skill, typ string, opt ScoutCodeOptions) string {
	if skill == "F" {
		return "Unknown freeball type"
	}
	if skill == "S" || skill == "R" {
		verb := "serve"
		if skill == "R" {
			verb = "serve reception"
		}
		if opt.Beach {
			label := beachServeTypeDescriptions[typ]
			if label == "" {
				return "Unknown " + verb + " type"
			}
			return label + " " + verb
		}
		label := indoorServeTypeDescriptions[opt.Style][typ]
		if label != "" {
			return label + " " + verb
		}
	}
	name := skillTypeDescriptions[typ]
	if name != "" {
		return name + " " + strings.ToLower(skillName(skill))
	}
	if skillName(skill) == "" {
		return "Unknown skill type: " + typ
	}
	return "Unknown " + strings.ToLower(skillName(skill)) + " type"
}

func decodeEvaluation(skill, code string, style ScoutCodeStyle) string {
	if label := evaluationDescriptionsByStyle[style][skill][code]; label != "" {
		return label
	}
	if value := evaluationDescriptions[skill][code]; value != "" {
		return value
	}
	name := evaluationSkillNames[skill]
	if name == "" {
		return "Cannot decode evaluation for unknown skill " + skill
	}
	return "Unknown " + name + " evaluation"
}

func decodeSubFields(c *ScoutCode, opt ScoutCodeOptions) {
	code := c.SkillSubtypeCode
	if hasCodeValue(code) {
		c.SkillSubtype = skillSubtypeDescriptions[c.SkillCode][code]
		if c.SkillSubtype == "" && strings.Contains("ABRED", c.SkillCode) {
			c.SkillSubtype = "Unknown " + code
			c.addIssue("Unexpected skill subtype: "+code, 1)
		}
	}
	if c.NumberOfPlayers == "" || c.NumberOfPlayers == "~" || c.NumberOfPlayers == "~~" {
		return
	}
	if c.SkillCode == "A" || c.SkillCode == "B" {
		labels := blockCountDescriptions["indoor"]
		if opt.Beach {
			labels = blockCountDescriptions["beach"]
		}
		if n, err := strconv.Atoi(c.NumberOfPlayers); err == nil && n >= 0 && n < len(labels) {
			c.NumberPlayersLabel = labels[n]
		} else {
			c.NumberPlayersLabel = "Unexpected " + c.NumberOfPlayers
			c.addIssue("Unexpected number of players: "+c.NumberOfPlayers, 2)
		}
	} else if c.SkillCode == "R" {
		c.NumberPlayersLabel = receptionFormationDescriptions[c.NumberOfPlayers]
		if c.NumberPlayersLabel == "" {
			c.NumberPlayersLabel = "Unexpected " + c.NumberOfPlayers
			c.addIssue("Unexpected number of players: "+c.NumberOfPlayers, 2)
		}
	}
}

var specialDescriptions = map[string]map[string]string{
	"A": {
		"S": "Attack out - side", "O": "Attack out - long", "N": "Attack in net",
		"I": "Net contact", "Z": "Referee call", "A": "Antenna", "C": "Block control",
	},
	"B": {
		"S": "Ball out - side", "O": "Ball out - long", "F": "Ball on floor",
		"X": "Between hands", "N": "Hands - net", "I": "Net contact", "A": "Antenna",
		"P": "No jump", "T": "Position error", "Z": "Referee call",
	},
	"R": {"U": "Unplayable", "X": "Body error", "P": "Position error", "Z": "Referee call", "E": "Lack of effort"},
	"F": {"U": "Unplayable", "X": "Body error", "P": "Position error", "Z": "Referee call"},
	"D": {
		"U": "Unplayable", "X": "Body error", "P": "Position error", "Z": "Referee call",
		"F": "Ball on floor", "O": "Ball out", "E": "Lack of effort",
	},
	"E": {"U": "Cannot be hit", "I": "Net touch", "Z": "Referee call"},
	"S": {
		"O": "Ball out - long", "L": "Ball out - left", "R": "Ball out - right",
		"N": "Ball in net", "Z": "Referee call",
	},
}

func decodeSpecial(code *ScoutCode) {
	if !hasCodeValue(code.SpecialCode) {
		return
	}
	if !validSpecialCode(code.SkillCode, code.EvaluationCode, code.SpecialCode) {
		code.Special = "Unexpected " + code.SpecialCode
		code.addIssue("Unexpected special code: "+code.SpecialCode+" for "+strings.ToLower(code.Skill), 1)
		return
	}
	code.Special = specialDescription(code.SkillCode, code.EvaluationCode, code.SpecialCode)
	if code.Special == "" {
		code.Special = "Unexpected " + code.SpecialCode
	}
}

func specialDescription(skill, evaluation, special string) string {
	if skill == "A" {
		switch evaluation {
		case "#":
			if special == "N" {
				return "Let"
			}
			return attackWinningSpecialDescriptions[special]
		case "+", "-", "!", "/":
			if special == "N" {
				return "Let"
			}
		}
	}
	if skill == "S" {
		if special == "N" && evaluation != "=" {
			return "Let"
		}
	}
	return specialDescriptions[skill][special]
}

var allowedSpecialCodes = map[string]string{
	"B": "SOFXNIAPTZ", "R": "UXPZE", "F": "UXPZ", "D": "UXPZFOE", "E": "UIZ",
}

func validSpecialCode(skill, evaluation, special string) bool {
	switch skill {
	case "A":
		switch evaluation {
		case "=":
			return strings.Contains("SONIZA", special)
		case "#":
			return strings.Contains("SOFXN", special)
		case "+", "-", "!", "/":
			return strings.Contains("CN", special)
		default:
			return false
		}
	case "S":
		switch evaluation {
		case "=":
			return strings.Contains("OLRNZ", special)
		case "#", "/", "-", "+", "!":
			return special == "N"
		default:
			return false
		}
	default:
		return strings.Contains(allowedSpecialCodes[skill], special)
	}
}
