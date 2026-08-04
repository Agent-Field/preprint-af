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
			TransitionIssues:       []LocatedIssue{},
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
    "transition_issues": [],
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
    "paper/sections/03_results.tex:7 symbol — x < y & z"
  ]
}`
	if got := prettyJSON(compactCritique(critique)); got != want {
		t.Fatalf("repair critique JSON drifted from reference bytes\n--- Go ---\n%s\n--- Golden ---\n%s", got, want)
	}
}
