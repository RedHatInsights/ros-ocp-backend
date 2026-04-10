package utils

import (
	"math"
	"strconv"
	"strings"
)

// CalculatePercentage returns (numerator / denominator) * 100.
// If numerator or denominator is zero, it returns 0 to avoid Inf/NaN from division by zero.
func CalculatePercentage(numerator, denominator float64) float64 {
	if numerator == 0.0 || denominator == 0.0 {
		return 0.0
	}
	return (numerator / denominator) * 100
}

func hasMoreThanThreeDecimals(value float64) bool {
	const decimalPrecision = 3
	str := strconv.FormatFloat(value, 'f', -1, 64)
	decimalPart := strings.Split(str, ".")
	return len(decimalPart) > 1 && len(decimalPart[1]) > decimalPrecision
}

// TruncateToThreeDecimalPlaces matches API display rules for cores and percentage amounts
// (see internal/api transformComponentUnits / convertVariationToPercentage).
func TruncateToThreeDecimalPlaces(value float64) float64 {
	if hasMoreThanThreeDecimals(value) {
		truncated := math.Trunc(value * 1000)
		return truncated / 1000
	}
	return value
}

// TruncateMemoryBytesToMiBTwoDecimals converts bytes to MiB and truncates to two decimal places.
func TruncateMemoryBytesToMiBTwoDecimals(memoryBytes float64) float64 {
	memoryInMiB := memoryBytes / 1024 / 1024
	return math.Trunc(memoryInMiB*100) / 100
}

// TruncateMemoryBytesToGiBTwoDecimals converts bytes to GiB and truncates to two decimal places.
func TruncateMemoryBytesToGiBTwoDecimals(memoryBytes float64) float64 {
	memoryInGiB := memoryBytes / 1024 / 1024 / 1024
	return math.Trunc(memoryInGiB*100) / 100
}

// VariationPercentOfRequestCPU computes request variation as percent of current CPU request (cores),
// matching transformComponentUnits (cores) + convertVariationToPercentage.
func VariationPercentOfRequestCPU(variationCores, currentCores float64) float64 {
	v := TruncateToThreeDecimalPlaces(variationCores)
	d := TruncateToThreeDecimalPlaces(currentCores)
	p := CalculatePercentage(v, d)
	return TruncateToThreeDecimalPlaces(p)
}

// VariationPercentOfRequestMemoryBytesMiB computes request variation as percent of current memory request
// using MiB with two decimal places, matching transformComponentUnits (MiB) + convertVariationToPercentage.
func VariationPercentOfRequestMemoryBytesMiB(variationBytes, currentBytes float64) float64 {
	v := TruncateMemoryBytesToMiBTwoDecimals(variationBytes)
	d := TruncateMemoryBytesToMiBTwoDecimals(currentBytes)
	p := CalculatePercentage(v, d)
	return TruncateToThreeDecimalPlaces(p)
}
