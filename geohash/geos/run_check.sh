cd "$(dirname "$0")"
go test -run 'TestMergeCurves' -count=1 . 2>&1 | tail -20
