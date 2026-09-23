package etcd

type Type byte

const (
	TypePut    = Type(1) // 新增or修改
	TypeDelete = Type(2) // 删除
)

type BackupType byte

const (
	TypeAll = BackupType(1) // 全量
	TypeInc = BackupType(2) // 增量
)

type EventData struct {
	EvType Type   `json:"ev_type"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

type EventDatas struct {
	BackupType BackupType
	EventDatas []EventData
}

type ConfigData struct {
	// test
}
