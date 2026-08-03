package pipeline

import "testing"

func TestParseEvidenceSummaryFromDurableLedger(t *testing.T) {
	body := `# Evidence

## Facts

### E1
- **Statement:** The measured result is exact.
- **Numbers:** 3.2x

## Figure candidates
- Plot the measured result from input/results.csv.

## Gaps
- Run three additional seeds.

## Existing draft
The existing draft overclaims the measured result.

## Citation inventory
- smith2024: Verified title
` + string(make([]byte, 500))
	got := parseEvidenceSummary(body)
	if got.FactCount != 1 || !got.Confident || len(got.Gaps) != 1 || len(got.FigureCandidates) != 1 || len(got.ExistingCitations) != 1 || got.StrongestFactualThesis != "The measured result is exact." {
		t.Fatalf("unexpected summary: %#v", got)
	}
}
