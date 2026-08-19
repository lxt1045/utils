package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnPostgre(ctx context.Context, conf config.DB) (gormdb *gorm.DB, err error) {
	sslmode := "disable"
	if conf.SSLMode {
		sslmode = "enable"
	}
	params := map[string]string{
		"host":     conf.Host,
		"user":     conf.User,
		"password": conf.Password,
		"dbname":   conf.DBName,
		"port":     conf.Port,
		"sslmode":  sslmode,
		"TimeZone": "Asia/Shanghai",
	}
	dsn := PostgreDNS(params)

	gormdb, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: log.NewGormLogger(log.Ctx(ctx)),
	})
	if err != nil {
		err = errors.WithErr(err)
		return
	}

	return
}

func PostgreDNS(params map[string]string) string {
	strs := make([]string, 0, len(params))
	for k, v := range params {
		strs = append(strs, k+"="+v)
	}

	// dsn := "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=Asia/Shanghai"
	dsn := strings.Join(strs, " ")
	return dsn
}

func ConnClickHouse(ctx context.Context, conf config.DB) (*gorm.DB, error) {
	if conf.DialTimeout <= 0 {
		conf.DialTimeout = 10
	}
	if conf.ReadTimeout <= 0 {
		conf.ReadTimeout = 20
	}
	dsn := fmt.Sprintf("clickhouse://%v:%v@%v:%v/?dial_timeout=%ds&read_timeout=%ds",
		conf.User, conf.Password, conf.Host, conf.Port, conf.DialTimeout, conf.ReadTimeout)
	gormdb, err := gorm.Open(clickhouse.Open(dsn), &gorm.Config{
		Logger: log.NewGormLogger(log.Ctx(ctx)),
	})
	if err != nil {
		err = errors.WithErr(err)
		return nil, err
	}
	return gormdb, nil
}
