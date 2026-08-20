package db

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/log"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnPostgreGorm(ctx context.Context, conf config.DB) (gormdb *gorm.DB, err error) {
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

func ConnCkSqlx(ctx context.Context, conf config.DB) (sqlxDb *sqlx.DB, err error) {
	tcpInfo := fmt.Sprintf("clickhouse://%s:%s@%s:%s/%s?read_timeout=%ds&output_format_native_use_flattened_dynamic_and_json_serialization=1",
		conf.User, url.QueryEscape(conf.Password), conf.Host, conf.Port, conf.DBName, conf.ReadTimeout)

	sqlxDb, err = sqlx.Open("clickhouse", tcpInfo)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return sqlxDb, err
}

func ConnCkGorm(ctx context.Context, conf config.DB) (*gorm.DB, error) {
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

func ConnMysqlSqlx(ctx context.Context, conf config.DB) (sqlxDb *sqlx.DB, err error) {
	// tcpInfo := fmt.Sprintf("mysql://%s:%s@%s:%s/%s?username=%s&password=%s&read_timeout=5s&compress=true",
	// 	conf.User, url.QueryEscape(conf.Password), conf.Host, conf.Port, conf.DBName, conf.User, url.QueryEscape(conf.Password))

	// 构建 DSN (Data Source Name)
	// 使用 MySQL 驱动的标准 DSN 格式
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		conf.User,
		url.QueryEscape(conf.Password),
		conf.Host,
		conf.Port,
		conf.DBName,
	)

	sqlxDb, err = sqlx.Open("mysql", dsn)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	// 测试连接
	if err = sqlxDb.PingContext(ctx); err != nil {
		sqlxDb.Close() // 关闭连接
		err = errors.WithErr(err)
		return
	}
	return sqlxDb, err
}

func ConnMysqlGorm(ctx context.Context, conf config.DB) (db *gorm.DB, err error) {
	// gormdb, _ := gorm.Open(mysql.Open("root:@(127.0.0.1:3306)/demo?charset=utf8mb4&parseTime=True&loc=Local"))
	uri := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		conf.User,
		conf.Password,
		conf.Host,
		conf.Port,
		conf.DBName)
	log.Ctx(ctx).Debug().Msg(uri)
	db, err = gorm.Open(mysql.Open(uri), &gorm.Config{Logger: log.NewGormLogger(log.Ctx(ctx))})
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	sqlDB, err := db.DB()
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetMaxOpenConns(100)
	return
}

type GormOption struct {
	log *log.GormLogger
}

func NewGormOption(ctx context.Context) *GormOption {
	return &GormOption{
		log: log.NewGormLogger(log.Ctx(ctx)),
	}
}

func (o *GormOption) Apply(conf *gorm.Config) error {
	conf.Logger = o.log
	return nil
}

func (o *GormOption) AfterInitialize(*gorm.DB) error {
	return nil
}

// CreateMysqlDB 检查DB是否创建
func CreateMysqlDB(ctx context.Context, dbconf config.DB) (err error) {
	defer func() {
		if err != nil {
			if _, ok := err.(*errors.Code); !ok {
				err = errors.WithErr(err)
			}
		}
	}()
	dbconf1 := dbconf
	dbconf1.DBName = ""
	db, err := ConnMysqlGorm(ctx, dbconf1)
	if err != nil {
		return
	}

	dbs := []string{}
	err = db.Raw("SHOW DATABASES;").Scan(&dbs).Error
	if err != nil {
		return
	}

	log.Ctx(ctx).Info().Caller().Msgf("CreateMysqlDB: %s", dbconf.DBName)
	if !slices.Contains(dbs, dbconf.DBName) {
		err = db.Exec("CREATE DATABASE " + dbconf.DBName).Error
		if err != nil {
			return
		}
	}
	return
}

func CreateCkDB(ctx context.Context, dbconf config.DB) (err error) {
	defer func() {
		if err != nil {
			if _, ok := err.(*errors.Code); !ok {
				err = errors.WithErr(err)
			}
		}
	}()
	dbconf1 := dbconf
	dbconf1.DBName = ""
	db, err := ConnCkGorm(ctx, dbconf1)
	if err != nil {
		return
	}

	dbs := []string{}
	err = db.Raw("SHOW DATABASES;").Scan(&dbs).Error
	if err != nil {
		return
	}

	log.Ctx(ctx).Info().Caller().Msgf("CreateCkDB: %s", dbconf.DBName)
	if !slices.Contains(dbs, dbconf.DBName) {
		err = db.Exec("CREATE DATABASE " + dbconf.DBName).Error
		if err != nil {
			return
		}
	}

	return
}
