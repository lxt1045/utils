package tools

import (
	"os"
	"path/filepath"
)

func UserTempDir() string {
	// 1. 优先使用环境变量
	for _, env := range []string{"TMPDIR", "TMP", "TEMP"} {
		if dir := os.Getenv(env); dir != "" {
			if isWritable(dir) {
				return dir
			}
		}
	}

	// 2. 尝试系统 /tmp（可能只是本次用户无权限）
	if isWritable("/tmp") {
		return "/tmp"
	}

	// 3. 回退到用户目录
	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".cache", "tmp"), // XDG 风格
			filepath.Join(home, ".tmp"),
			home,
		}
		for _, dir := range candidates {
			_ = os.MkdirAll(dir, 0700)
			if isWritable(dir) {
				return dir
			}
		}
	}

	// 4. 最后的兜底
	return os.TempDir()
}

// 通过尝试创建临时文件判断是否可写
func isWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	_ = os.Remove(name)
	return true
}
