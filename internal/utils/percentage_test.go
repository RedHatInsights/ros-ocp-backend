package utils

import "testing"

// Regression: MiB 2dp truncation before percent must match API ordering (DB sort vs displayed %).
func TestVariationPercentOfRequestMemoryBytesMiB_rowOrderMatchesDisplay(t *testing.T) {
	const (
		rowAVar = 1152385
		rowACur = 3146804
		rowBVar = 1143997
		rowBCur = 3144707
	)

	rawA := CalculatePercentage(float64(rowAVar), float64(rowACur))
	rawB := CalculatePercentage(float64(rowBVar), float64(rowBCur))
	if rawA <= rawB {
		t.Fatalf("raw byte ratios: expected row A > row B (%.4f vs %.4f)", rawA, rawB)
	}

	pctA := VariationPercentOfRequestMemoryBytesMiB(float64(rowAVar), float64(rowACur))
	pctB := VariationPercentOfRequestMemoryBytesMiB(float64(rowBVar), float64(rowBCur))
	if pctA >= pctB {
		t.Fatalf("MiB-aligned %%: expected row B > row A (%.4f vs %.4f)", pctB, pctA)
	}
}
