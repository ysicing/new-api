package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestICodeRC46DatabaseMigration(t *testing.T) {
	if os.Getenv("TEST_QUOTA_POOL_DESTRUCTIVE") != "1" {
		t.Skip("requires dedicated disposable databases")
	}
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		for _, upgrade := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/upgrade=%v", dialect, upgrade), func(t *testing.T) {
				dsn := os.Getenv("TEST_" + map[string]string{"mysql": "MYSQL", "postgres": "POSTGRES"}[dialect] + "_DSN")
				logDSN := os.Getenv("TEST_ICODE_" + map[string]string{"mysql": "MYSQL", "postgres": "POSTGRES"}[dialect] + "_LOG_DSN")
				var driver, logDriver gorm.Dialector
				switch dialect {
				case "sqlite":
					driver = sqlite.Open(t.TempDir() + "/main.db")
					logDriver = sqlite.Open(t.TempDir() + "/logs.db")
				case "mysql":
					if dsn == "" || logDSN == "" {
						t.Skip("dedicated main/log MySQL DSNs not configured")
					}
					driver = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: dsn}}}
					logDriver = mysqlMigrationDialector{mysql.Dialector{Config: &mysql.Config{DSN: logDSN}}}
				case "postgres":
					if dsn == "" || logDSN == "" {
						t.Skip("dedicated main/log PostgreSQL DSNs not configured")
					}
					driver = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: dsn, PreferSimpleProtocol: true}}}
					logDriver = postgresMigrationDialector{postgres.Dialector{Config: &postgres.Config{DSN: logDSN, PreferSimpleProtocol: true}}}
				}
				db, err := gorm.Open(driver, newGormConfig(false))
				require.NoError(t, err)
				logDB, err := gorm.Open(logDriver, newGormConfig(false))
				require.NoError(t, err)
				oldDB, oldLogDB := DB, LOG_DB
				oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
				DB, LOG_DB = db, logDB
				common.SetDatabaseTypes(common.DatabaseType(dialect), common.DatabaseType(dialect))
				initCol()
				t.Cleanup(func() {
					DB, LOG_DB = oldDB, oldLogDB
					common.SetDatabaseTypes(oldMainType, oldLogType)
					initCol()
					for _, connection := range []*gorm.DB{db, logDB} {
						sqlDB, err := connection.DB()
						require.NoError(t, err)
						assert.NoError(t, sqlDB.Close())
					}
				})
				// 环境变量必须指向可销毁的测试库；每个场景清理后验证全新启动或旧版本升级。
				for _, connection := range []*gorm.DB{db, logDB} {
					tables, err := connection.Migrator().GetTables()
					require.NoError(t, err)
					for _, table := range tables {
						require.NoError(t, connection.Migrator().DropTable(table))
					}
				}
				query := "SELECT version()"
				if dialect == "sqlite" {
					query = "SELECT sqlite_version()"
				}
				var version string
				require.NoError(t, db.Raw(query).Scan(&version).Error)
				t.Logf("database: %s, separate log database enabled", version)
				var user rc25ICodeUser
				var channel rc25ICodeChannel
				var log rc25ICodeLog
				if upgrade {
					require.NoError(t, db.AutoMigrate(&rc25ICodeUser{}, &rc25ICodeChannel{}, &QuotaPool{}, &QuotaPoolAdmin{}, &QuotaPoolTransaction{}, &Option{}))
					require.NoError(t, logDB.AutoMigrate(&rc25ICodeLog{}))
					pool := QuotaPool{Name: "旧业务池", PoolType: QuotaPoolTypeNormal, Quota: 4200, BaseQuota: 10000}
					require.NoError(t, db.Create(&pool).Error)
					require.NoError(t, db.Model(&pool).Update("enabled", false).Error)
					user = rc25ICodeUser{
						Username: "migration-root", Password: "password-hash", Email: "root@example.com", AffCode: "rc25-upgrade",
						Role: common.RoleRootUser, Status: common.UserStatusEnabled, Quota: 500, UsedQuota: 120,
						QuotaFrozen: true, QuotaPoolId: pool.Id, DingTalkId: "legacy-union", LDAPId: "root@example.com",
						Department: "研发部", AuthVersion: 3,
					}
					require.NoError(t, db.Create(&user).Error)
					limit := 5
					baseURL := "http://127.0.0.1:3430"
					// 保留旧版持久化编号；不迁移原生适配器也不能重编号存量渠道。
					channel = rc25ICodeChannel{
						Name: "旧自建池", Key: "fixture-key", Type: 64,
						Status: common.ChannelStatusAutoDisabled, MaxConcurrency: &limit, BaseURL: &baseURL,
						MagpieAutoDisabledAt: 1700000000, OtherSettings: `{"magpie_enabled":true}`,
					}
					require.NoError(t, db.Create(&channel).Error)
					log = rc25ICodeLog{UserId: user.Id, CreatedAt: 1700000000, Type: LogTypeConsume, Content: "legacy consume", Quota: 100, RequestId: "legacy-request", Other: `{"quota_pool_id":1}`}
					require.NoError(t, logDB.Create(&log).Error)
				}
				for range 2 {
					require.NoError(t, migrateDB())
					require.NoError(t, migrateLOGDB())
					require.NoError(t, syncSystemQuotaPools(db))
				}
				assert.True(t, db.Migrator().HasTable(&UserSession{}))
				assert.True(t, db.Migrator().HasTable(&UserAccessToken{}))
				assert.True(t, logDB.Migrator().HasTable(&AuditLog{}))
				if !upgrade {
					return
				}
				var storedUser rc25ICodeUser
				var storedChannel rc25ICodeChannel
				var storedLog rc25ICodeLog
				require.NoError(t, db.First(&storedUser, user.Id).Error)
				require.NoError(t, db.First(&storedChannel, channel.Id).Error)
				require.NoError(t, logDB.First(&storedLog, log.Id).Error)
				assert.Equal(t, user, storedUser, "upgrade preserves identity, role, freeze and wallet state")
				assert.Equal(t, channel, storedChannel, "upgrade preserves routing and monitor settings")
				assert.Equal(t, log, storedLog, "usage history survives the independently configured log migration")
				var claim ExternalIdentityClaim
				require.NoError(t, db.Where("provider = ? AND subject = ?", ExternalIdentityProviderDingTalk, user.DingTalkId).First(&claim).Error)
				assert.Equal(t, user.Id, claim.UserId)
				assert.Error(t, db.Create(&ExternalIdentityClaim{Provider: ExternalIdentityProviderDingTalk, Subject: user.DingTalkId, UserId: user.Id + 1}).Error)
				var pool QuotaPool
				require.NoError(t, db.First(&pool, user.QuotaPoolId).Error)
				assert.False(t, pool.Enabled)
				assert.Equal(t, 4200, pool.Quota)
				assert.Equal(t, 10000, pool.BaseQuota)
			})
		}
	}
}

