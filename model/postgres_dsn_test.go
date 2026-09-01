package model

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 保护契约:personal 为兼容 PgBouncer/Supabase 固定开启 PreferSimpleProtocol。
// pgx 的 sanitizeForSimpleQuery 强制要求 client_encoding=UTF8 与
// standard_conforming_strings=on,否则所有带参数 raw SQL(HasTable、
// migratePrefillGroupUniqueness 等)在连接编码非 UTF8 的库上启动即 FATAL。
// normalizePostgresDSN 兜底补两参数;用户显式携带时尊重覆盖值,不重复。
func TestNormalizePostgresDSN(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantEnc string
		wantSCS string
	}{
		{
			name:    "no query parameters",
			dsn:     "postgresql://user:pass@host:5432/mydb",
			wantEnc: "UTF8",
			wantSCS: "on",
		},
		{
			name:    "existing sslmode parameter",
			dsn:     "postgresql://user:pass@host:5432/mydb?sslmode=require",
			wantEnc: "UTF8",
			wantSCS: "on",
		},
		{
			name:    "both already present are untouched",
			dsn:     "postgresql://u:p@h/mydb?client_encoding=UTF8&standard_conforming_strings=on",
			wantEnc: "UTF8",
			wantSCS: "on",
		},
		{
			name:    "user-provided client_encoding is respected",
			dsn:     "postgresql://user:pass@host:5432/mydb?client_encoding=GBK",
			wantEnc: "GBK",
			wantSCS: "on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePostgresDSN(tt.dsn)
			cfg, err := pgx.ParseConfig(got)
			require.NoError(t, err, "normalized DSN must be parseable by pgx")
			assert.Equal(t, tt.wantEnc, cfg.RuntimeParams["client_encoding"])
			assert.Equal(t, tt.wantSCS, cfg.RuntimeParams["standard_conforming_strings"])
		})
	}
}