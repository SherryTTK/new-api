package common

import "github.com/shopspring/decimal"

func GetTrustQuota() int {
	return int(10 * QuotaPerUnit)
}

func USDToQuota(amountUSD float64) int {
	if amountUSD <= 0 || QuotaPerUnit <= 0 {
		return 0
	}
	return int(decimal.NewFromFloat(amountUSD).
		Mul(decimal.NewFromFloat(QuotaPerUnit)).
		Round(0).
		IntPart())
}

func QuotaToUSD(quota int64) float64 {
	if quota == 0 || QuotaPerUnit == 0 {
		return 0
	}
	return decimal.NewFromInt(quota).
		Div(decimal.NewFromFloat(QuotaPerUnit)).
		InexactFloat64()
}
