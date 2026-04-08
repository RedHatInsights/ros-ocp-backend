package utils

// CalculatePercentage returns (numerator / denominator) * 100.
// If numerator or denominator is zero, it returns 0 to avoid Inf/NaN from division by zero.
func CalculatePercentage(numerator, denominator float64) float64 {
	if numerator == 0.0 || denominator == 0.0 {
		return 0.0
	}
	return (numerator / denominator) * 100
}
