package pipeline

import "testing"

func TestCompactRepairJSONMatchesReferenceBytes(t *testing.T) {
	critique := CritiqueBundle{
		PersonaReviews: []PersonaReview{{
			AcceptanceRisk: .25,
			Verdict:        "Use <measured> evidence & revise.",
			Issues: []LocatedIssue{{
				Section: "results", Issue: "Claim < evidence", FixHint: "Use A & B", Severity: "major",
			}},
		}},
		Narrative: NarrativeReview{
			TransitionIssues: []LocatedIssue{{
				Section: "discussion", Issue: "Jump", FixHint: "Bridge", Severity: "major",
			}},
			PromiseAlignmentIssues: []string{"abstract > body"},
			ArcAssessment:          "A & B",
		},
		Fidelity: FidelityAudit{
			UnsupportedClaims: []string{"x < y"},
			NumberMismatches:  []string{},
			CitationIssues:    []string{"A & B"},
			Blocking:          true,
		},
		Slop: SlopReport{Violations: []SlopViolation{{
			File: "paper/sections/03_results.tex", Line: 7, Rule: "symbol", Excerpt: "x < y & z",
		}}},
	}
	want := `{
  "personas": [
    {
      "acceptance_risk": 0.25,
      "verdict": "Use <measured> evidence & revise.",
      "issues": [
        {
          "section": "results",
          "issue": "Claim < evidence",
          "fix_hint": "Use A & B",
          "severity": "major"
        }
      ]
    }
  ],
  "narrative": {
    "transition_issues": [
      {
        "section": "discussion",
        "issue": "Jump",
        "fix_hint": "Bridge"
      }
    ],
    "promise_alignment_issues": [
      "abstract > body"
    ],
    "arc_assessment": "A & B"
  },
  "fidelity": {
    "blocking": true,
    "unsupported_claims": [
      "x < y"
    ],
    "number_mismatches": [],
    "citation_issues": [
      "A & B"
    ]
  },
  "slop": [
    "paper/sections/03_results.tex:7 symbol \u2014 x < y & z"
  ]
}`
	if got := pythonPrettyJSON(compactCritique(critique)); got != want {
		t.Fatalf("repair critique JSON drifted from reference bytes\n--- Go ---\n%s\n--- Golden ---\n%s", got, want)
	}
}

func TestCompactRepairOrdersMajorsBeforeFirstThreeMinors(t *testing.T) {
	critique := CritiqueBundle{PersonaReviews: []PersonaReview{{Issues: []LocatedIssue{
		{Issue: "minor one", Severity: "minor"},
		{Issue: "major one", Severity: "major"},
		{Issue: "minor two", Severity: "minor"},
		{Issue: "minor three", Severity: "minor"},
		{Issue: "minor four", Severity: "minor"},
		{Issue: "major two", Severity: "MAJOR"},
	}}}}
	issues := compactCritique(critique).Personas[0].Issues
	want := []string{"major one", "major two", "minor one", "minor two", "minor three"}
	if len(issues) != len(want) {
		t.Fatalf("kept %d issues, want %d", len(issues), len(want))
	}
	for i := range want {
		if issues[i].Issue != want[i] {
			t.Fatalf("issue %d = %q, want %q", i, issues[i].Issue, want[i])
		}
	}
}
