package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var _ = assert.NotNil

func TestGetGID(t *testing.T) {
	t.Run("GetGID", func(t *testing.T) {
	})
}

func BenchmarkGetGID(b *testing.B) {
	b.Run("time.Now().Unix()", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = time.Now().Unix()
		}
	})
}
