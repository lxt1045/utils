package tools

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var _ = assert.NotNil

func TestUserTempDir(t *testing.T) {
	t.Run("UserTempDir", func(t *testing.T) {
		t.Logf("UserTempDir:%s", UserTempDir())
	})
}
