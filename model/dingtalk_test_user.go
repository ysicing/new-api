package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

type DingTalkTestUser struct {
	Id            int    `json:"id"`
	Username      string `json:"username"`
	DisplayName   string `json:"display_name"`
	Email         string `json:"email"`
	Department    string `json:"department"`
	DingTalkBound bool   `json:"dingtalk_bound"`
}

func ListDingTalkTestUsers(keyword string, startIdx, pageSize int) ([]DingTalkTestUser, int64, error) {
	query := DB.Model(&User{}).
		Where("deleted_at IS NULL AND status = ? AND role <> ?", common.UserStatusEnabled, common.RoleRootUser).
		Where("email LIKE ?", "%_@_%")
	keyword = strings.TrimSpace(keyword)
	if keyword != "" {
		escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(strings.ToLower(keyword))
		like := "%" + escaped + "%"
		condition := "LOWER(username) LIKE ? ESCAPE '!' OR LOWER(display_name) LIKE ? ESCAPE '!' OR LOWER(email) LIKE ? ESCAPE '!' OR LOWER(department) LIKE ? ESCAPE '!'"
		args := []any{like, like, like, like}
		if id, err := strconv.Atoi(keyword); err == nil {
			condition = "id = ? OR " + condition
			args = append([]any{id}, args...)
		}
		query = query.Where("("+condition+")", args...)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 50 {
		pageSize = 50
	}
	var users []User
	if err := query.Select("id", "username", "display_name", "email", "department", "dingtalk_id").
		Order("id ASC").Offset(startIdx).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	items := make([]DingTalkTestUser, 0, len(users))
	for _, user := range users {
		items = append(items, DingTalkTestUser{
			Id: user.Id, Username: user.Username, DisplayName: user.DisplayName,
			Email: user.Email, Department: user.Department,
			DingTalkBound: strings.TrimSpace(user.DingTalkId) != "",
		})
	}
	return items, total, nil
}
