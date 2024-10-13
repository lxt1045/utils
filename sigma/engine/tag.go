package engine

import "github.com/lxt1045/utils/tag"

func tagToInfo(tag string, tis map[string]*tag.TagInfo) (ti *tag.TagInfo) {
	ti, ok := tis[tag]
	if !ok {
		tag, ok = tagMap[tag]
		if ok {
			ti, ok = tis[tag]
		}
	}
	return
}

var tagMap = map[string]string{
	"task": "extra1",
}
