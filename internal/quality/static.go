// Package quality contains provider-independent quality analysis primitives.
// Static findings are deliberately separate from real mutation execution.
package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type RuleVersion struct{ ID, Version string }

type Rule struct {
	ID, Description, Severity string
	Pattern                   string
}

type Candidate struct {
	ID, RuleID, Path string
	Line             int
	Evidence         string
}

type FindingStatus string

const (
	FindingDetected FindingStatus = "detected"
	FindingSkipped  FindingStatus = "skipped"
	FindingError    FindingStatus = "analysis_error"
)

type Finding struct {
	CandidateID, RuleID, Path string
	Line                      int
	Status                    FindingStatus
	Severity, EvidenceHash    string
}

// AnalysisResult is static analysis output. MutationScore is never populated by
// this analyzer; a static hit is not evidence that a mutant was executed.
type AnalysisResult struct {
	Rules         RuleVersion
	Findings      []Finding
	EvidenceHash  string
	MutationScore *float64
}

type QualityEvent struct{ Kind, AnalysisID, Status, EvidenceHash string }

// DefaultRules are intentionally syntax/provider independent textual safeguards.
var DefaultRules = []Rule{
	{ID: "assertion.skip", Description: "test explicitly skips execution", Severity: "high", Pattern: "t.Skip"},
	{ID: "assertion.todo", Description: "test contains an unresolved TODO", Severity: "medium", Pattern: "TODO"},
}

func Analyze(files map[string]string, version RuleVersion, rules []Rule) AnalysisResult {
	if len(rules) == 0 {
		rules = DefaultRules
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	var findings []Finding
	for _, path := range paths {
		lines := strings.Split(files[path], "\n")
		for _, rule := range rules {
			for lineNo, line := range lines {
				if strings.Contains(line, rule.Pattern) {
					id := fmt.Sprintf("%s:%d:%s", path, lineNo+1, rule.ID)
					findings = append(findings, Finding{CandidateID: id, RuleID: rule.ID, Path: path, Line: lineNo + 1, Status: FindingDetected, Severity: rule.Severity, EvidenceHash: HashEvidence(path, lineNo+1, line)})
				}
			}
		}
	}
	result := AnalysisResult{Rules: version, Findings: findings}
	result.EvidenceHash = hashResult(result)
	return result
}

func HashEvidence(path string, line int, content string) string {
	b, _ := json.Marshal(struct {
		Path    string `json:"path"`
		Line    int    `json:"line"`
		Content string `json:"content"`
	}{path, line, content})
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}

// GateInputs adapts immutable analysis output to a gate policy input. It does
// not synthesize a mutation score and preserves runner/analyzer distinctions.
func GateInputs(result AnalysisResult) map[string]any {
	detected, skipped, errors := 0, 0, 0
	for _, f := range result.Findings {
		switch f.Status {
		case FindingDetected:
			detected++
		case FindingSkipped:
			skipped++
		case FindingError:
			errors++
		}
	}
	return map[string]any{"quality": map[string]any{"rule_version": result.Rules, "static_findings": detected, "skipped": skipped, "analysis_errors": errors, "evidence_hash": result.EvidenceHash, "mutation_score": nil}}
}

func hashResult(result AnalysisResult) string {
	b, _ := json.Marshal(result)
	s := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(s[:])
}
