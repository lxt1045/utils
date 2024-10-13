package engine

import (
	"encoding/json"
	"testing"

	"github.com/lxt1045/utils/tag"
)

func Test_NewTagInfos(t *testing.T) {
	t.Run("my", func(t *testing.T) {
		logInfo := Log{}
		tis, err := tag.NewTagInfos(logInfo, "json")
		if err != nil {
			t.Fatal(err)
		}
		bs, err := json.Marshal(tis)
		if err != nil {
			t.Fatal(err)
		}

		t.Log(string(bs))

		tis, err = tag.NewTagInfos(&logInfo, "json")
		if err != nil {
			t.Fatal(err)
		}
		bs, err = json.Marshal(tis)
		if err != nil {
			t.Fatal(err)
		}

		t.Log(string(bs))
	})
}
