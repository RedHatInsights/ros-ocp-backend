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

// Regression: extreme (variation/current) must not exceed PostgreSQL NUMERIC(10,4) (|v| < 1e6).
func TestVariationPercentOfRequest_clampsToNumeric10_4(t *testing.T) {
	// After 3dp cores truncation, current stays 0.001; (100/0.001)*100 = 1e7 %, unclamped
	cpu := VariationPercentOfRequestCPU(100.0, 0.001)
	if cpu != maxNumeric10_4Magnitude {
		t.Fatalf("CPU: got %v, want %v", cpu, maxNumeric10_4Magnitude)
	}
	// 1 PiB-like variation vs 1 MiB current request → |%| > 1e6 before clamp
	mem := VariationPercentOfRequestMemoryBytesMiB(1e12, 1048576)
	if mem != maxNumeric10_4Magnitude {
		t.Fatalf("memory: got %v, want %v", mem, maxNumeric10_4Magnitude)
	}
}
