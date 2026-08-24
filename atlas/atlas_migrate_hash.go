package atlas

import (
	"context"

	"ariga.io/atlas/sql/migrate"
	cmdmigrate "github.com/lxt1045/atlas-cmd-internal/migrate"
)

// atlas migrate hash --dir "file://e:/test/atlas/migrations?format=golang-migrate"   # 手动删文件后，需要重建 hash 文件
func MigrateHashRun(ctx context.Context, dirURL string) (err error) {
	dir, err := cmdmigrate.Dir(ctx, dirURL, false)
	if err != nil {
		return err
	}
	sum, err := dir.Checksum()
	if err != nil {
		return err
	}
	return migrate.WriteSumFile(dir, sum)
}
