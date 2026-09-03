package service

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/xuri/excelize/v2"
)

var errQuotaPoolStatsExportFormat = errors.New("unsupported quota pool statistics export format")

func ExportQuotaPoolStats(format, poolName string, stats *model.QuotaPoolStats) ([]byte, string, string, error) {
	if stats == nil {
		return nil, "", "", errQuotaPoolStatsExportFormat
	}
	location := common.BeijingTimeLocation
	startLabel := time.Unix(stats.StartTimestamp, 0).In(location).Format("20060102-1504")
	endLabel := time.Unix(stats.EndTimestamp, 0).In(location).Format("20060102-1504")
	filenameBase := sanitizeQuotaPoolStatsFilename(poolName) + "_" + startLabel + "_" + endLabel
	switch format {
	case "markdown":
		return renderQuotaPoolStatsMarkdown(poolName, stats), filenameBase + ".md", "text/markdown; charset=utf-8", nil
	case "xlsx":
		data, err := renderQuotaPoolStatsXLSX(stats)
		return data, filenameBase + ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", err
	default:
		return nil, "", "", errQuotaPoolStatsExportFormat
	}
}

func renderQuotaPoolStatsMarkdown(poolName string, stats *model.QuotaPoolStats) []byte {
	var output bytes.Buffer
	fmt.Fprintf(&output, "# %s额度池统计\n\n", markdownTableCell(poolName))
	fmt.Fprintf(&output, "统计区间：%s 至 %s  \n", stats.StartTime, stats.EndTime)
	fmt.Fprintf(&output, "走势颗粒度：%s  \n", stats.Granularity)
	fmt.Fprintf(&output, "统计时区：%s  \n", stats.TimeZone)
	fmt.Fprintf(&output, "数据生成时间：%s\n\n", formatQuotaPoolStatsTime(stats.GeneratedAt))
	output.WriteString("## 概览\n\n")
	output.WriteString("| 指标 | 数值 |\n| --- | ---: |\n")
	fmt.Fprintf(&output, "| 成员数 | %d |\n", stats.Summary.MemberCount)
	fmt.Fprintf(&output, "| 活跃成员数 | %d |\n", stats.Summary.ActiveMembers)
	fmt.Fprintf(&output, "| 活跃率 | %.2f%% |\n", stats.Summary.ActiveRate)
	fmt.Fprintf(&output, "| 调用次数 | %d |\n", stats.Summary.RequestCount)
	fmt.Fprintf(&output, "| 总Token量 | %.2f |\n", float64(stats.Summary.TotalTokens))
	fmt.Fprintf(&output, "| 总费用 | $%.2f |\n", quotaPoolStatsCost(stats.Summary.TotalUsage))
	fmt.Fprintf(&output, "| 活跃成员人均Token量 | %.2f |\n", stats.Summary.AverageTokensPerActiveMember)
	fmt.Fprintf(&output, "| 活跃成员人均费用 | $%.2f |\n", quotaPoolStatsCostFloat(stats.Summary.AverageUsagePerActiveMember))
	fmt.Fprintf(&output, "| 总充值（内部额度单位） | %d |\n", stats.TotalRefill)
	fmt.Fprintf(&output, "| 总分配（内部额度单位） | %d |\n", stats.TotalAllocate)
	fmt.Fprintf(&output, "| 总回收（内部额度单位） | %d |\n\n", stats.TotalReclaim)

	output.WriteString("## 走势明细\n\n")
	output.WriteString("| 时间 | 活跃成员 | 活跃率 | 调用次数 | Token量 | 费用 |\n| --- | ---: | ---: | ---: | ---: | ---: |\n")
	for _, item := range stats.Trend {
		fmt.Fprintf(&output, "| %s | %d | %.2f%% | %d | %.2f | $%.2f |\n",
			markdownTableCell(item.Label), item.ActiveMembers, item.ActiveRate, item.RequestCount,
			float64(item.TokenUsed), quotaPoolStatsCost(item.UsedQuota),
		)
	}

	output.WriteString("\n## 成员明细\n\n")
	output.WriteString("| 成员 | 状态 | 活跃天数 | 最后活跃时间 | 调用次数 | Token量 | 费用 | 费用占比 | 日均Token量 | 日均费用 | 模型占比 |\n")
	output.WriteString("| --- | --- | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, item := range stats.Members {
		status := "未活跃"
		if item.Active {
			status = "活跃"
		}
		lastActive := formatQuotaPoolStatsTime(item.LastActiveAt)
		fmt.Fprintf(&output, "| %s | %s | %d | %s | %d | %.2f | $%.2f | %.2f%% | %.2f | $%.2f | %s |\n",
			markdownTableCell(item.Username), status, item.ActiveDays, lastActive, item.RequestCount,
			float64(item.TokenUsed), quotaPoolStatsCost(item.UsedQuota), item.UsageShare,
			item.AverageDailyTokens, quotaPoolStatsCostFloat(item.AverageDailyUsage), markdownTableCell(quotaPoolModelShareText(item.QuotaPoolUsageStat)),
		)
	}

	// 资金变动暂不对外导出，统计响应仍保留相关数据供页面展示。
	return output.Bytes()
}