func TestQuotaPoolLegacyMigrationAcrossExternalDatabases(t *testing.T) {
	if os.Getenv("TEST_QUOTA_POOL_DESTRUCTIVE") != "1" {
		t.Skip("set TEST_QUOTA_POOL_DESTRUCTIVE=1 with dedicated test databases")
	}
	tests := []struct {
		name      string
		dsn       string
		dialector gorm.Dialector
		dbType    common.DatabaseType
	}{
		{name: "mysql", dsn: os.Getenv("TEST_MYSQL_DSN"), dialector: mysql.Open(os.Getenv("TEST_MYSQL_DSN")), dbType: common.DatabaseTypeMySQL},
		{name: "postgres", dsn: os.Getenv("TEST_POSTGRES_DSN"), dialector: postgres.Open(os.Getenv("TEST_POSTGRES_DSN")), dbType: common.DatabaseTypePostgreSQL},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.dsn == "" {
				t.Skip("dedicated DSN not configured")
			}
			db, err := gorm.Open(test.dialector, &gorm.Config{})
			require.NoError(t, err)
			common.SetDatabaseTypes(test.dbType, test.dbType)
			dropQuotaPoolMigrationFixture(t, db)
			t.Cleanup(func() { dropQuotaPoolMigrationFixture(t, db) })
			seedLegacyQuotaPoolFixture(t, db)
			before := captureLegacyQuotaPoolSnapshot(t, db)

			require.NoError(t, migrateQuotaPoolSchema(db))
			require.NoError(t, migrateQuotaPoolSchema(db))

			after := captureLegacyQuotaPoolSnapshot(t, db)
			assert.Equal(t, before, after)
			assert.True(t, db.Migrator().HasColumn(&QuotaPool{}, "monthly_refill_top_up"))
			assert.True(t, db.Migrator().HasColumn(&QuotaPool{}, "budget_tag_id"))
			assert.True(t, db.Migrator().HasTable(&QuotaPoolBudgetTag{}))
			// 显式 fixture ID 不会推进 PostgreSQL sequence；迁移新增池时让 GORM 生成新 ID。
			if test.dbType == common.DatabaseTypePostgreSQL {
				require.NoError(t, db.Exec("SELECT setval(pg_get_serial_sequence('quota_pools', 'id'), (SELECT MAX(id) FROM quota_pools))").Error)
			}
			require.NoError(t, syncSystemQuotaPools(db))
			require.NoError(t, syncSystemQuotaPools(db))
			var pool QuotaPool
			require.NoError(t, db.First(&pool, 1).Error)
			assert.Equal(t, QuotaPoolTypeNormal, pool.PoolType)
			assert.False(t, pool.IsDefault)
			assert.Zero(t, pool.BaseQuota)
			assert.Zero(t, pool.Quota)
			var user legacyQuotaPoolUser
			require.NoError(t, db.First(&user, 1).Error)
			assert.Equal(t, pool.Id, user.QuotaPoolId)
			assert.Equal(t, 500, user.Quota)

		})
	}
}

func seedLegacyQuotaPoolFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&legacyQuotaPoolUser{}, &legacyQuotaPool{}, &legacyQuotaPoolAdmin{}, &legacyQuotaPoolTransaction{}, &Option{}))
	require.NoError(t, db.Create(&[]legacyQuotaPoolUser{{Id: 1, Quota: 500, UsedQuota: 120}, {Id: 2, Quota: 80, UsedQuota: 900, QuotaPoolId: 2, LDAPId: "alice@example.com", Department: "R&D"}}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPool{{Id: 1, Name: QuotaPoolDefaultName, PoolType: QuotaPoolTypeDefault, BaseQuota: -1, Quota: -1}, {Id: 2, Name: "研发一组", PoolType: QuotaPoolTypeNormal, BaseQuota: 10000, Quota: 4200}}).Error)
	require.NoError(t, db.Create(&legacyQuotaPoolAdmin{Id: 1, PoolId: 2, UserId: 2, Level: 1}).Error)
	require.NoError(t, db.Create(&[]legacyQuotaPoolTransaction{{Id: 1, PoolId: 2, Amount: 10000}, {Id: 2, PoolId: 2, Amount: -5800}}).Error)
}

func dropQuotaPoolMigrationFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable("quota_pool_transactions", "quota_pool_admins", "quota_pools", "quota_pool_budget_tags", "users", "options"))
}

