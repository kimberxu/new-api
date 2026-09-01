package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// 保护契约:64 位钱包要求(上游 a073f74b3 deprecate int32)下,PostgreSQL/MySQL
// 的 users.quota/used_quota/aff_quota/aff_history 四列必须为 64 位整数
// (bigint/int8);历史遗留的 32 位列(integer/int)会在启动时被
// ensureUserQuotaColumns 拒绝,避免 int64 写入 32 位列静默溢出。
// 本测试用 SQLite 内存库模拟列类型 + 显式数据库类型,验证判定逻辑本身。
func TestEnsureUserQuotaColumns(t *testing.T) {
	t.Run("PostgreSQL 32 位列被拒绝", func(t *testing.T) {
		db := openTestDB(t)
		// User 模型 tag 为 type:int → SQLite 建 integer 列,模拟历史 32 位 schema
		require.NoError(t, db.AutoMigrate(&User{}))
		err := ensureUserQuotaColumns(db, common.DatabaseTypePostgreSQL)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "32-bit is not supported")
	})

	t.Run("PostgreSQL 64 位列通过", func(t *testing.T) {
		db := openTestDB(t)
		require.NoError(t, db.AutoMigrate(&bigIntWalletUser{}))
		require.NoError(t, ensureUserQuotaColumns(db, common.DatabaseTypePostgreSQL))
	})

	t.Run("SQLite 直接跳过", func(t *testing.T) {
		db := openTestDB(t)
		require.NoError(t, db.AutoMigrate(&User{}))
		require.NoError(t, ensureUserQuotaColumns(db, common.DatabaseTypeSQLite))
	})

	t.Run("无 users 表跳过", func(t *testing.T) {
		db := openTestDB(t)
		require.NoError(t, ensureUserQuotaColumns(db, common.DatabaseTypePostgreSQL))
	})

	t.Run("SKIP 开关跳过检查", func(t *testing.T) {
		t.Setenv("SKIP_64BIT_QUOTA_SCHEMA_CHECK", "true")
		db := openTestDB(t)
		require.NoError(t, db.AutoMigrate(&User{}))
		require.NoError(t, ensureUserQuotaColumns(db, common.DatabaseTypePostgreSQL))
	})
}

// bigIntWalletUser 与 User 同表名,但四列显式 bigint,模拟 64 位钱包 schema。
type bigIntWalletUser struct {
	ID              int    `gorm:"primaryKey"`
	Quota           int    `gorm:"type:bigint;default:0"`
	UsedQuota       int    `gorm:"type:bigint;default:0;column:used_quota"`
	AffQuota        int    `gorm:"type:bigint;default:0;column:aff_quota"`
	AffHistoryQuota int    `gorm:"type:bigint;default:0;column:aff_history"`
	Other           string `gorm:"type:text"`
}

func (bigIntWalletUser) TableName() string { return "users" }

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}