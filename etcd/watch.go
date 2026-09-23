package etcd

import (
	"context"
	"reflect"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/log"
	cmap "github.com/orcaman/concurrent-map/v2"
	mvccpb3 "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	etcdcli "go.etcd.io/etcd/client/v3"
)

const (
	TTL           = 30 * time.Minute
	configPrefix  = "/config/"
	controlPrefix = "/control/"
)

var (
	CacheApps = cmap.New[*ConfigData]() // NewCache[*App]()
)

func All(ctx context.Context, cliEtcd *etcdcli.Client, chEvents chan EventDatas, prefixs ...string) (err error) {
	for _, prefix := range prefixs {
		resp, err := cliEtcd.Get(ctx, prefix, clientv3.WithPrefix())
		if nil != err {
			err = errors.WithErr(err)
			return err
		}
		events := make([]EventData, 0)
		for _, ev := range resp.Kvs {
			if len(ev.Value) == 0 {
				continue
			}
			events = append(events, EventData{
				EvType: TypePut,
				Key:    string(ev.Key),
				Value:  string(ev.Value),
			})
		}
		if len(events) == 0 {
			chEvents <- EventDatas{
				BackupType: TypeAll,
				EventDatas: []EventData{{
					EvType: TypePut,
					Key:    prefix,
					Value:  "",
				}},
			}
			continue
		}
		chEvents <- EventDatas{
			BackupType: TypeAll,
			EventDatas: events,
		}
	}

	return nil
}

// doWatch
func doWatch(ctx context.Context, cliEtcd *etcdcli.Client, chEvents chan EventDatas, prefixs ...string) (err error) {
	defer close(chEvents)

	err = All(ctx, cliEtcd, chEvents, prefixs...)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	// incCount := 0 // 增量备份超过100时，做一次全量
	cases := []reflect.SelectCase{
		{
			Dir:  reflect.SelectRecv,          // 设置为接收操作
			Chan: reflect.ValueOf(ctx.Done()), // 获取 channel 的反射值
		},
	}

	for _, prefix := range prefixs {
		ch := cliEtcd.Watch(context.Background(), prefix, clientv3.WithPrefix())
		log.Ctx(ctx).Info().Caller().Msgf("watching prefix:%s now...", prefix)
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,  // 设置为接收操作
			Chan: reflect.ValueOf(ch), // 获取 channel 的反射值
		})
	}

	for {
		chosen, recv, recvOK := reflect.Select(cases)
		if chosen == 0 {
			// case <-ctx.Done():
			return
		}
		if !recvOK {
			log.Ctx(ctx).Info().Caller().Err(err).Msgf("Channel %d has been closed\n", chosen)
			// 将已关闭的 channel 从监听列表中移除，防止再次被选中
			cases[chosen].Chan = reflect.ValueOf(nil)
			continue
		}
		// 打印接收到的消息和对应的 channel 索引
		log.Ctx(ctx).Info().Caller().Err(err).Any("data", recv.Interface()).Msgf("Received from channel %d:", chosen)

		wresp, ok := recv.Interface().(clientv3.WatchResponse)
		if !ok {
			log.Ctx(ctx).Info().Caller().Err(err).Msgf("Channel %d revc %T\n", chosen, recv.Interface())
			continue
		}

		events := make([]EventData, 0)
		for _, ev := range wresp.Events {
			e := EventData{
				Key:   string(ev.Kv.Key),
				Value: string(ev.Kv.Value),
			}
			switch ev.Type {
			case mvccpb3.PUT:
				e.EvType = TypePut
			case mvccpb3.DELETE:
				e.EvType = TypeDelete
			}
			events = append(events, e)
		}

		chEvents <- EventDatas{
			BackupType: TypeInc,
			EventDatas: events,
		}
	}

}

func doUpdate(ctx context.Context, chEvents chan EventDatas, f func(t BackupType, d EventData)) (err error) {
	for {
		var event EventDatas
		var ok bool
		select {
		case <-ctx.Done():
			return
		case event, ok = <-chEvents:
			if !ok {
				return
			}
		}

		if event.BackupType != TypeAll && event.BackupType != TypeInc {
			continue
		}
		for _, data := range event.EventDatas {
			f(event.BackupType, data)
		}
	}
}
