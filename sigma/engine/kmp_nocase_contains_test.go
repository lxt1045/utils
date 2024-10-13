package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/paddie/gokmp"
)

func Test_kmpIndex(t *testing.T) {
	str := "D:/project/go/src/gitlab.wecode.com/abcabcab/dev/broker/cmd/run_236/1034-find.yml"

	substrs := []string{
		"gitlab",
		"wecode.com",
		"abcabcab",
	}
	for _, substr := range substrs {
		t.Logf("std-i:%d", strings.Index(str, substr))
		t.Logf("kmp-i:%d", kmpIndexNoCaseMaker(substr)(str))
	}
}

func Test_kmpIndexNoCase(t *testing.T) {
	str := "D:/project/go/src/gitlab.wecode.com/abcabcab/dev/broker/cmd/run_236/1034-find.yml"

	substrs := []string{
		"gitlab",
		"wecode.com",
		"abcabcab",
		"AbcabCab",
	}
	for _, substr := range substrs {
		t.Logf("std-i:%d", strings.Index(str, substr))
		t.Logf("kmpIndex-i:%d", kmpIndexNoCaseMaker(substr)(str))
	}
}

func Test_Time(t *testing.T) {
	var id int64 = 1823553616469225474
	tt := id / 1073741824
	ttt := time.Unix(tt, 0)
	t.Log(ttt.Format(time.RFC3339Nano))
}

func Test_kmpIndexNoCase2(t *testing.T) {
	str := "gitlabBBa"

	substrs := []string{
		"iTLa",
		"aBb",
		"abB",
	}
	for _, substr := range substrs {

		t.Logf("kmpIndexNoCaseMaker-i:%v\n\n", kmpIndexNoCaseMaker(substr)(str))
		t.Logf("std-i:%d", strings.Index(str, substr))
		substr = strings.ToLower(substr)
		t.Logf("std-i:%d\n\n", strings.Index(str, substr))
	}
}

/*
go test -benchmem -run=^$ -bench ^Benchmark_Index2$ github.com/lxt1045/Experiment/golang/sigma/sigma -count=1 -v -cpuprofile cpu.prof -c
go test -benchmem -run=^$ -bench ^Benchmark_Index2$ github.com/lxt1045/Experiment/golang/sigma/sigma -count=1 -v -memprofile cpu.prof -c
go tool pprof sigma.test.exe cpu.prof
*/

func Benchmark_Index(b *testing.B) {
	str := "D:/project/go/src/gitlab.wecode.com/abcabcab/dev/broker/cmd/run_236/1034-find.yml"

	substrs := []string{
		"gitlab",
		"wecode.com",
		"abcabcab",
		"AbcabCab",
	}

	for _, substr := range substrs {
		b.Run("std-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				strings.Index(str, substr)
			}
		})

		kmp1, _ := gokmp.NewKMP(substr)
		b.Run("FindStringIndex-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				kmp1.FindStringIndex(str)
			}
		})

		fkmp := kmpIndexNoCaseMaker(substr)
		b.Run("kmpIndexNoCaseMaker-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				fkmp(str)
			}
		})

		fkmpc1 := kmpContainsMaker(substr)
		b.Run("kmpContainsMaker-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				fkmpc1(str)
			}
		})

		fkmpc := kmpContainsNoCaseMaker(substr)
		b.Run("kmpContainsNoCaseMaker-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				fkmpc(str)
			}
		})

		fkmpc2 := kmpContainsNoCaseMaker2(substr)
		b.Run("kmpContainsNoCaseMaker2-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				fkmpc2(str)
			}
		})
		fkmpc3 := kmpContainsNoCaseMaker3(substr)
		b.Run("kmpContainsNoCaseMaker3-"+substr, func(b *testing.B) {
			for j := 0; j < b.N; j++ {
				fkmpc3(str)
			}
		})
		b.Log("\n\n")
	}
}