func renderQuotaPoolStatsXLSX(stats *model.QuotaPoolStats) ([]byte, error) {
	book := excelize.NewFile()
	defer book.Close()
	if err := book.SetSheetName("Sheet1", "概览"); err != nil {
		return nil, err
	}
	overviewRows := [][]any{
		{"指标", "数值"},
		{"统计开始时间", stats.StartTime}, {"统计结束时间", stats.EndTime},
		{"走势颗粒度", string(stats.Granularity)},
		{"统计时区", stats.TimeZone},
		{"数据生成时间", formatQuotaPoolStatsTime(stats.GeneratedAt)},
		{"成员数", stats.Summary.MemberCount}, {"活跃成员数", stats.Summary.ActiveMembers},
		{"活跃率", stats.Summary.ActiveRate / 100}, {"调用次数", stats.Summary.RequestCount},
		{"总Token量", stats.Summary.TotalTokens}, {"总费用", quotaPoolStatsCost(stats.Summary.TotalUsage)},
		{"活跃成员人均Token量", stats.Summary.AverageTokensPerActiveMember}, {"活跃成员人均费用", quotaPoolStatsCostFloat(stats.Summary.AverageUsagePerActiveMember)},
		{"总充值（内部额度单位）", stats.TotalRefill}, {"总分配（内部额度单位）", stats.TotalAllocate}, {"总回收（内部额度单位）", stats.TotalReclaim},
	}
	if err := writeQuotaPoolStatsSheet(book, "概览", overviewRows); err != nil {
		return nil, err
	}

	trendRows := [][]any{{"时间", "活跃成员", "活跃率", "调用次数", "Token量", "费用"}}
	for _, item := range stats.Trend {
		trendRows = append(trendRows, []any{item.Label, item.ActiveMembers, item.ActiveRate / 100, item.RequestCount, item.TokenUsed, quotaPoolStatsCost(item.UsedQuota)})
	}
	if err := writeQuotaPoolStatsSheet(book, "走势明细", trendRows); err != nil {
		return nil, err
	}

	memberRows := [][]any{{"成员", "状态", "活跃天数", "最后活跃时间", "调用次数", "Token量", "费用", "费用占比", "日均Token量", "日均费用", "模型占比"}}
	for _, item := range stats.Members {
		status := "未活跃"
		if item.Active {
			status = "活跃"
		}
		memberRows = append(memberRows, []any{
			excelSafeText(item.Username), status, item.ActiveDays, formatQuotaPoolStatsTime(item.LastActiveAt),
			item.RequestCount, item.TokenUsed, quotaPoolStatsCost(item.UsedQuota), item.UsageShare / 100,
			item.AverageDailyTokens, quotaPoolStatsCostFloat(item.AverageDailyUsage),
			excelSafeText(quotaPoolModelShareText(item.QuotaPoolUsageStat)),
		})
	}
	if err := writeQuotaPoolStatsSheet(book, "成员明细", memberRows); err != nil {
		return nil, err
	}

	// 资金变动暂不对外导出，统计响应仍保留相关数据供页面展示。
	percentageStyle, err := book.NewStyle(&excelize.Style{NumFmt: 10})
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("概览", "B9", "B9", percentageStyle); err != nil {
		return nil, err
	}
	twoDecimalStyle, err := book.NewStyle(&excelize.Style{NumFmt: 2})
	if err != nil {
		return nil, err
	}
	currencyFormat := `"$"0.00`
	currencyStyle, err := book.NewStyle(&excelize.Style{CustomNumFmt: &currencyFormat})
	if err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("概览", "B11", "B11", twoDecimalStyle); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("概览", "B12", "B12", currencyStyle); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("概览", "B13", "B13", twoDecimalStyle); err != nil {
		return nil, err
	}
	if err := book.SetCellStyle("概览", "B14", "B14", currencyStyle); err != nil {
		return nil, err
	}
	if len(stats.Trend) > 0 {
		if err := book.SetCellStyle("走势明细", "C2", fmt.Sprintf("C%d", len(stats.Trend)+1), percentageStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("走势明细", "E2", fmt.Sprintf("E%d", len(stats.Trend)+1), twoDecimalStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("走势明细", "F2", fmt.Sprintf("F%d", len(stats.Trend)+1), currencyStyle); err != nil {
			return nil, err
		}
	}
	if len(stats.Members) > 0 {
		if err := book.SetCellStyle("成员明细", "F2", fmt.Sprintf("F%d", len(stats.Members)+1), twoDecimalStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("成员明细", "G2", fmt.Sprintf("G%d", len(stats.Members)+1), currencyStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("成员明细", "H2", fmt.Sprintf("H%d", len(stats.Members)+1), percentageStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("成员明细", "I2", fmt.Sprintf("I%d", len(stats.Members)+1), twoDecimalStyle); err != nil {
			return nil, err
		}
		if err := book.SetCellStyle("成员明细", "J2", fmt.Sprintf("J%d", len(stats.Members)+1), currencyStyle); err != nil {
			return nil, err
		}
	}
	book.SetActiveSheet(0)
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeQuotaPoolStatsSheet(book *excelize.File, sheet string, rows [][]any) error {
	if sheet != "概览" {
		if _, err := book.NewSheet(sheet); err != nil {
			return err
		}
	}
	for rowIndex, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+1)
		if err != nil {
			return err
		}
		if err := book.SetSheetRow(sheet, cell, &row); err != nil {
			return err
		}
	}
	headerStyle, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	lastColumn, err := excelize.ColumnNumberToName(len(rows[0]))
	if err != nil {
		return err
	}
	if err := book.SetCellStyle(sheet, "A1", lastColumn+"1", headerStyle); err != nil {
		return err
	}
	return book.SetColWidth(sheet, "A", lastColumn, 18)
}

func excelSafeText(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}

func quotaPoolStatsCost(quota int) float64 {
	return quotaPoolStatsCostFloat(float64(quota))
}

func quotaPoolStatsCostFloat(quota float64) float64 {
	if common.QuotaPerUnit <= 0 {
		return 0
	}
	return quota / common.QuotaPerUnit
}

func formatQuotaPoolStatsTime(timestamp int64) string {
	if timestamp <= 0 {
		return "-"
	}
	return time.Unix(timestamp, 0).In(common.BeijingTimeLocation).Format("2006-01-02 15:04:05 -07:00 MST")
}

func markdownTableCell(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	for _, marker := range []string{"`", "*", "_", "{", "}", "[", "]", "(", ")", "#", "+", "-", ".", "!", "|"} {
		value = strings.ReplaceAll(value, marker, "\\"+marker)
	}
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, "\r\n", "<br>")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return strings.ReplaceAll(value, "\r", "<br>")
}

func sanitizeQuotaPoolStatsFilename(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`/\\:*?"<>|`, r) {
			return '_'
		}
		return r
	}, strings.TrimSpace(value))
	value = strings.Trim(value, ". ")
	if value == "" {
		return "quota_pool"
	}
	if runes := []rune(value); len(runes) > 80 {
		value = string(runes[:80])
	}
	return value
}

func quotaPoolModelShareText(item model.QuotaPoolUsageStat) string {
	if item.UsedQuota == 0 {
		return "-"
	}
	shares := []struct {
		name  string
		quota int
	}{
		{name: "GPT", quota: item.GptQuota}, {name: "Claude", quota: item.ClaudeQuota},
		{name: "DeepSeek", quota: item.DeepSeekQuota}, {name: "Gemini", quota: item.GeminiQuota},
		{name: "Qwen", quota: item.QwenQuota}, {name: "Other", quota: item.OtherQuota},
	}
	parts := make([]string, 0, len(shares))
	for _, share := range shares {
		if share.quota != 0 {
			parts = append(parts, fmt.Sprintf("%s %.2f%%", share.name, float64(share.quota)*100/float64(item.UsedQuota)))
		}
	}
	return strings.Join(parts, ", ")
}
