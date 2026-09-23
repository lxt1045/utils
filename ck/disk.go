package ck

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
)

type DiskInfo struct {
	Name  string `db:"name"`
	Path  string `db:"path"`
	Free  int64  `db:"free_space"`
	Total int64  `db:"total_space"`
}

type UsageStat struct {
	DiskInfo
	Used        int64
	UsedPercent float64
}

func GetCkDiskInfo(ctx context.Context, db *sqlx.DB) (disk UsageStat, err error) {
	sql1 := "SELECT name, path, free_space, total_space FROM system.disks;"
	log.Ctx(ctx).Debug().Caller().Str("sql", sql1).Msg("xxxxxxxxxxx")

	disks := []DiskInfo{}
	err = db.Select(&disks, sql1) // Gorm 对 array 类型支持有点问题, sqlx 没问题的
	if err != nil {
		err = errors.Errorf("获取磁盘信息失败:%v", err)
		return
	}

	if len(disks) == 0 {
		err = errors.New("获取磁盘信息失败: got empty")
		return
	}
	disk = UsageStat{
		DiskInfo: disks[0],
		Used:     disks[0].Total - disks[0].Free,
	}
	disk.UsedPercent = 100 * float64(disk.Used) / float64(disk.Total)
	return
}
