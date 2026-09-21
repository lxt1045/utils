package config

import (
	"strings"
	"testing"
)

type commentCfg struct {
	Port     int      `mapstructure:"port"`
	Debug    bool     `mapstructure:"debug"`
	Rate     float64  `mapstructure:"rate"`
	Name     string   `mapstructure:"name"`
	MaxConns int      `mapstructure:"max_conns"`
	Hosts    []string `mapstructure:"hosts"`
	Limits   []int    `mapstructure:"limits"`
	NoDelay  *int     `mapstructure:"kcp_nodelay"`
	Sub      *subCfg  `mapstructure:"sub"`
}

type subCfg struct {
	Level int `mapstructure:"level"`
}

// 注释形式的环境变量赋值：覆盖所有类型
func TestCommentEnvAllTypes(t *testing.T) {
	t.Setenv("CFG_PORT", "9090")
	t.Setenv("CFG_DEBUG", "true")
	t.Setenv("CFG_RATE", "1.5")
	t.Setenv("CFG_NAME", "hello world")
	t.Setenv("CFG_LIMITS", "[1, 2, 3]")
	t.Setenv("CFG_NODELAY", "0")

	bs := []byte(`
port: 8080 # ${CFG_PORT} 监听端口
debug: false # ${CFG_DEBUG} 调试开关
rate: 0.5 # ${CFG_RATE}
name: "abc" # ${CFG_NAME}
limits: [9] # ${CFG_LIMITS}
kcp_nodelay: # ${CFG_NODELAY}
`)
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", c.Port)
	}
	if !c.Debug {
		t.Fatalf("Debug = %v, want true", c.Debug)
	}
	if c.Rate != 1.5 {
		t.Fatalf("Rate = %v, want 1.5", c.Rate)
	}
	if c.Name != "hello world" {
		t.Fatalf("Name = %q, want %q", c.Name, "hello world")
	}
	if len(c.Limits) != 3 || c.Limits[0] != 1 || c.Limits[2] != 3 {
		t.Fatalf("Limits = %v, want [1 2 3]", c.Limits)
	}
	if c.NoDelay == nil || *c.NoDelay != 0 {
		t.Fatalf("NoDelay = %v, want ptr(0)", c.NoDelay)
	}
}

// 环境变量不存在时保持 yaml 中的值
func TestCommentEnvMissingKeepsYAML(t *testing.T) {
	bs := []byte(`
port: 8080 # ${CFG_NO_SUCH_ENV_XYZ}
debug: true
  # ${CFG_NO_SUCH_ENV_XYZ}
`)
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 {
		t.Fatalf("Port = %d, want 8080 (yaml value kept)", c.Port)
	}
	if !c.Debug {
		t.Fatalf("Debug = %v, want true (yaml value kept)", c.Debug)
	}
}

// key 的驼峰/下划线/中划线/大小写开头映射
func TestKeyMappingForms(t *testing.T) {
	for _, doc := range []string{
		"max-conns: 8\n",
		"max_conns: 8\n",
		"maxConns: 8\n",
		"MaxConns: 8\n",
	} {
		c := &commentCfg{}
		if err := Unmarshal([]byte(doc), c); err != nil {
			t.Fatalf("%q: %v", doc, err)
		}
		if c.MaxConns != 8 {
			t.Fatalf("%q: MaxConns = %d, want 8", doc, c.MaxConns)
		}
	}
}

// 注释形式优先于值形式
func TestCommentEnvPriorThanValue(t *testing.T) {
	t.Setenv("CFG_COMMENT", "111")
	t.Setenv("CFG_VALUE", "222")
	bs := []byte("port: ${CFG_VALUE} # ${CFG_COMMENT}\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 111 {
		t.Fatalf("Port = %d, want 111 (comment form wins)", c.Port)
	}
}

// 嵌套结构体字段的注释形式
func TestCommentEnvNested(t *testing.T) {
	t.Setenv("CFG_LEVEL", "7")
	bs := []byte(`
sub:
  level: 1 # ${CFG_LEVEL}
`)
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Sub == nil || c.Sub.Level != 7 {
		t.Fatalf("Sub = %+v, want Level=7", c.Sub)
	}
}

