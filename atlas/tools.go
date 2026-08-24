package atlas

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/utils/config"
	"github.com/lxt1045/utils/db"
	// _ "github.com/go-sql-driver/mysql"
)

/*
	atlas migrate diff test_name \
	  --dir "file://e:/test/atlas/migrations" \
	  --to "file://e:/test/atlas/test.sql" \
	  --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev" \
	  --format '{{ sql . \"  \" }}'
*/
// versionName 本次操作的名字
func MigrateDiff(ctx context.Context, versionName, inSqlFile, outMigrateDir string, fdb fs.FS, conf config.DB) (err error) {

	// toURL := []string{"file://e:/test/atlas/test.sql"}
	inSqlFile, err = filepath.Abs(inSqlFile)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	toURL := []string{"file://" + filepath.ToSlash(inSqlFile)}

	// devURL := "mysql://root:password@127.0.0.1:3306/atlas_dev"
	devURL := fmt.Sprintf("mysql://%s:%s@%s:%s/%s", conf.User, conf.Password, conf.Host, conf.Port, conf.AtlasDB.DBName)

	// dirURL := "file://e:/test/atlas/migrations?format=golang-migrate"
	outMigrateDir, err = filepath.Abs(outMigrateDir)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	dirURL := "file://" + filepath.ToSlash(outMigrateDir) + "?format=golang-migrate"

	// versionName = versionName   + time.Now().Format("20060102_150405")
	schemas := []string{}
	formatTo := "{{ sql . \"  \" }}"

	err = MigrateDiffRun(ctx, toURL, schemas, versionName, formatTo, dirURL, devURL, "")
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}

/*
	atlas migrate apply \
	  --dir "file://e:/test/atlas/migrations?format=golang-migrate"  \
	  --url ""mysql://root:password@127.0.0.1:3306/dji88"
*/
func MigrateApply(ctx context.Context, inMigrateDir string, conf config.DB) (err error) {
	err = db.CreateMysqlDB(ctx, conf)
	if err != nil {
		return
	}
	// toURL := "mysql://root:password@127.0.0.1:3306/dji88"
	// fromURL := "file://e:/test/atlas/migrations?format=golang-migrate"
	toURL := fmt.Sprintf("mysql://%s:%s@%s:%s/%s", conf.User, conf.Password, conf.Host, conf.Port, conf.DBName)
	fromURL := "file://" + filepath.ToSlash(inMigrateDir) + "?format=golang-migrate"
	err = MigrateApplyRun(ctx, fromURL, toURL)
	if err != nil {
		err = errors.WithErr(err)
		return
	}
	return
}
func MigrateApplyFS(ctx context.Context, fdb fs.FS, conf config.DB) (err error) {
	tmp, err := TempRangeDir()
	if err != nil {
		return
	}
	defer os.Remove(tmp)
	err = os.CopyFS(tmp, fdb)
	if err != nil {
		err = errors.WithErr(err)
		return
	}

	return MigrateApply(ctx, filepath.Join(tmp, conf.AtlasDB.MigrateDir), conf)
}

func CopyFSFile(file, fsFile string, fsys fs.FS) (err error) {
	r, err := fsys.Open(fsFile)
	if err != nil {
		err = errors.WithErr(err)
		return err
	}
	defer r.Close()
	info, err := r.Stat()
	if err != nil {
		return err
	}
	w, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0666|info.Mode()&0777)
	if err != nil {
		return err
	}

	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return &os.PathError{Op: "Copy", Path: file, Err: err}
	}
	return w.Close()
}

func TempRangeDir() (tmp string, err error) {
	tmp = os.TempDir()
	tmp = filepath.Join(tmp, RangeStr("atlas_", ""))
	err = os.MkdirAll(tmp, 0666)
	if err != nil {
		err = errors.WithErr(err)
		return
	}

	return
}

func RangeStr(pre, post string) (tmp string) {
	return pre + time.Now().Format("20060102150405") + "_" + strconv.FormatInt(rand.Int63n(10000000), 10) + post
}
