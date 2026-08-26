package cache

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"testing"

	"github.com/VictoriaMetrics/fastcache"
)

type Person struct {
	Name  string
	Age   int
	Email string
}

func Test_fastcache(t *testing.T) {
	// 初始化：设置最大内存 100MB
	cache := fastcache.New(100 * 1024 * 1024)

	// 存：Key/Value 必须是 []byte
	cache.Set([]byte("user:1001"), []byte(`{"name":"张三","age":25}`))

	// 取：准备一个空 []byte 接收结果
	var dst []byte
	if val := cache.Get(dst, []byte("user:1001")); len(val) > 0 {
		fmt.Println("✅ 获取成功:", string(val))
	}

	// 查 + 删
	if cache.Has([]byte("user:1001")) {
		cache.Del([]byte("user:1001"))
		fmt.Println("🗑️ 缓存已删除")
	}
	cache.Del([]byte("user:000"))
}

func TestEncode(t *testing.T) {
	// 1. 创建待序列化的对象
	original := Person{
		Name:  "Alice",
		Age:   30,
		Email: "alice@example.com",
	}

	// 2. 序列化（编码）
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)
	err := encoder.Encode(original)
	if err != nil {
		log.Fatal("序列化失败:", err)
	}
	fmt.Printf("序列化后的二进制数据长度: %d 字节\n", buffer.Len())

	// 3. 反序列化（解码）
	var decoded Person
	decoder := gob.NewDecoder(&buffer)
	err = decoder.Decode(&decoded)
	if err != nil {
		log.Fatal("反序列化失败:", err)
	}

	// 4. 验证结果
	fmt.Printf("反序列化后的对象: %+v\n", decoded)
}
