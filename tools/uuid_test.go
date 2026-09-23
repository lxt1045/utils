package tools

import (
	"fmt"
	"testing"
	"uuid"
)

func TestUUID(t *testing.T) {
	uuidStr := "01939c00-282d-7f2f-9cc2-887dc7b40629"
	t1, err := extractTimeFromUUIDv7Str(uuidStr)
	if err != nil {
		fmt.Printf("解析失败: %v\n", err)
		return
	}
	fmt.Printf("提取的时间戳: %v\n", t1)
	t1, err = UUIDv7StrToTime(uuidStr)
	if err != nil {
		t.Fatal()
	}
	fmt.Printf("提取的时间戳: %v\n", t1)

	t1, err = extractTimeFromUUIDv7(uuid.NewV7())
	if err != nil {
		fmt.Printf("解析失败: %v\n", err)
		return
	}
	fmt.Printf("提取的时间戳: %v\n", t1)
	fmt.Printf("提取的时间戳: %v\n", UUIDv7ToTime(uuid.NewV7()))
}
