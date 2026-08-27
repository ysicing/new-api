package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const (
	operationsStatsSnapshotVersion = 1
	operationsStatsSnapshotLimit   = 30
)

// OperationsStatsUserSection 保存一个用户用量榜单及其实际生成时间。
type OperationsStatsUserSection struct {
	GeneratedAt int64                 `json:"generated_at"`
	Items       []model.UserQuotaStat `json:"items"`
}

// OperationsStatsRechargeSection 保存本周充值次数榜及其实际生成时间。
type OperationsStatsRechargeSection struct {
	GeneratedAt int64                    `json:"generated_at"`
	Items       []model.UserRechargeStat `json:"items"`
}

// OperationsStatsSnapshot 是运营统计接口的持久化读模型。
// 每个成功的系统任务都保存一份完整快照，失败任务不会覆盖上一份成功结果。
type OperationsStatsSnapshot struct {
	Version             int                            `json:"version"`
	WeeklyTopUsers      OperationsStatsUserSection     `json:"weekly_top_users"`
	MonthlyTopUsers     OperationsStatsUserSection     `json:"monthly_top_users"`
	RechargeLeaderboard OperationsStatsRechargeSection `json:"recharge_leaderboard"`
}

type operationsStatsRefreshHandler struct{}

func (operationsStatsRefreshHandler) Type() string {
	return model.SystemTaskTypeOperationsStatsRefresh
}

func (operationsStatsRefreshHandler) Enabled() bool { return true }

func (operationsStatsRefreshHandler) Interval() time.Duration { return 30 * time.Minute }

func (operationsStatsRefreshHandler) NewPayload() any { return struct{}{} }

func (operationsStatsRefreshHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	previous, err := GetOperationsStatsSnapshot()
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	snapshot, err := buildOperationsStatsSnapshot(ctx, time.Now(), previous)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, snapshot, ""); err != nil {
		logSystemTaskLockError(ctx, task, err)
	}
}

// GetOperationsStatsSnapshot 读取最近一次成功生成的运营统计快照。
func GetOperationsStatsSnapshot() (*OperationsStatsSnapshot, error) {
	task, err := model.GetLatestSucceededSystemTask(model.SystemTaskTypeOperationsStatsRefresh)
	if err != nil || task == nil {
		return nil, err
	}
	snapshot := &OperationsStatsSnapshot{}
	if err := task.DecodeResult(snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func buildOperationsStatsSnapshot(ctx context.Context, now time.Time, previous *OperationsStatsSnapshot) (*OperationsStatsSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := time.Date(now.Year(), now.Month(), now.Day()-weekday+1, 0, 0, 0, 0, now.Location())
	weekly, err := model.GetTopUsers(weekStart.Unix(), now.Unix(), "", operationsStatsSnapshotLimit)
	if err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	recharge, err := model.GetRechargeLeaderboardAt(operationsStatsSnapshotLimit, now)
	if err != nil {
		return nil, err
	}

	snapshot := &OperationsStatsSnapshot{
		Version: operationsStatsSnapshotVersion,
		WeeklyTopUsers: OperationsStatsUserSection{
			GeneratedAt: now.Unix(),
			Items:       weekly,
		},
		RechargeLeaderboard: OperationsStatsRechargeSection{
			GeneratedAt: now.Unix(),
			Items:       recharge,
		},
	}

	// 月榜按自然日刷新；同一天的半小时任务沿用上一次月榜，避免重复扫描近一月日志。
	if previous != nil && previous.Version == operationsStatsSnapshotVersion && previous.MonthlyTopUsers.GeneratedAt > 0 {
		previousTime := time.Unix(previous.MonthlyTopUsers.GeneratedAt, 0).In(now.Location())
		if previousTime.Year() == now.Year() && previousTime.YearDay() == now.YearDay() {
			snapshot.MonthlyTopUsers = previous.MonthlyTopUsers
			return snapshot, nil
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	monthly, err := model.GetTopUsers(now.AddDate(0, -1, 0).Unix(), now.Unix(), "", operationsStatsSnapshotLimit)
	if err != nil {
		return nil, err
	}
	snapshot.MonthlyTopUsers = OperationsStatsUserSection{
		GeneratedAt: now.Unix(),
		Items:       monthly,
	}
	return snapshot, nil
}

func init() {
	RegisterSystemTaskHandler(operationsStatsRefreshHandler{})
}
