package atlas_test

import (
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lxt1045/utils/atlas"
	"github.com/lxt1045/utils/atlas/testdata"
	"github.com/lxt1045/utils/config"
)

func TestMigrateDiff(t *testing.T) {
	conf := config.DB{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "password",
		DBName:   "testdata",
		SSLMode:  true,
		AtlasDB: config.AtlasDB{
			DBName:     "atlas_dev",
			MigrateDir: "testdata/migrate",
		},
	}
	err := atlas.MigrateDiff(t.Context(), "test_version", "./testdata/test.sql", "./testdata/migrate", testdata.Migrate, conf)
	if err != nil {
		t.Fatal(err)
	}
}

/*
# 执行的时候也需要执行 golang-migrate 格式, 否则 up 和 down 文件都会被执行。
atlas migrate apply --dir "file://e:/test/atlas/migrations?format=golang-migrate" - --url ""mysql://root:password@127.0.0.1:3306/dji88"
*/
func TestMigrateApplyFS(t *testing.T) {
	conf := config.DB{
		Host:     "127.0.0.1",
		Port:     "3306",
		User:     "root",
		Password: "password",
		DBName:   "testdata",
		SSLMode:  true,
		AtlasDB: config.AtlasDB{
			DBName:     "atlas_dev",
			MigrateDir: "migrate",
		},
	}
	err := atlas.MigrateApplyFS(t.Context(), testdata.Migrate, conf)
	if err != nil {
		t.Fatal(err)
	}
}
