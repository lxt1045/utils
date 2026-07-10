#!/bin/bash
export PATH=/usr/local/lib/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /mnt/d/project/go/src/github.com/lxt1045/utils/geohash/geos
go test -bench BenchmarkMergeOverlap -benchmem -run '^$' -v . 2>&1 | grep -E '重合|条|MergeCurves3|ns/op|ok  '
