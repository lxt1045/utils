package config

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
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

func TestLog1(t *testing.T) {
	t.Run("this-zerolog", func(t *testing.T) {
		bs := []byte(`
template:
  name: system_alert
  tpl_id: 1478887 # 腾讯云模板id
  title: 系统故障告警
  content: "{1}，系统故障，请及时处理!"
templates:
  - name: system_alert
    tpl_id: 1478887 # 腾讯云模板id
    title: 系统故障告警
    content: "{1}，系统故障，请及时处理!"
  - tpl_id: 408221
    name: app_alert
    title: 应用告警
    content: "{1}请查看邮件处理 "
  - tpl_id: 406247
    name: packet_capture_delay_alert
    title: app抓包延迟
    content: "app {1} 抓包延迟"
  - tpl_id: 406243
    name: version_update_alert
    title: app版本更新
    content: "app {1} 版本更新"
`)
		type Template struct {
			TplID   string
			Name    string
			Title   string
			Content string
		}
		type Alert struct {
			Template  Template
			Templates []Template
		}
		conf := Alert{}
		err := Unmarshal(bs, &conf)
		if err != nil {
			t.Fatal(err)
		}
		bs, err = json.Marshal(conf)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("conf: %s", string(bs))
	})
}
func TestCamel2Case(t *testing.T) {
	t.Run("Camel2Case", func(t *testing.T) {
		cases := map[string]string{
			"ProxyAddr":     "proxy_addr",
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

func TestParseBytes(t *testing.T) {
	testCases := map[string]int64{
		"100GB":  100 * 1024 * 1024 * 1024,
		"100gb":  100 * 1024 * 1024 * 1024,
		"100 GB": 100 * 1024 * 1024 * 1024,
		"100G":   100 * 1024 * 1024 * 1024,
		"100g":   100 * 1024 * 1024 * 1024,
		"100 G":  100 * 1024 * 1024 * 1024,
		"128MB":  128 * 1024 * 1024,
		"256M":   256 * 1024 * 1024,
		"64mb":   64 * 1024 * 1024,
		"32m":    32 * 1024 * 1024,
		"128KB":  128 * 1024,
		"256K":   256 * 1024,
		"64kb":   64 * 1024,
		"32k":    32 * 1024,
		"128":    128,
		"256":    256,
		"64":     64,
		"32":     32,
	}
	t.Run("testCases", func(t *testing.T) {
		var defaultValue int64 = 111
		for k, v := range testCases {
			n, err := ParseBytes(k, defaultValue)
			if err != nil {
				t.Fatal(err)
			}
			if v != n {
				t.Fatalf("%s, v:%d != n:%d", k, v, n)
			}
		}
		k := "1a01G"
		n, err := ParseBytes(k, defaultValue)
		if err == nil {
			t.Fatal("err")
		}
		if defaultValue != n {
			t.Fatalf("%s, defaultValue:%d != n:%d", k, defaultValue, n)
		}
	})
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

func TestAssignVarFromEnv(t *testing.T) {
	err := os.Setenv("Var_Test", "test")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("AssignVarFromEnv", func(t *testing.T) {
		out, ok := AssignVarFromEnv("${Var_Test}")
		if !ok {
			t.Fatal("not exist")
		}
		t.Logf("out: %s", out)
	})

	t.Run("AssignMapFromEnv", func(t *testing.T) {
		m := map[string]string{
			"key1": "${Var_Test}",
		}
		out, err := AssignMapFromEnv(m)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %s", out)
	})

	t.Run("AssignMapFromEnv-interface", func(t *testing.T) {
		m := map[string]interface{}{
			"key1": "${Var_Test}",
		}
		out, err := AssignMapFromEnv(m)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %s", out)
	})

	t.Run("AssignVarsFromEnv-slice", func(t *testing.T) {
		m := []string{"test222", "${Var_Test}"}
		err := AssignVarsFromEnv(&m)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %+v", m)
	})

	t.Run("AssignVarsFromEnv-array", func(t *testing.T) {
		m := [2]string{"test222", "${Var_Test}"}
		err := AssignVarsFromEnv(&m)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %+v", m)
	})

	t.Run("AssignVarsFromEnv", func(t *testing.T) {
		obj := struct {
			Str string
			M   map[string]interface{}
		}{
			Str: "${Var_Test}",
			M: map[string]interface{}{
				"key1": "${Var_Test}",
			},
		}
		err := AssignVarsFromEnv(&obj)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %+v", obj)
	})

	t.Run("AssignVarsFromEnv-OR", func(t *testing.T) {
		obj := struct {
			Str   string
			Str2  string
			Empty string
			M     map[string]interface{}
		}{
			Str:   "${Var_Test}|888888 ",
			Str2:  "${Var_yyy}|888888 ",
			Empty: "${Var_xxx}   ",
			M: map[string]interface{}{
				"key1":  "${Var_Test} | 6666666 ",
				"key2":  "${Var_xxx} | 6666666 ",
				"Empty": "${Var_xxx}  ",
			},
		}
		err := AssignVarsFromEnv(&obj)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("out: %+v", obj)
	})
}