// key 不存在时字段保持零值/nil（无节点即无注释）
func TestCommentEnvAbsentKey(t *testing.T) {
	t.Setenv("CFG_NODELAY", "1")
	c := &commentCfg{}
	if err := Unmarshal([]byte("port: 1\n"), c); err != nil {
		t.Fatal(err)
	}
	if c.NoDelay != nil {
		t.Fatalf("NoDelay should stay nil without yaml key, got %v", *c.NoDelay)
	}
}

// 错误配置：字段类型与 yaml 节点类型不匹配，错误信息需带路径和行号
func TestConfigTypeMismatchErrors(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want []string // 错误信息必须包含的片段
	}{
		{"struct_gets_scalar", "sub: 123\n", []string{"config sub (line 1)", "expect mapping"}},
		{"slice_gets_scalar", "limits: 5\n", []string{"config limits (line 1)", "expect sequence"}},
		{"int_gets_string", "port: abc\n", []string{"config port (line 1)", "cannot decode"}},
		{"nested_path", "sub:\n  level: abc\n", []string{"config sub.level (line 2)", "cannot decode"}},
		{"map_gets_scalar", "hosts: 3\n", []string{"config hosts (line 1)", "expect sequence"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Unmarshal([]byte(c.doc), &commentCfg{})
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q missing %q", err.Error(), w)
				}
			}
			t.Logf("err: %v", err)
		})
	}
}

// env 值无法解析成字段类型时，错误信息带 env 名和路径
func TestCommentEnvDecodeError(t *testing.T) {
	t.Setenv("CFG_PORT_BAD", "not-a-number")
	bs := []byte("port: 8080 # ${CFG_PORT_BAD}\n")
	err := Unmarshal(bs, &commentCfg{})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "env ${CFG_PORT_BAD}") || !strings.Contains(err.Error(), "config port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// 未知 key 容忍（同一配置文件可能被不同的 Config 结构体读取）；null 值容忍
func TestUnknownKeyAndNullTolerated(t *testing.T) {
	bs := []byte("unknown-key: 1\nsub:\nport: 1\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Sub != nil {
		t.Fatalf("null sub should stay nil, got %+v", c.Sub)
	}
}

// POSIX 参数扩展：值形式
func TestPOSIXValueForm(t *testing.T) {
	cases := []struct {
		name    string
		envSet  bool
		envVal  string
		expr    string // yaml 中 name 字段的值
		want    string
		wantErr string // 非空则表示期望错误包含该片段
	}{
		{"default_unset", false, "", "${PX_V:-ddd}", "ddd", ""},
		{"default_set", true, "real", "${PX_V:-ddd}", "real", ""},
		{"default_empty", true, "", "${PX_V:-ddd}", "ddd", ""},      // :- 空值也用默认
		{"dash_unset", false, "", "${PX_V-ddd}", "ddd", ""},
		{"dash_set", true, "real", "${PX_V-ddd}", "real", ""},
		{"dash_empty", true, "", "${PX_V-ddd}", "", ""},             // - 空值保留
		{"qmark_unset", false, "", "${PX_V:?boom msg}", "", "boom msg"},
		{"qmark_empty", true, "", "${PX_V:?boom msg}", "", "boom msg"}, // :? 空值也报错
		{"qmark_set", true, "real", "${PX_V:?boom msg}", "real", ""},
		{"qmark_noarg", false, "", "${PX_V:?}", "", "PX_V: parameter not set or null"},
		{"q_unset", false, "", "${PX_V?boom msg}", "", "boom msg"},
		{"q_empty", true, "", "${PX_V?boom msg}", "", ""},           // ? 空值保留
		{"q_set", true, "real", "${PX_V?boom msg}", "real", ""},
		{"plain_unset", false, "", "${PX_V}", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.envSet {
				t.Setenv("PX_V", c.envVal)
			}
			cfg := &commentCfg{}
			doc := "name: \"" + c.expr + "\"\n"
			err := Unmarshal([]byte(doc), cfg)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Name != c.want {
				t.Fatalf("expr %q envSet=%v envVal=%q: Name = %q, want %q",
					c.expr, c.envSet, c.envVal, cfg.Name, c.want)
			}
		})
	}
}

