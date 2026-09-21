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
# ${CFG_DEBUG} 调试开关
debug: false
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
