package db

import (
	"context"

	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
)

func InitTest(ctx context.Context) (err error) {
	// 初始化
	err = log.Init(ctx, config.Log{LogLevel: "trace"})
	if err != nil {
		log.Ctx(ctx).Fatal().Caller().Err(err).Send()
		return
	}

	db := config.DB{
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "test",
		Password: "pw",
		DBName:   "db",
		SSLMode:  false,
	}

	err = CreateMysqlDB(ctx, db)
	if err != nil {
		log.Ctx(ctx).Fatal().Err(err).Send()
	}
	return
}