// POSIX 参数扩展：注释形式（与值形式语义一致，且支持类型转换）
func TestPOSIXCommentForm(t *testing.T) {
	// :- 未设置 → 默认值（按字段类型解析）
	bs := []byte("port: 8080 # ${PX_PORT:-9090}\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 {
		t.Fatalf("Port = %d, want 9090", c.Port)
	}

	// :- 已设置 → 环境变量的值
	t.Setenv("PX_PORT", "7070")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 7070 {
		t.Fatalf("Port = %d, want 7070", c.Port)
	}

	// :? 未设置 → 报错，带路径和自定义消息
	bs = []byte("port: 8080 # ${PX_UNSET_XYZ:?must set port}\n")
	err := Unmarshal(bs, &commentCfg{})
	if err == nil || !strings.Contains(err.Error(), "must set port") || !strings.Contains(err.Error(), "config port") {
		t.Fatalf("want path+msg error, got %v", err)
	}

	// - 未设置 → 默认值；slice 类型
	bs = []byte("limits: [9] # ${PX_LIMITS-[1, 2]}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if len(c.Limits) != 2 || c.Limits[0] != 1 || c.Limits[1] != 2 {
		t.Fatalf("Limits = %v, want [1 2]", c.Limits)
	}
}

// 旧语法 "${VAR}|default" 仍然有效（值形式）
func TestLegacyPipeDefaultStillWorks(t *testing.T) {
	c := &commentCfg{}
	if err := Unmarshal([]byte("name: \"${PX_LEGACY_UNSET}|fallback\"\n"), c); err != nil {
		t.Fatal(err)
	}
	if c.Name != "fallback" {
		t.Fatalf("Name = %q, want %q", c.Name, "fallback")
	}
}

// POSIX 参数扩展：${VAR:+alt} 和 ${VAR:=default}（值形式）
func TestPOSIXPlusAssignValueForm(t *testing.T) {
	cases := []struct {
		name    string
		envSet  bool
		envVal  string
		expr    string
		want    string
		wantErr string
	}{
		{"plus_set", true, "real", "${PX_V:+alt}", "alt", ""},
		{"plus_unset", false, "", "${PX_V:+alt}", "", ""},
		{"plus_empty", true, "", "${PX_V:+alt}", "", ""},        // :+ 空值不算已设置
		{"plus_nocolon_set", true, "real", "${PX_V+alt}", "alt", ""},
		{"plus_nocolon_empty", true, "", "${PX_V+alt}", "alt", ""}, // + 空值也算已设置
		{"plus_nocolon_unset", false, "", "${PX_V+alt}", "", ""},
		{"assign_unset", false, "", "${PX_V:=ddd}", "ddd", ""},
		{"assign_set", true, "real", "${PX_V:=ddd}", "real", ""},
		{"assign_empty", true, "", "${PX_V:=ddd}", "ddd", ""},   // := 空值也赋默认
		{"assign_nocolon_unset", false, "", "${PX_V=ddd}", "ddd", ""},
		{"assign_nocolon_empty", true, "", "${PX_V=ddd}", "", ""},  // = 空值保留
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.envSet {
				t.Setenv("PX_V", c.envVal)
			}
			cfg := &commentCfg{}
			doc := "name: \"" + c.expr + "\"\n"
			err := Unmarshal([]byte(doc), cfg)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error containing %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Name != c.want {
				t.Fatalf("expr %q envSet=%v envVal=%q: Name = %q, want %q",
					c.expr, c.envSet, c.envVal, cfg.Name, c.want)
			}
		})
	}
}

// POSIX := 的写回语义：同一 Unmarshal 内后续引用可见（值形式）
func TestPOSIXAssignWriteBack(t *testing.T) {
	// 第一个字段用 := 赋默认值，第二个字段直接引用同一变量
	type twoNames struct {
		A string `mapstructure:"a"`
		B string `mapstructure:"b"`
	}
	c := &twoNames{}
	doc := "a: \"${PX_WRITEBACK:=shared}\"\nb: \"${PX_WRITEBACK}\"\n"
	if err := Unmarshal([]byte(doc), c); err != nil {
		t.Fatal(err)
	}
	if c.A != "shared" || c.B != "shared" {
		t.Fatalf("A=%q B=%q, want both %q", c.A, c.B, "shared")
	}
}

