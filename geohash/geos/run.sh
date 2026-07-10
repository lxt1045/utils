#!/bin/bash
export PATH=/usr/local/lib/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /mnt/d/project/go/src/github.com/lxt1045/utils/geohash/geos
go vet . 2>&1 | tail -20 && echo "VET_OK"
