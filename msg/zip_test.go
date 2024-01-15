package msg

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/gogo/protobuf/proto"
	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/lxt1045/utils/gid"
	"github.com/lxt1045/utils/msg/pb"
	"github.com/pierrec/lz4/v4"
)

func TestHeader(t *testing.T) {
	t.Run("Header", func(t *testing.T) {
		t.Logf("len(Header):%d", unsafe.Sizeof(Header{}))
	})

}

func newMsgPB(ctx context.Context, req *pb.Log) (bs []byte, err error) {
	return MarshalMsg(ctx, req, req.LogId, nil)
}

func fromMsgPB(ctx context.Context, r io.Reader) (m Msg, n int) {
	h, pack, err := ReadPack(ctx, r, nil)
	if err != nil {
		panic(err)
	}
	n = len(pack)
	if len(pack) > HeaderSize {
		m, err = ParseMsg(ctx, h, pack[HeaderSize:])
		if err != nil {
			return
		}
	}
	return
}

func TestZstdStreamClose(t *testing.T) {
	ctx := context.TODO()

	t.Run("zstd-stream-pipe", func(t *testing.T) {

		rr, ww := io.Pipe()
		r := &PipeReader{p: rr}
		w := &PipeWriter{p: ww}

		zstdw, err := zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.SpeedBestCompression),
			// zstd.WithEncoderConcurrency(1),
			// zstd.WithLowerEncoderMem(true),
		)
		if err != nil {
			t.Fatal(err)
		}

		zstdr, err := zstd.NewReader(r,
			zstd.WithDecoderConcurrency(1),
		)
		if err != nil {
			t.Fatal(err)
		}

		fRead := func(n int) {
			N := 0
			lastL := 0
			for i := 0; i < n; i++ {
				m, n := fromMsgPB(ctx, zstdr)
				msg := m.(*pb.Log)
				N += n
				t.Logf("Read[%d]: N:%d, L:%d, n:%d,l:%d", msg.LogId, N, r.L, n, r.L-lastL)
				lastL = r.L
			}
		}

		N := 0
		lastL := 0
		f := func(req *pb.Log, i int) {
			l := AllRandMsg()
			bsPB, _ := newMsgPB(ctx, l)

			half := len(bsPB) / 2
			n1, err := zstdw.Write(bsPB[:half])
			if err != nil {
				t.Fatal(err)
			}
			N += n1
			n2, err := zstdw.Write(bsPB[half:])
			if err != nil {
				t.Fatal(err)
			}
			// err = zstdw.Close()
			// if err != nil {
			// 	t.Fatal(err)
			// }
			N += n2
			if i%3 == 0 {
				err = zstdw.Flush() // Flush 之后才能读
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("Write[%d]: N:%d, L:%d n:%d, l:%d", l.LogId, N, w.L, n1+n2, w.L-lastL)
			lastL = w.L
		}
		for i := 0; i < 1000; i++ {
			req := RandMsg()
			f(req, i)
		}
		// select {}

		fRead(200)
		fRead(800)

		zstdr.Close()
		err = zstdw.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestZstdStream(t *testing.T) {
	ctx := context.TODO()

	t.Run("zstd-stream-pipe", func(t *testing.T) {

		rr, ww := io.Pipe()
		r := &PipeReader{p: rr}
		w := &PipeWriter{p: ww}

		zstdw, err := zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.SpeedBestCompression),
			// zstd.WithEncoderConcurrency(1),
			// zstd.WithLowerEncoderMem(true),
		)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			zstdr, err := zstd.NewReader(r,
				zstd.WithDecoderConcurrency(1),
			)
			if err != nil {
				t.Fatal(err)
			}
			N := 0
			lastL := 0
			for i := 0; i < 1000; i++ {
				m, n := fromMsgPB(ctx, zstdr)
				msg := m.(*pb.Log)
				N += n
				t.Logf("Read[%d]: N:%d, L:%d, n:%d,l:%d", msg.LogId, N, r.L, n, r.L-lastL)
				lastL = r.L
			}
		}()

		N := 0
		lastL := 0
		f := func(req *pb.Log, i int) {
			l := AllRandMsg()
			bsPB, _ := newMsgPB(ctx, l)

			half := len(bsPB) / 2
			n1, err := zstdw.Write(bsPB[:half])
			if err != nil {
				t.Fatal(err)
			}
			N += n1
			n2, err := zstdw.Write(bsPB[half:])
			if err != nil {
				t.Fatal(err)
			}
			N += n2
			if i%3 == 0 {
				err = zstdw.Flush() // Flush 之后才能读
				if err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("Write[%d]: N:%d, L:%d n:%d, l:%d", l.LogId, N, w.L, n1+n2, w.L-lastL)
			lastL = w.L
		}
		for i := 0; i < 1000; i++ {
			req := RandMsg()
			f(req, i)
		}
		// select {}
	})
}

