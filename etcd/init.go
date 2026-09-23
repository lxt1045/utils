package etcd

import (
	"context"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	etcdcli "go.etcd.io/etcd/client/v3"
)

type Config struct {
	User        string
	Password    string
	DialTimeout int
	Nodes       []string
}

func New(ctx context.Context, etcdCOnf Config) (cliEtcd *etcdcli.Client, err error) {
	cliEtcd, err = etcdcli.New(etcdcli.Config{
		Endpoints:   etcdCOnf.Nodes,
		DialTimeout: 5 * time.Second,
		Username:    etcdCOnf.User,
		Password:    etcdCOnf.Password,
	})
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}

func Watch(ctx context.Context, cliEtcd *etcdcli.Client, cancel context.CancelFunc, f func(t BackupType, d EventData), prefixs ...string) (err error) {
	chEvents := make(chan EventDatas, 32)
	go TryN(ctx, "doWatch", cancel, 100, time.Second, func() (err error) {
		return doWatch(ctx, cliEtcd, chEvents, prefixs...)
	})

	go TryN(ctx, "doUpdate", cancel, 100, time.Second, func() (err error) {
		return doUpdate(ctx, chEvents, f)
	})

	return
}

func TryN(ctx context.Context, name string, cancel context.CancelFunc, N int, secSleep time.Duration, f func() error) {
	defer func() {
		if cancel != nil {
			cancel()
		}
		log.Ctx(ctx).Error().Caller().Msg("TryN return")
	}()

	last := time.Now().Unix()
	for i := 0; i < N; i++ {
		func() {
			defer func() {
				if e := recover(); e != nil {
					err := errors.Errorf("recove:%+v", e)
					log.Ctx(ctx).Error().Caller().Err(err).Send()
				}
				log.Ctx(ctx).Error().Caller().Int("times", i).Str("name", name).Msg("return")
			}()
			err := f()
			if err != nil {
				log.Ctx(ctx).Error().Caller().Err(err).Send()
			}
		}()

		// 一个小时内 连续退出100次则退出
		if time.Now().Unix()-last > 3600 {
			i = 0
		}
		time.Sleep(secSleep)
		last = time.Now().Unix()
	}
}