// POSIX :+ / := 注释形式（类型转换 + 未命中保持 yaml 值）
func TestPOSIXPlusAssignCommentForm(t *testing.T) {
	// :+ 已设置 → alt（按字段类型解析）
	t.Setenv("PX_SET", "1")
	bs := []byte("port: 8080 # ${PX_SET:+9090}\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 {
		t.Fatalf("Port = %d, want 9090 (:+ hit)", c.Port)
	}

	// :+ 未设置 → 求值为空 → 保持 yaml 值
	bs = []byte("port: 8080 # ${PX_UNSET_XYZ:+9090}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 8080 {
		t.Fatalf("Port = %d, want 8080 (:+ miss keeps yaml)", c.Port)
	}

	// := 未设置 → 默认值并写回；同文件后续字段引用同一变量
	bs = []byte("port: 8080 # ${PX_ASSIGNED:-1}\ndebug: false # ${PX_ASSIGNED:=true}\n")
	_ = bs
	bs = []byte("port: 1 # ${PX_ASSIGNED2:=7070}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 7070 {
		t.Fatalf("Port = %d, want 7070 (:= default)", c.Port)
	}
}

// 新规则：只处理 key/value 同一行行尾注释中的第一个 ${...}，其他注释忽略
func TestCommentEnvLineOnly(t *testing.T) {
	// 1. 字段上方的整块注释（HeadComment）不再生效，即使环境变量已设置
	t.Setenv("PX_HEAD", "true")
	bs := []byte("# ${PX_HEAD}\ndebug: false\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Debug {
		t.Fatalf("head comment should be ignored, Debug = %v", c.Debug)
	}

	// 2. 行尾注释中多个 ${...} 只取第一个
	t.Setenv("PX_FIRST", "111")
	t.Setenv("PX_SECOND", "222")
	bs = []byte("port: 1 # ${PX_FIRST} ${PX_SECOND}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 111 {
		t.Fatalf("Port = %d, want 111 (first ${...} wins)", c.Port)
	}

	// 3. 第一个 ${...} 不在注释开头也生效
	bs = []byte("port: 1 # 监听端口 ${PX_FIRST} 说明\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 111 {
		t.Fatalf("Port = %d, want 111 (first ${...} anywhere in line comment)", c.Port)
	}
}

// 注释回退链：多个 ${...} 时第一个存在的环境变量为值，全不存在取第一个默认值
func TestCommentEnvChain(t *testing.T) {
	// 1. 第一个不存在、第二个存在 → 用第二个的值
	t.Setenv("PX_CHAIN_B", "222")
	bs := []byte("port: 1 # ${PX_CHAIN_UNSET_A} ${PX_CHAIN_B}\n")
	c := &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 222 {
		t.Fatalf("Port = %d, want 222 (first existing var)", c.Port)
	}

	// 2. 都不存在 → 第一个默认值
	bs = []byte("port: 1 # ${PX_CHAIN_UNSET_A} ${PX_CHAIN_UNSET_B:-9090}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 9090 {
		t.Fatalf("Port = %d, want 9090 (first default)", c.Port)
	}

	// 3. 存在的环境变量优先于靠前的默认值：A 有默认值但 B 存在 → 用 B
	bs = []byte("port: 1 # ${PX_CHAIN_UNSET_A:-111} ${PX_CHAIN_B}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 222 {
		t.Fatalf("Port = %d, want 222 (existing var beats earlier default)", c.Port)
	}

	// 4. 都不存在且无默认值 → 保持 yaml 值
	bs = []byte("port: 1 # ${PX_CHAIN_UNSET_A} ${PX_CHAIN_UNSET_B}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 1 {
		t.Fatalf("Port = %d, want 1 (keep yaml)", c.Port)
	}

	// 5. 链上遇到 :? 且变量不满足 → 立即报错
	bs = []byte("port: 1 # ${PX_CHAIN_UNSET_A:?need A} ${PX_CHAIN_UNSET_B:-5}\n")
	err := Unmarshal(bs, &commentCfg{})
	if err == nil || !strings.Contains(err.Error(), "need A") {
		t.Fatalf("want chain :? error, got %v", err)
	}

	// 6. 两个默认值取第一个
	bs = []byte("port: 1 # ${PX_CHAIN_UNSET_A:-111} ${PX_CHAIN_UNSET_B:-222}\n")
	c = &commentCfg{}
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Port != 111 {
		t.Fatalf("Port = %d, want 111 (first default wins)", c.Port)
	}
}
