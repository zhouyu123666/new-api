package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestTextBillingCostDetailsUsesSettledQuotaAsTotal(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	summary := textQuotaSummary{
		Quota:       1_500,
		TotalTokens: 1,
		CostSplit:   true,
		InputQuota:  decimal.NewFromInt(1_000),
		OutputQuota: decimal.NewFromInt(500),
	}
	costs := textBillingCostDetails(summary, true)
	require.NotNil(t, costs)
	require.NotNil(t, costs.InputUSD)
	require.NotNil(t, costs.OutputUSD)
	require.InDelta(t, 0.002, *costs.InputUSD, 1e-12)
	require.InDelta(t, 0.001, *costs.OutputUSD, 1e-12)
	require.InDelta(t, 0.003, costs.TotalUSD, 1e-12)
}

func TestTextBillingCostDetailsOmitsUnknownZeroUsage(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	// A missing upstream usage must not become an authoritative $0 cost point
	// in Langfuse; it remains unknown unless the gateway actually settled quota.
	costs := textBillingCostDetails(textQuotaSummary{Quota: 0}, false)
	require.Nil(t, costs)
}

func TestTextBillingCostDetailsFallsBackToTotalWhenSplitIsUnsafe(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	summary := textQuotaSummary{
		Quota:       1,
		TotalTokens: 1,
		CostSplit:   true,
		InputQuota:  decimal.NewFromFloat(2.5),
		OutputQuota: decimal.Zero,
	}
	costs := textBillingCostDetails(summary, true)
	require.NotNil(t, costs)
	require.Nil(t, costs.InputUSD)
	require.Nil(t, costs.OutputUSD)
	require.InDelta(t, 0.000002, costs.TotalUSD, 1e-12)
}
