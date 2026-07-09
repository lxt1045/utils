#!/bin/bash
export PATH=/usr/local/lib/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /mnt/d/project/go/src/github.com/lxt1045/utils/geohash/geos
go test -run 'TestMergeCurves_CompareWithGEOSLineMerge|TestMergeCurves_ToleranceRobustness' -count=1 -v 2>&1 | tail -60
