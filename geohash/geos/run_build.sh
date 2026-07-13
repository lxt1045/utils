export PATH=/usr/local/lib/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /mnt/d/project/go/src/github.com/lxt1045/utils/geohash/geos
go mod tidy 2>&1 | head -30
echo "---MOD TIDY DONE---"
go test -c -o /tmp/geos_test . 2>&1 | head -80
echo "---TEST COMPILE rc=$?---"
