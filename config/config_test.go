package config

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	t.Run("this-zerolog", func(t *testing.T) {
		bs := []byte(`storage-way: 3 
storage-path: comqtt.db 
bridge-way: 1 
bridge-path: ./cmd/config/bridge-kafka.yml 
auth:
  way: 1 
  datasource: 1 
  conf-path: ./config/auth-redis.yml 
  blacklist-path: ./config/blacklist.yml 
mqtt:
  tcp: :1883
  ws: :1882
  http: :8080
  tls-conf:
    ca-cert: ./config/blacklist.yml 
    server-cert: ./config/blacklist.yml 
    server-key: ./config/blacklist.yml 
`)
		c := &Config{}
		err := Unmarshal(bs, &c)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, c.StoragePath, "comqtt.db")
		t.Logf("%+v", *c)
	})
}

func TestCamel2Case(t *testing.T) {
	t.Run("Camel2Case", func(t *testing.T) {
		cases := map[string]string{
			"TCP":           "tcp",
			"CanDo":         "can_do",
			"CBACanDo":      "cba_can_do",
			"CBAC-anDo":     "cbac_an_do",
			"CBAC-AnDo":     "cbac_an_do",
			"xxxCBAC-anDo":  "xxx_cbac_an_do",
			"xxx-CBAC-anDo": "xxx_cbac_an_do",
			"TLSConf":       "tls_conf",
			"tls-conf":      "tls_conf",
			"Mqtt":          "mqtt",
		}
		for k, v := range cases {
			vv := Camel2Case(k)
			assert.Equal(t, v, vv)
		}
	})
}

type Config struct {
	StorageWay  uint
	StoragePath string
	BridgeWay   uint
	BridgePath  string
	Auth        struct {
		Way           uint
		Datasource    uint
		ConfPath      string
		BlacklistPath string
	}
	Mqtt struct {
		TCP     string
		WS      string
		HTTP    string
		TLSConf struct {
			CACert     string
			ServerCert string
			ServerKey  string
			ClientCert string
			ClientKey  string
		}
	}
}

func TestFloat(t *testing.T) {
	t.Run("Camel2Case", func(t *testing.T) {
		fInsert := func(i int) string {
			x := rand.Intn(100)
			if x < 5 {
				return strconv.Itoa(i)
			}
			return ""
		}
		c := 0
		n := 10000
		for i := 0; i < n; i++ {
			x := fInsert(i)
			if x != "" {
				t.Logf("x:%s", x)
				c++
			}
		}

		t.Logf("c:%f", float32(c)/float32(n))
	})
	t.Run("Camel2Case", func(t *testing.T) {
		var f float64 = 0.4
		cpuNum := 32
		fmt.Println("cpuNum:", cpuNum)
		cpuNum = int(math.Floor(float64(cpuNum)*f + 0.5))
		fmt.Println("cpuNum->", cpuNum)

	})
}
