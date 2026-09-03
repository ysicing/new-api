package service

import (
	"bytes"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func quotaPoolStatsExportFixture() *model.QuotaPoolStats {
	return &model.QuotaPoolStats{
		Preset: "custom", Granularity: model.QuotaPoolStatsGranularityDay,
		StartTimestamp: 1785686400, EndTimestamp: 1785772800,
		StartTime: "2026-08-03 00:00:00 +08:00 CST", EndTime: "2026-08-04 00:00:00 +08:00 CST", GeneratedAt: 1785686400,
		Summary: model.QuotaPoolStatsSummary{MemberCount: 2, ActiveMembers: 1, ActiveRate: 50, RequestCount: 3, TotalUsage: 1_250_000, AverageUsagePerActiveMember: 1_250_000, TotalTokens: 1235, AverageTokensPerActiveMember: 1235},
		Trend: []model.QuotaPoolTrendStat{
			{Label: "2026-08-03", ActiveMembers: 1, ActiveRate: 50, RequestCount: 3, UsedQuota: 1_250_000, TokenUsed: 1235},
			{Label: "2026-08-04"},
		},
		Members: []model.QuotaPoolMemberStat{
			{QuotaPoolUsageStat: model.QuotaPoolUsageStat{UserId: 1, Username: "alice|研发\n一组", RequestCount: 3, UsedQuota: 1_250_000, TokenUsed: 1235, GptQuota: 1_250_000}, Active: true, ActiveDays: 2, LastActiveAt: 1785686400, UsageShare: 100, AverageDailyUsage: 625_000, AverageDailyTokens: 617.5},
			{QuotaPoolUsageStat: model.QuotaPoolUsageStat{UserId: 2, Username: "=cmd", RequestCount: 0}, Active: false},
		},
		Recharge:    []model.QuotaPoolRechargeStat{{Type: model.QuotaPoolTransactionManualRefill, Count: 1, Amount: 200}},
		TotalRefill: 200,
	}
}

func TestExportQuotaPoolStatsXLSXCreatesSheetsAndNeutralizesFormulaText(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	data, filename, contentType, err := ExportQuotaPoolStats("xlsx", "产研中心", quotaPoolStatsExportFixture())

	require.NoError(t, err)
	assert.Equal(t, "产研中心_20260803-0000_20260804-0000.xlsx", filename)
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", contentType)
	book, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, book.Close()) })
	assert.Equal(t, []string{"概览", "走势明细", "成员明细"}, book.GetSheetList())
	value, err := book.GetCellValue("成员明细", "A3")
	require.NoError(t, err)
	assert.Equal(t, "'=cmd", value)
	assert.Equal(t, "Token量", mustQuotaPoolStatsCellValue(t, book, "成员明细", "F1"))
	assert.Equal(t, "费用", mustQuotaPoolStatsCellValue(t, book, "成员明细", "G1"))
	assert.Equal(t, "日均Token量", mustQuotaPoolStatsCellValue(t, book, "成员明细", "I1"))
	assert.Equal(t, "日均费用", mustQuotaPoolStatsCellValue(t, book, "成员明细", "J1"))
	assert.Equal(t, "1235.00", mustQuotaPoolStatsCellValue(t, book, "成员明细", "F2"))
	assert.Equal(t, "$2.50", mustQuotaPoolStatsCellValue(t, book, "成员明细", "G2"))
	assert.Equal(t, "617.50", mustQuotaPoolStatsCellValue(t, book, "成员明细", "I2"))
	assert.Equal(t, "$1.25", mustQuotaPoolStatsCellValue(t, book, "成员明细", "J2"))
}

func TestExportQuotaPoolStatsMarkdownIncludesAllSectionsAndEscapesCells(t *testing.T) {
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500_000
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })
	data, filename, contentType, err := ExportQuotaPoolStats("markdown", "产研/中心", quotaPoolStatsExportFixture())

	require.NoError(t, err)
	assert.Equal(t, "产研_中心_20260803-0000_20260804-0000.md", filename)
	assert.Equal(t, "text/markdown; charset=utf-8", contentType)
	content := string(data)
	assert.Contains(t, content, "# 产研/中心额度池统计")
	assert.Contains(t, content, "## 概览")
	assert.Contains(t, content, "## 走势明细")
	assert.Contains(t, content, "## 成员明细")
	assert.NotContains(t, content, "## 资金变动")
	assert.Contains(t, content, "alice\\|研发<br>一组")
	assert.Contains(t, content, "| 总Token量 | 1235.00 |")
	assert.Contains(t, content, "| 总费用 | $2.50 |")
	assert.Contains(t, content, "| 活跃成员人均Token量 | 1235.00 |")
	assert.Contains(t, content, "| 活跃成员人均费用 | $2.50 |")
	assert.Contains(t, content, "| Token量 | 费用 |")
	assert.Contains(t, content, "| 日均Token量 | 日均费用 |")
	assert.Contains(t, content, "| 1235.00 | $2.50 | 100.00% | 617.50 | $1.25 |")
}

func mustQuotaPoolStatsCellValue(t *testing.T, book *excelize.File, sheet, cell string) string {
	t.Helper()
	value, err := book.GetCellValue(sheet, cell)
	require.NoError(t, err)
	return value
}

func TestExportQuotaPoolStatsMarkdownNeutralizesLinksAndImages(t *testing.T) {
	stats := quotaPoolStatsExportFixture()
	stats.Members[0].Username = `![](https://tracker.example/pixel)`

	data, _, _, err := ExportQuotaPoolStats("markdown", `[恶意池](https://tracker.example)`, stats)

	require.NoError(t, err)
	content := string(data)
	assert.NotContains(t, content, `![](`)
	assert.NotContains(t, content, `](https://tracker.example)`)
	assert.Contains(t, content, `\!\[\]\(https://tracker\.example/pixel\)`)
}
