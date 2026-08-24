package atlas_test

import (
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lxt1045/utils/atlas"
)

/*
Atlas 提供了两个主要的 diff 命令，你需要根据场景选择：
 1. atlas schema diff：用于一次性对比两个不同的 Schema 来源（如两个数据库、或一个数据库与一个 SQL 文件），并直接输出差异 SQL。它不涉及版本管理，适合临时对比或紧急修改。
 2. atlas migrate diff (推荐)：专为版本化迁移设计。它接受一个“理想状态”（SQL/HCL文件）和一个“迁移目录”，自动生成增量的、带版本号的 SQL 迁移文件。这是实现你“版本增量”需求的标准方法。
*/

/*
// atlas schema apply (声明式)： 不生成文件，直接修改数据库； 默认采用“安全模式”不会删表/列；不建议用于生产环境
// 1. 计算差异，并输出
--format '{{ sql . }}' : 一行 sql ;
--format '{{ sql . \"  \" }}' : 转成多行，方便查看 ;
atlas schema diff --from "mysql://root:password@127.0.0.1:3306/dji1" --to "file://e:/test/atlas/test.sql" --format '{{ sql . \"  \" }}' --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev"
atlas schema diff --from "mysql://root:password@127.0.0.1:3306/dji1" --to "file://e:/test/atlas/test.sql" --format '{{ sql . }}' --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev"
// 2. 强制将目标数据库的当前结构，变更为 schema.sql 文件中定义的“理想状态”。
// --dry-run 只看不执行; --schema apply --allow-drops 非安全模式。
atlas schema apply -u "mysql://localhost" --to "file://schema.sql" --dev-url "docker://mysql/8/dev"
*/
func Test_schemaDiff(t *testing.T) {
	fromURL := []string{"mysql://root:password@127.0.0.1:3306/dji1"}
	toURL := []string{"file://e:/test/atlas/test.sql"}
	schemas, exclude := []string{}, []string{}
	formatTo := "{{ sql . \"  \" }}"
	devURL := "mysql://root:password@127.0.0.1:3306/atlas_dev"
	err := atlas.SchemaDiffRun(t.Context(), fromURL, toURL, schemas, exclude, formatTo, devURL)

	if err != nil {
		t.Fatal(err)
	}
}

func Test_schemaInspectRun(t *testing.T) {
	fromURL := []string{"mysql://root:password@127.0.0.1:3306/dji88"}
	// toURL := []string{"file://e:/test/atlas/test.sql"}
	schemas, exclude := []string{}, []string{}
	formatTo := "{{ sql . \"  \" }}"
	devURL := "mysql://root:password@127.0.0.1:3306/atlas_dev"
	err := atlas.SchemaInspectRun(t.Context(), fromURL, schemas, exclude, formatTo, devURL)

	if err != nil {
		t.Fatal(err)
	}
}

/*
// atlas migrate diff/apply (版本化)： 生成带时间戳的 xxx.sql 版本文件
// 1. 基线文件, initial 只是名字
atlas migrate diff initial --dir "file://e:/test/atlas/migrations?format=golang-migrate" --to "file://e:/test/atlas/test.sql" --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev" --format '{{ sql . \"  \" }}'

// 2. 如果数据库已存在，你需要告诉 Atlas 不要应用这个基线，因为它已经存在了
// 结果存于 url 指向的DB的 atlas_schema_revisions 表中
atlas migrate set 20260807064528 --dir "file://e:/test/atlas/migrations?format=golang-migrate"  --url "mysql://root:password@127.0.0.1:3306/dji1"

// 3. 修改 test.sql 后, 会与 migrations/ 目录下已有的所有迁移文件进行对比，生成一个新的版本化 SQL 文件
atlas migrate diff test_dev_name --dir "file://e:/test/atlas/migrations?format=golang-migrate" --to "file://e:/test/atlas/test.sql" --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev" --format '{{ sql . \"  \" }}'

// 4. 应用增量迁移, 运行 atlas migrate apply 命令; 会检查数据库的当前迁移版本后按顺序应用所有尚未执行的迁移文件
atlas migrate apply --dir "file://migrations" --url "mysql://..."
*/
func Test_migrateDiff(t *testing.T) {
	// fromURL := []string{"mysql://root:password@127.0.0.1:3306/dji1"}
	toURL := []string{"file://e:/test/atlas/test.sql"}
	schemas := []string{}
	dirURL := "file://e:/test/atlas/migrations?format=golang-migrate"
	name := "name_by_test_" + time.Now().Format("20060102_150405")
	formatTo := "{{ sql . \"  \" }}"
	devURL := "mysql://root:password@127.0.0.1:3306/atlas_dev"
	err := atlas.MigrateDiffRun(t.Context(), toURL, schemas, name, formatTo, dirURL, devURL, "")
	if err != nil {
		t.Fatal(err)
	}
}

/*
# 执行的时候也需要执行 golang-migrate 格式, 否则 up 和 down 文件都会被执行。
atlas migrate apply --dir "file://e:/test/atlas/migrations?format=golang-migrate" - --url ""mysql://root:password@127.0.0.1:3306/dji88"
*/
func Test_migrateApply(t *testing.T) {
	toURL := "mysql://root:password@127.0.0.1:3306/dji88"
	fromURL := "file://e:/test/atlas/migrations?format=golang-migrate"
	err := atlas.MigrateApplyRun(t.Context(), fromURL, toURL)
	if err != nil {
		t.Fatal(err)
	}
}

func Test_MigrateHashRun(t *testing.T) {
	dirURL := "file://e:/test/atlas/migrations?format=golang-migrate"
	err := atlas.MigrateHashRun(t.Context(), dirURL)
	if err != nil {
		t.Fatal(err)
	}
}
