package tag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var _ = assert.NotNil

func TestSetLastGID(t *testing.T) {
	t.Run("SetLastGID", func(t *testing.T) {
		tt := &TagInfo{}
		tis, err := NewTagInfos(tt, "")
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%+v", tis)
	})
}
