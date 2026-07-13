export PATH=/usr/local/lib/go/bin:/usr/bin:/bin
export CGO_ENABLED=1
cd /mnt/d/project/go/src/github.com/lxt1045/utils/geohash/geos
go test -c -o /dev/null . 2>&1 | head -80
echo "---TESTCOMPILE DONE rc=$?---"