func TestLZ4(t *testing.T) {
	ctx := context.TODO()

	t.Run("lz4-stream-0", func(t *testing.T) {
		rr, ww := io.Pipe()
		r := &PipeReader{p: rr}
		w := &PipeWriter{p: ww}

		lzw := lz4.NewWriter(w)
		lzr := lz4.NewReader(r)
		bsOut := make([]byte, 0x7fff)

		lzw.Apply(lz4.ConcurrencyOption(1))

		wg := sync.WaitGroup{}
		wg.Add(1)
		// bsPB ->lzw -> w-r -> lzr -> bsOut
		go func() {
			defer wg.Done()
			N := 0
			for i := 0; i < 10; i++ {
				n, err := lzr.Read(bsOut[:1024])
				if err != nil && err != io.EOF {
					t.Error(err)
				}
				N += n
				t.Logf("Reader: N: %d, L:%d", N, r.L)
			}
		}()

		N := 0
		for i := 0; i < 10; i++ {
			bsPB, _ := newMsgPB(ctx, RandMsg())
			n, err := lzw.Write(bsPB) // n: 写入了多少字节
			if err != nil {
				t.Error(err)
			}
			// if i%2 == 1 {
			if err = lzw.Flush(); err != nil {
				t.Fatal(err)
			}
			// }
			N += n
			t.Logf("Writer: N: %d, L:%d", N, w.L)

		}
		// wg.Wait()
	})

	t.Run("lz4-stream", func(t *testing.T) {
		req := RandMsg()
		buf := proto.NewBuffer(nil)
		err := buf.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		bsPB := buf.Bytes()

		r := bytes.NewBuffer(bsPB)
		if err != nil {
			t.Fatal(err)
		}
		r.Write(bsPB)
		r.Write(bsPB)
		r.Write(bsPB)
		r.Write(bsPB)

		w := bytes.NewBuffer(nil)
		if err != nil {
			t.Fatal(err)
		}
		lzw := lz4.NewWriter(w)
		// err = lzw.Apply(lz4.CompressionLevelOption(lz4.Level8))
		// if err != nil {
		// 	t.Fatal(err)
		// }
		n, err := io.Copy(lzw, r)
		if err != nil {
			t.Error(err)
		}
		// err = lzw.Close()
		err = lzw.Flush()
		if err != nil {
			t.Fatal(err)
		}

		out := w.Bytes()

		t.Logf("%d -> %d, n: %d, %d", len(bsPB), len(out), w.Len(), n)
		t.Logf("%s", string(bsPB))
		t.Logf("%s", string(out))

		lzr := lz4.NewReader(bytes.NewBuffer(out))
		if err != nil {
			t.Fatal(err)
		}
		rw := bytes.NewBuffer(nil)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		n, err = io.Copy(rw, lzr)
		if err != nil {
			t.Error(err)
		}
		t.Logf("rw:%s", string(rw.Bytes()))
	})

	t.Run("lz4-0", func(t *testing.T) {
		req := RandMsg()
		buf := proto.NewBuffer(nil)
		err := buf.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		bsPB := buf.Bytes()
		// bsPB = append(bsPB, bsPB...)
		// bsPB = append(bsPB, bsPB...)
		// bsPB = append(bsPB, bsPB...)

		lz4Writer := &lz4.Compressor{}

		bsLZ4 := make([]byte, 1024*8)
		compressedSize, err := lz4Writer.CompressBlock(bsPB, bsLZ4)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d -> %d", len(bsPB), compressedSize)

		compressedSize, err = lz4Writer.CompressBlock(bsPB, bsLZ4)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%d -> %d", len(bsPB), compressedSize)

		bsUnLZ4 := make([]byte, 1024*32)
		compressedSize, err = lz4.UncompressBlock(bsLZ4[:compressedSize], bsUnLZ4)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("bsUnLZ4:%s", string(bsUnLZ4[:compressedSize]))
	})

	t.Run("lz4", func(t *testing.T) {
		req := RandMsg()
		buf := proto.NewBuffer(nil)
		err := buf.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		bsPB := buf.Bytes()
		bsPB = append(bsPB, bsPB...)
		bsPB = append(bsPB, bsPB...)
		bsPB = append(bsPB, bsPB...)

		bsLZ4 := make([]byte, 1024*8)
		compressedSize, err := lz4.CompressBlock(bsPB, bsLZ4, nil)
		if err != nil {
			t.Fatal(err)
		}

		t.Logf("%d -> %d", len(bsPB), compressedSize)
		t.Logf("%s", string(bsPB))
		t.Logf("%s", string(bsLZ4[:compressedSize]))

		bsUnLZ4 := make([]byte, 1024*32)
		compressedSize, err = lz4.UncompressBlock(bsLZ4[:compressedSize], bsUnLZ4)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("bsUnLZ4:%s", string(bsUnLZ4[:compressedSize]))
	})
	t.Run("zstd", func(t *testing.T) {
		req := RandMsg()
		buf := proto.NewBuffer(nil)
		err := buf.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		bsPB := buf.Bytes()
		bsPB = append(bsPB, bsPB...)
		bsPB = append(bsPB, bsPB...)

		bsLZ4 := make([]byte, 1024)

		w, err := zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.SpeedFastest),
			// zstd.WithEncoderConcurrency(1),
			// zstd.WithLowerEncoderMem(true),
		)
		if err != nil {
			t.Fatal(err)
		}
		bsLZ4 = w.EncodeAll(bsPB, bsLZ4)

		t.Logf("%d -> %d", len(bsPB), len(bsLZ4))
	})

	t.Run("zstd-stream", func(t *testing.T) {

		buf := bytes.NewBuffer(nil)

		zstdw, err := zstd.NewWriter(buf,
			zstd.WithEncoderLevel(zstd.SpeedFastest),
			// zstd.WithEncoderConcurrency(1),
			// zstd.WithLowerEncoderMem(true),
		)
		if err != nil {
			t.Fatal(err)
		}

		bsOut := make([]byte, 0x7fff)

		f := func(bsPB []byte) {
			n, err := zstdw.Write(bsPB)
			if err != nil {
				t.Fatal(err)
			}
			err = zstdw.Flush()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("bsPB:%d -> n:%d, buf:%d, %d", len(bsPB), n, buf.Len(), len(buf.Bytes()))

			zstdr, err := zstd.NewReader(buf,
				zstd.WithDecoderConcurrency(1),
			)
			if err != nil {
				t.Fatal(err)
			}
			n, err = zstdr.Read(bsOut)
			if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				t.Fatal(err)
			}
			t.Logf("bsOut:%d -> n:%d, buf:%d, %d", len(bsOut[:n]), n, buf.Len(), len(buf.Bytes()))
		}
		for i := 0; i < 2; i++ {
			bsPB, _ := newMsgPB(ctx, RandMsg())
			f(bsPB)
		}
		// zstdr.Close()
		// if err != nil {
		// 	t.Fatal(err)
		// }
	})

	t.Run("zstd-stream-pipe", func(t *testing.T) {

		rr, ww := io.Pipe()
		r := &PipeReader{p: rr}
		w := &PipeWriter{p: ww}

		zstdw, err := zstd.NewWriter(w,
			zstd.WithEncoderLevel(zstd.SpeedFastest),
			// zstd.WithEncoderConcurrency(1),
			// zstd.WithLowerEncoderMem(true),
		)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			zstdr, err := zstd.NewReader(r,
				zstd.WithDecoderConcurrency(1),
			)
			if err != nil {
				t.Fatal(err)
			}
			N := 0
			lastL := 0
			for i := 0; i < 10; i++ {
				// // n, err := zstdr.Read(bsOut)
				// n, err := io.ReadFull(zstdr, bsOut[:1024])
				// if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
				// 	t.Fatal(err)
				// }
				m, n := fromMsgPB(ctx, zstdr)
				msg := m.(*pb.Log)
				N += n
				t.Logf("Read[%d]: N:%d, L:%d, n:%d,l:%d", msg.LogId, N, r.L, n, r.L-lastL)
				lastL = r.L
			}
		}()

		N := 0
		lastL := 0
		f := func(req *pb.Log, i int) {
			l := RandMsg()
			bsPB, _ := newMsgPB(ctx, l)

			half := len(bsPB) / 2
			n1, err := zstdw.Write(bsPB[:half])
			if err != nil {
				t.Fatal(err)
			}
			N += n1
			n2, err := zstdw.Write(bsPB[half:])
			if err != nil {
				t.Fatal(err)
			}
			N += n2
			err = zstdw.Flush()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("Write[%d]: N:%d, L:%d n:%d, l:%d", l.LogId, N, w.L, n1+n2, w.L-lastL)
			lastL = w.L
		}
		for i := 0; i < 10; i++ {
			req := RandMsg()
			f(req, i)
		}
		// select {}
	})
}

