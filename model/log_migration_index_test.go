package model

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupLogMigrationTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	dsn := fmt.Sprintf("file:log_migration_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}

	oldLogDB := LOG_DB
	LOG_DB = db

	cleanup := func() {
		LOG_DB = oldLogDB
	}
	return db, cleanup
}

func TestMigrateLOGDBCreatesRechargeLeaderboardCompositeIndex(t *testing.T) {
	db, cleanup := setupLogMigrationTestDB(t)
	defer cleanup()

	if err := migrateLOGDB(); err != nil {
		t.Fatalf("migrateLOGDB failed: %v", err)
	}

	type indexRow struct {
		Name string `gorm:"column:name"`
	}
	var indexes []indexRow
	if err := db.Raw("PRAGMA index_list(`logs`)").Scan(&indexes).Error; err != nil {
		t.Fatalf("query index list failed: %v", err)
	}

	found := false
	for _, idx := range indexes {
		if idx.Name == "idx_logs_type_created_at_user_id" {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(indexes))
		for _, idx := range indexes {
			names = append(names, idx.Name)
		}
		t.Fatalf("missing index idx_logs_type_created_at_user_id, existing: %v", names)
	}

	type indexInfoRow struct {
		Seqno int    `gorm:"column:seqno"`
		Name  string `gorm:"column:name"`
	}
	var indexCols []indexInfoRow
	if err := db.Raw("PRAGMA index_info(`idx_logs_type_created_at_user_id`)").Scan(&indexCols).Error; err != nil {
		t.Fatalf("query index_info failed: %v", err)
	}
	sort.Slice(indexCols, func(i, j int) bool {
		return indexCols[i].Seqno < indexCols[j].Seqno
	})
	gotCols := make([]string, 0, len(indexCols))
	for _, c := range indexCols {
		gotCols = append(gotCols, c.Name)
	}
	wantCols := []string{"type", "created_at", "user_id"}
	if !reflect.DeepEqual(gotCols, wantCols) {
		t.Fatalf("unexpected index columns, got %v want %v", gotCols, wantCols)
	}
}
