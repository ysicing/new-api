package console_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateAnnouncementsRequiresUniquePositiveIds(t *testing.T) {
	missingId := `[{"content":"公告","publishDate":"2026-09-01T12:00:00+08:00","type":"default"}]`
	duplicateId := `[{"id":1,"content":"公告一","publishDate":"2026-09-01T12:00:00+08:00","type":"default"},{"id":1,"content":"公告二","publishDate":"2026-09-01T13:00:00+08:00","type":"warning"}]`
	unsafeId := `[{"id":9007199254740992,"content":"公告","publishDate":"2026-09-01T12:00:00+08:00","type":"default"}]`

	assert.ErrorContains(t, validateAnnouncements(missingId), "缺少有效 ID")
	assert.ErrorContains(t, validateAnnouncements(duplicateId), "ID 重复")
	assert.ErrorContains(t, validateAnnouncements(unsafeId), "缺少有效 ID")
}