type FakeWriter struct{}

func (w *FakeWriter) Write(data []byte) (n int, err error) {
	return len(data), nil
}

type PipeWriter struct {
	p *io.PipeWriter
	L int

	ch    chan []byte
	wonce sync.Once
}

func (w *PipeWriter) Write(data []byte) (n int, err error) {
	w.L += len(data)
	return w.p.Write(data)
}

func (w *PipeWriter) Write1(data []byte) (n int, err error) {
	w.L += len(data)
	w.wonce.Do(func() {
		w.ch = make(chan []byte, 1024*100)
		go func() {
			for data := range w.ch {
				w.p.Write(data)
			}
		}()
	})
	n = len(data)
	w.ch <- data
	return
}

type PipeReader struct {
	p *io.PipeReader
	L int

	ch    chan []byte
	ronce sync.Once
}

func (w *PipeReader) Read(data []byte) (n int, err error) {
	n, err = w.p.Read(data)
	w.L += n
	return
}
func (w *PipeReader) Read1(data []byte) (n int, err error) {
	w.ronce.Do(func() {
		w.ch = make(chan []byte, 1024*100)
		var d []byte
		go func() {
			for {
				n, err = w.p.Read(d)
				if err == nil {
					w.ch <- d
				}
			}
		}()
	})
	d := <-w.ch
	n = copy(data, d)
	w.L += n
	return
}

