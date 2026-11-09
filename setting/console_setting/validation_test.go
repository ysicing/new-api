package console_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAnnouncementsRequiresUniquePositiveIds(t *testing.T) {
	missingId := `[{"content":"公告","publishDate":"2026-09-01T12:00:00+08:00","type":"default"}]`
	duplicateId := `[{"id":1,"content":"公告一","publishDate":"2026-09-01T12:00:00+08:00","type":"default"},{"id":1,"content":"公告二","publishDate":"2026-09-01T13:00:00+08:00","type":"warning"}]`
	unsafeId := `[{"id":9007199254740992,"content":"公告","publishDate":"2026-09-01T12:00:00+08:00","type":"default"}]`

	cases := []struct {
		name    string
		payload string
		key     string
		index   int
		message string
	}{
		{"missing ID", missingId, "Announcement #{{index}} is missing a valid ID", 1, "Announcement #1 is missing a valid ID"},
		{"duplicate ID", duplicateId, "Announcement #{{index}} has a duplicate ID", 2, "Announcement #2 has a duplicate ID"},
		{"unsafe ID", unsafeId, "Announcement #{{index}} is missing a valid ID", 1, "Announcement #1 is missing a valid ID"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateConsoleSettings(testCase.payload, "Announcements")
			var message *common.Message
			require.ErrorAs(t, err, &message)
			assert.EqualError(t, err, testCase.message)
			assert.Equal(t, testCase.key, message.Fields()["message_key"])
			assert.Equal(t, map[string]any{"index": testCase.index}, message.Fields()["message_params"])
		})
	}
}
