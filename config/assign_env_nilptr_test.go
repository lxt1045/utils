package config

import (
	"testing"
)

type envNested struct {
	Name string `mapstructure:"name" yaml:"name"`
}

type envHost struct {
	Sub  *envNested `mapstructure:"sub" yaml:"sub"`
	Port int        `mapstructure:"port" yaml:"port"`
}

// nil 指针字段：先创建实例去确认环境变量中是否有相关值，
// 没有实际赋值则丢弃该实例，字段保持 nil（且不能 panic）。
func TestAssignEnvNilPointerDiscarded(t *testing.T) {
	t.Setenv("CFG_TEST_SUB_NAME", "from-env")
	c := &envHost{}
	if err := Unmarshal([]byte("port: 8080\n"), c); err != nil {
		t.Fatal(err)
	}
	if c.Sub != nil {
		t.Fatalf("nil pointer field should stay nil, got %+v", c.Sub)
	}
	if c.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", c.Port)
	}
}

// yaml 提供 ${VAR} 占位符时指针已被 yaml 分配，环境变量替换正常生效。
func TestAssignEnvPointerPlaceholder(t *testing.T) {
	t.Setenv("CFG_TEST_SUB_NAME", "from-env")
	c := &envHost{}
	bs := []byte("sub:\n  name: ${CFG_TEST_SUB_NAME}\n")
	if err := Unmarshal(bs, c); err != nil {
		t.Fatal(err)
	}
	if c.Sub == nil {
		t.Fatal("pointer field should be allocated by yaml")
	}
	if c.Sub.Name != "from-env" {
		t.Fatalf("Sub.Name = %q, want %q", c.Sub.Name, "from-env")
	}
}

// 嵌套的 nil 指针字段同样走 分配-检查-丢弃 路径，保持 nil 且不 panic。
func TestAssignVarsFromEnvNestedNil(t *testing.T) {
	type outer struct {
		In *envNested `mapstructure:"in"`
	}
	c := &outer{}
	if err := AssignVarsFromEnv(c, map[string]string{"X": "1"}); err != nil {
		t.Fatal(err)
	}
	if c.In != nil {
		t.Fatalf("expected nil, got %+v", c.In)
	}
}

// 非 nil 指针字段内部的 ${VAR} 替换保持不变（回归）。
func TestAssignVarsFromEnvNonNilPointer(t *testing.T) {
	t.Setenv("CFG_TEST_INNER", "inner-value")
	c := &envHost{Sub: &envNested{Name: "${CFG_TEST_INNER}"}}
	if err := AssignVarsFromEnv(c, nil); err != nil {
		t.Fatal(err)
	}
	if c.Sub.Name != "inner-value" {
		t.Fatalf("Sub.Name = %q, want %q", c.Sub.Name, "inner-value")
	}
}