type FakeReader struct {
	bs []byte
	i  int
}

func (r *FakeReader) Read(data []byte) (n int, err error) {
	for n < len(data) {
		i := r.i % len(r.bs)
		m := copy(data[n:], r.bs[i:])
		r.i += m
		n += m
	}
	return
}

func BenchmarkL_zstd(b *testing.B) {
	ctx := context.TODO()

	b.Run("zstd-Read", func(b *testing.B) {
		bspb, _ := newMsgPB(ctx, RandMsg())
		bsz := make([]byte, 1024*4)
		bsz1 := make([]byte, 1024*4)

		zstdw, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
		if err != nil {
			b.Fatal(err)
		}
		bsz = zstdw.EncodeAll(bspb, bsz[:0])
		zstdr0, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
		if err != nil {
			b.Fatal(err)
		}
		bsz1, err = zstdr0.DecodeAll(bsz, bsz1[:0])
		if err != nil {
			b.Fatal(err)
		}

		r := &FakeReader{
			bs: bsz,
		}

		zstdr, err := zstd.NewReader(r, zstd.WithDecoderConcurrency(1))
		if err != nil {
			b.Fatal(err)
		}

		bsOut := make([]byte, 0x7fff)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			n, err := zstdr.Read(bsOut)
			if err != nil || n != len(bspb) {
				b.Fatal(err)
			}
		}
	})

	w := &FakeWriter{}

	zstdw, err := zstd.NewWriter(w,
		zstd.WithEncoderLevel(zstd.SpeedFastest),
		zstd.WithEncoderConcurrency(1),
		// zstd.WithEncoderCRC(false),
	)
	if err != nil {
		b.Fatal(err)
	}

	b.Run("zstd-Write-Flush", func(b *testing.B) {
		pbs := make([][]byte, b.N)
		for i := range pbs {
			pbs[i], _ = newMsgPB(ctx, RandMsg())
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			n, err := zstdw.Write(pbs[i])
			if err != nil || n != len(pbs[i]) {
				b.Fatal(err)
			}
			err = zstdw.Flush()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("zstd-Write", func(b *testing.B) {
		pbs := make([][]byte, b.N)
		for i := range pbs {
			pbs[i], _ = newMsgPB(ctx, RandMsg())
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			n, err := zstdw.Write(pbs[i])
			if err != nil || n != len(pbs[i]) {
				b.Fatal(err)
			}
		}
		err = zstdw.Flush()
		if err != nil {
			b.Fatal(err)
		}
	})
}

func BenchmarkL_lz4(b *testing.B) {
	ctx := context.TODO()

	b.Run("lz4-Read", func(b *testing.B) {

		lz4buf := bytes.NewBuffer(nil)

		lzw := lz4.NewWriter(lz4buf)

		for i := 0; i < b.N; i++ {
			bsPB, _ := newMsgPB(ctx, RandMsg())
			_, err := lzw.Write(bsPB) // n: 写入了多少字节
			if err != nil {
				b.Error(err)
			}
			// if err := lzw.Flush(); err != nil {
			// 	b.Fatal(err)
			// }
		}
		if err := lzw.Close(); err != nil {
			b.Fatal(err)
		}

		bsOut0 := make([]byte, 0x7fff)
		lzr0 := lz4.NewReader(bytes.NewBuffer(lz4buf.Bytes()))
		n, err := lzr0.Read(bsOut0)
		if err != nil && err != io.EOF {
			b.Error(err)
		}
		bsOut0 = bsOut0[:n]
		if n < 100 {
			b.Fatal("err")
		}

		r := lz4buf
		lzr := lz4.NewReader(r)

		// bsOut := make([]byte, 0x7fff)
		b.ResetTimer()
		count := 0
		for i := 0; i < b.N; i++ {
			n, err := lzr.Read(bsOut0)
			if err == io.EOF {
				count++
				continue
			}
			if err != nil || n != len(bsOut0) {
				b.Fatal(err)
			}
		}
		if count != 0 {
			b.Log("count:", count)
		}
	})

	b.Run("lz4-Write", func(b *testing.B) {
		w := &FakeWriter{}
		lzw := lz4.NewWriter(w)
		err := lzw.Apply(lz4.ConcurrencyOption(1))
		if err != nil {
			b.Fatal(err)
		}

		pbs := make([][]byte, b.N)
		for i := range pbs {
			pbs[i], _ = newMsgPB(ctx, RandMsg())
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			n, err := lzw.Write(pbs[i])
			if err != nil || n != len(pbs[i]) {
				b.Fatal(err)
			}
			err = lzw.Flush()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func AllRandMsg() (log *pb.Log) {
	log = &pb.Log{
		LogId:           gid.GetGID(),
		EventId:         randInt64(0),
		LogonId:         randInt64(0),
		Platform:        "windows",
		Username:        randStr(10),
		AgentId:         1,
		Time:            time.Now().UnixNano(),
		EventTime:       time.Now().UnixNano(),
		Source:          randStr(20),
		SessionId:       1123243343,
		Description:     randStr(15),
		Product:         randStr(38),
		Company:         randStr(15),
		ParentPath:      randStr(15),
		Pid:             rand.Int63n(100000),
		ParentCmdline:   randStr(50),
		Cmdline:         randStr(20),
		Path:            randStr(35),
		Name:            randStr(15),
		State:           randStr(5),
		Cwd:             randStr(20),
		Root:            randStr(20),
		Euid:            111,
		Egid:            2222,
		PipeName:        randStr(45),
		PipeStatus:      randStr(8),
		DestPath:        randStr(45),
		CallTrace:       randStr(8),
		ImageLoaded:     randStr(8),
		Signature:       strconv.Itoa(rand.Intn(100000)),
		SignatureStatus: randStr(15),
		Key:             randStr(5),
		TargetObject:    randStr(15),
		EventType:       randStr(15),
		NewKey:          randStr(15),
		RelativePath:    randStr(15),
		Query:           "Microsoft-Windows-Sysmon",
		Script:          randStr(15),
		Consumer:        uuid.NewString(),
		Filter:          strings.ToUpper(uuid.NewString()),
		QueryName:       uuid.NewString(),
	}
	return
}

func RandMsg() (log *pb.Log) {
	log = &pb.Log{
		LogId:           gid.GetGID(),
		EventId:         randInt64(0),
		LogonId:         randInt64(0),
		Platform:        "windows",
		Username:        "SYSTEM " + randStr(5),
		AgentId:         1,
		Time:            time.Now().UnixNano(),
		EventTime:       time.Now().UnixNano(),
		Source:          "wertyuiopasfghjkl",
		SessionId:       1123243343,
		Description:     randStr(32),
		Product:         randStr(38),
		Company:         strconv.Itoa(rand.Intn(100000)),
		ParentPath:      "\\\\Users\\\\lacrimosa " + randStr(8),
		Pid:             rand.Int63n(100000),
		ParentCmdline:   "\\\\WINDOWS\\\\system32\\\\" + randStr(8) + ".exe",
		Cmdline:         randStr(20),
		Path:            "\\\\Windows\\\\System32\\\\" + randStr(8) + ".exe",
		Name:            "SHA256=" + randStr(64),
		State:           "HELLOWORLD\\\\lacrimosa" + randStr(8) + ".exe",
		Cwd:             randStr(20),
		Root:            "-",
		Euid:            111,
		Egid:            2222,
		PipeName:        "\\\\Windows\\\\System32\\\\" + randStr(8) + ".exe",
		PipeStatus:      "IP Configuration Utility" + randStr(8),
		DestPath:        "ipconfig.exe",
		CallTrace:       "HELLOWORLD\\\\lacrimosa" + randStr(8),
		ImageLoaded:     "1 " + randStr(8),
		Signature:       strconv.Itoa(rand.Intn(100000)),
		SignatureStatus: " ProcessCreate)",
		Key:             "1",
		TargetObject:    "HelloWorld 999",
		EventType:       "Microsoft-Windows-Sysmon/Operational",
		NewKey:          strconv.FormatInt(time.Now().Unix()+rand.Int63n(100000), 10),
		RelativePath:    strconv.Itoa(rand.Intn(100000)),
		Query:           "Microsoft-Windows-Sysmon",
		Script:          strconv.Itoa(rand.Intn(100000)),
		Consumer:        uuid.NewString(),
		Filter:          strings.ToUpper(uuid.NewString()),
		QueryName:       uuid.NewString(),
	}
	return
}

func randStr(n int) string {
	chars := []byte(`1234567890-=_+qwertyuiop[]{}asdfghjkl;':"zxcvbnm,./<>?QWERTYUIOPASDFGHJKLZXCVBNM`)
	bs := make([]byte, n)
	for i := range bs {
		x := rand.Intn(len(chars))
		bs[i] = chars[x]
	}

	return string(bs)
}

func randInt64(n int64) int64 {
	if n == 0 {
		return rand.Int63()
	}
	return rand.Int63n(n)
}