// rc.25 iCode 的持久化字段快照用于验证真实升级；不要随当前模型的字段调整同步修改。
type rc25ICodeUser struct {
	Id          int     `json:"id"`
	Username    string  `json:"username" gorm:"unique;index" validate:"max=20"`
	Password    string  `json:"password" gorm:"not null;" validate:"min=8,max=20"`
	DisplayName string  `json:"display_name" gorm:"index" validate:"max=20"`
	Department  string  `json:"department" gorm:"type:varchar(512);column:department;index" validate:"max=512"`
	Role        int     `json:"role" gorm:"type:int;default:1"`   // admin, common
	Status      int     `json:"status" gorm:"type:int;default:1"` // enabled, disabled
	Email       string  `json:"email" gorm:"index" validate:"max=50"`
	GitHubId    string  `json:"github_id" gorm:"column:github_id;index"`
	DiscordId   string  `json:"discord_id" gorm:"column:discord_id;index"`
	OidcId      string  `json:"oidc_id" gorm:"column:oidc_id;index"`
	LDAPId      string  `json:"ldap_id" gorm:"type:varchar(256);column:ldap_id;index"`
	DingTalkId  string  `json:"dingtalk_id" gorm:"type:varchar(256);column:dingtalk_id;index"`
	WeChatId    string  `json:"wechat_id" gorm:"column:wechat_id;index"`
	TelegramId  string  `json:"telegram_id" gorm:"column:telegram_id;index"`
	AccessToken *string `json:"-" gorm:"type:char(32);column:access_token;uniqueIndex"` // this token is for system management
	Quota       int     `json:"quota" gorm:"type:int;default:0"`
	// 新增列需让存量用户默认保持可用，冻结不改变账户登录状态或原有余额。
	QuotaFrozen     bool           `json:"quota_frozen" gorm:"type:boolean;not null;default:false"`
	QuotaPoolId     int            `json:"quota_pool_id" gorm:"type:int;default:0;column:quota_pool_id;index"`
	UsedQuota       int            `json:"used_quota" gorm:"type:int;default:0;column:used_quota"` // used quota
	RequestCount    int            `json:"request_count" gorm:"type:int;default:0;"`               // request number
	Group           string         `json:"group" gorm:"type:varchar(64);default:'default'"`
	AffCode         string         `json:"aff_code" gorm:"type:varchar(32);column:aff_code;uniqueIndex"`
	AffCount        int            `json:"aff_count" gorm:"type:int;default:0;column:aff_count"`
	AffQuota        int            `json:"aff_quota" gorm:"type:int;default:0;column:aff_quota"`           // 邀请剩余额度
	AffHistoryQuota int            `json:"aff_history_quota" gorm:"type:int;default:0;column:aff_history"` // 邀请历史额度
	InviterId       int            `json:"inviter_id" gorm:"type:int;column:inviter_id;index"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
	LinuxDOId       string         `json:"linux_do_id" gorm:"column:linux_do_id;index"`
	Setting         string         `json:"setting" gorm:"type:text;column:setting"`
	Remark          string         `json:"remark,omitempty" gorm:"type:varchar(255)" validate:"max=255"`
	StripeCustomer  string         `json:"stripe_customer" gorm:"type:varchar(64);column:stripe_customer;index"`
	CreatedAt       int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	LastLoginAt     int64          `json:"last_login_at" gorm:"default:0;column:last_login_at"`
	AuthVersion     int64          `json:"-" gorm:"type:bigint;not null;default:1;column:auth_version"`
}

func (rc25ICodeUser) TableName() string { return "users" }

type rc25ICodeChannel struct {
	// MagpieAutoDisabledAt 是 Magpie 自动禁用的标记；由服务维护，不能从客户端写入。
	MagpieAutoDisabledAt int64 `json:"-" gorm:"type:bigint;default:0;index"`
	// MaxConcurrency 为 nil 或 0 时不限并发；按渠道 ID 共享所有模型和 Key 的名额。
	MaxConcurrency     *int    `json:"max_concurrency" gorm:"type:int"`
	Id                 int     `json:"id"`
	Type               int     `json:"type" gorm:"default:0"`
	Key                string  `json:"key" gorm:"not null"`
	OpenAIOrganization *string `json:"openai_organization"`
	TestModel          *string `json:"test_model"`
	Status             int     `json:"status" gorm:"default:1"`
	Name               string  `json:"name" gorm:"index"`
	Weight             *uint   `json:"weight" gorm:"default:0"`
	CreatedTime        int64   `json:"created_time" gorm:"bigint"`
	TestTime           int64   `json:"test_time" gorm:"bigint"`
	ResponseTime       int     `json:"response_time"` // in milliseconds
	BaseURL            *string `json:"base_url" gorm:"column:base_url;default:''"`
	Other              string  `json:"other"`
	Balance            float64 `json:"balance"` // in USD
	BalanceUpdatedTime int64   `json:"balance_updated_time" gorm:"bigint"`
	Models             string  `json:"models"`
	Group              string  `json:"group" gorm:"type:varchar(64);default:'default'"`
	UsedQuota          int64   `json:"used_quota" gorm:"bigint;default:0"`
	ModelMapping       *string `json:"model_mapping" gorm:"type:text"`
	//MaxInputTokens     *int    `json:"max_input_tokens" gorm:"default:0"`
	StatusCodeMapping *string `json:"status_code_mapping" gorm:"type:varchar(1024);default:''"`
	Priority          *int64  `json:"priority" gorm:"bigint;default:0"`
	AutoBan           *int    `json:"auto_ban" gorm:"default:1"`
	OtherInfo         string  `json:"other_info"`
	Tag               *string `json:"tag" gorm:"index"`
	Setting           *string `json:"setting" gorm:"type:text"` // 渠道额外设置
	ParamOverride     *string `json:"param_override" gorm:"type:text"`
	HeaderOverride    *string `json:"header_override" gorm:"type:text"`
	Remark            *string `json:"remark" gorm:"type:varchar(255)" validate:"max=255"`
	// add after v0.8.5
	ChannelInfo ChannelInfo `json:"channel_info" gorm:"type:json"`

	OtherSettings string `json:"settings" gorm:"column:settings"` // 其他设置，存储azure版本等不需要检索的信息，详见dto.ChannelOtherSettings

	// cache info
}

func (rc25ICodeChannel) TableName() string { return "channels" }

type rc25ICodeLog struct {
	Id                int    `json:"id" gorm:"index:idx_created_at_id,priority:2;index:idx_user_id_id,priority:2"`
	UserId            int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:1;index:idx_created_at_type"`
	Type              int    `json:"type" gorm:"index:idx_created_at_type"`
	Content           string `json:"content"`
	Username          string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName         string `json:"token_name" gorm:"index;default:''"`
	ModelName         string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota             int    `json:"quota" gorm:"default:0"`
	PromptTokens      int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens  int    `json:"completion_tokens" gorm:"default:0"`
	UseTime           int    `json:"use_time" gorm:"default:0"`
	IsStream          bool   `json:"is_stream"`
	ChannelId         int    `json:"channel" gorm:"index"`
	ChannelName       string `json:"channel_name" gorm:"->"`
	TokenId           int    `json:"token_id" gorm:"default:0;index"`
	Group             string `json:"group" gorm:"index"`
	Ip                string `json:"ip" gorm:"index;default:''"`
	RequestId         string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	UpstreamRequestId string `json:"upstream_request_id,omitempty" gorm:"type:varchar(128);index:idx_logs_upstream_request_id;default:''"`
	Other             string `json:"other"`
}

func (rc25ICodeLog) TableName() string { return "logs" }
