package core

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
)

// Caste is a family of genomes with the same permissions.
type Caste string

const (
	Scout     Caste = "scout"
	Worker    Caste = "worker"
	Patroller Caste = "patroller"
	Soldier   Caste = "soldier"
	Gardener  Caste = "gardener"
	Queen     Caste = "queen"
)

// Tool is one action an ageant can take. This is the complete set:
// nothing else exists from inside Zone A.
type Tool string

const (
	ToolRead         Tool = "read"
	ToolWrite        Tool = "write"
	ToolRun          Tool = "run"
	ToolDeposit      Tool = "deposit"
	ToolFlag         Tool = "flag"
	ToolProposeFrame Tool = "propose_frame"
	ToolProposePlot  Tool = "propose_plot"
	ToolSubmitGenome Tool = "submit_genome"
	ToolFinish       Tool = "finish"
)

// AllTools is the closed set, in a stable order.
var AllTools = []Tool{ToolRead, ToolWrite, ToolRun, ToolDeposit, ToolFlag,
	ToolProposeFrame, ToolProposePlot, ToolSubmitGenome, ToolFinish}

// CasteTools is the "Tools by caste" table of the architecture.
// "by genome" and "ripener" entries are allowed here and narrowed by the
// genome itself (role, commands).
var CasteTools = map[Caste][]Tool{
	Scout:     {ToolRead, ToolDeposit, ToolFinish},
	Worker:    {ToolRead, ToolWrite, ToolRun, ToolDeposit, ToolProposeFrame, ToolFinish},
	Patroller: {ToolRead, ToolDeposit, ToolFinish},
	Soldier:   {ToolRead, ToolFlag, ToolFinish},
	Gardener:  {ToolRead, ToolDeposit, ToolProposePlot, ToolFinish},
	Queen:     {ToolRead, ToolSubmitGenome, ToolFinish},
}

// RoleReceptor marks a genome that answers one yes/no question about one
// signal, instead of choosing among several (see the small-models reflection).
const RoleReceptor = "receptor"

// AnswerFound is the receptor reply mode where the model names its evidence.
const AnswerFound = "found"

// Limits are enforced by the Runner and, later, the Warden.
type Limits struct {
	Steps        int `json:"steps"`
	Seconds      int `json:"seconds"`
	OutputTokens int `json:"output_tokens"`
	Malformed    int `json:"malformed"`
	// Deposits ends the session once this many signals were accepted.
	// 0 means no limit. A scout with 1 makes exactly one choice: small
	// models in a multi-turn loop tend to repeat their own last reply.
	Deposits int `json:"deposits,omitempty"`
}

// Genome is what an ageant is started from. Immutable: a change is a new genome.
type Genome struct {
	ID          string   `json:"id"`
	Caste       Caste    `json:"caste"`
	Role        string   `json:"role"`
	Template    string   `json:"template"`
	Author      string   `json:"author"`
	Model       string   `json:"model"`
	Temperature float64  `json:"temperature"`
	Seed        int64    `json:"seed"`
	Prompt      string   `json:"prompt"`
	ReactsTo    []string `json:"reacts_to"`
	MayDeposit  []string `json:"may_deposit"`
	Tools       []Tool   `json:"tools"`
	Commands    []string `json:"commands"`
	Limits      Limits   `json:"limits"`
	Status      string   `json:"status"`
	// NoneOption makes "none" one of the scout's signals, chosen like any
	// other, instead of a separate finish action (calibration hypothesis H1:
	// ageants invent a need when saying "nothing" is a different kind of act).
	NoneOption bool `json:"none_option,omitempty"`
	// Answer selects how a receptor replies: "" (yes or no) or "found"
	// (name the evidence, or leave it empty). "found" removes the polarity
	// token that biased the first receptor run (findings, rule 6).
	Answer string `json:"answer,omitempty"`
}

// LoadGenome reads one genome as JSON, rejecting unknown fields.
func LoadGenome(r io.Reader) (Genome, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var g Genome
	if err := dec.Decode(&g); err != nil {
		return Genome{}, fmt.Errorf("genome: %w", err)
	}
	return g, nil
}

// Validate applies the rules the Tool Gateway checks on submit_genome:
// tools within the caste, deposits are requests in the dictionary.
func (g Genome) Validate(d *Dictionary) error {
	allowed, ok := CasteTools[g.Caste]
	if !ok {
		return fmt.Errorf("genome %s: unknown caste %q", g.ID, g.Caste)
	}
	if g.ID == "" || g.Prompt == "" {
		return fmt.Errorf("genome: id and prompt are required")
	}
	for _, t := range g.Tools {
		if !slices.Contains(allowed, t) {
			return fmt.Errorf("genome %s: caste %s may not hold tool %q", g.ID, g.Caste, t)
		}
	}
	if !slices.Contains(g.Tools, ToolFinish) {
		return fmt.Errorf("genome %s: finish is always required", g.ID)
	}
	if len(g.MayDeposit) > 0 && !slices.Contains(g.Tools, ToolDeposit) {
		return fmt.Errorf("genome %s: may_deposit without the deposit tool", g.ID)
	}
	for _, em := range append(append([]string{}, g.MayDeposit...), g.ReactsTo...) {
		if _, ok := d.Lookup(em); !ok {
			return fmt.Errorf("genome %s: %q is not in the dictionary", g.ID, em)
		}
	}
	for _, em := range g.MayDeposit {
		if e, _ := d.Lookup(em); e.Class != Request {
			return fmt.Errorf("genome %s: %s is a verdict and cannot be deposited", g.ID, em)
		}
	}
	l := g.Limits
	if l.Steps < 1 || l.Seconds < 1 || l.OutputTokens < 1 || l.Malformed < 1 {
		return fmt.Errorf("genome %s: every limit must be at least 1", g.ID)
	}
	if l.Deposits < 0 {
		return fmt.Errorf("genome %s: deposits limit cannot be negative", g.ID)
	}
	if g.NoneOption && (g.Role == RoleReceptor || l.Deposits != 1) {
		return fmt.Errorf("genome %s: none_option is for one-choice scouts (deposits 1, not a receptor)", g.ID)
	}
	switch {
	case g.Answer != "" && g.Answer != AnswerFound:
		return fmt.Errorf("genome %s: answer %q is not empty or %q", g.ID, g.Answer, AnswerFound)
	case g.Answer == AnswerFound && g.Role != RoleReceptor:
		return fmt.Errorf("genome %s: answer %q is for receptors", g.ID, AnswerFound)
	}
	if g.Role == RoleReceptor && (len(g.MayDeposit) != 1 || l.Deposits != 1) {
		return fmt.Errorf("genome %s: a receptor deposits exactly one signal, once", g.ID)
	}
	return nil
}

// Has reports whether the genome holds a tool.
func (g Genome) Has(t Tool) bool { return slices.Contains(g.Tools, t) }
