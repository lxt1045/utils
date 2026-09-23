package ck

import (
	"database/sql"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/jmoiron/sqlx"
	"github.com/tidwall/gjson"
	"github.com/lxt1045/utils/config"
)

type ToSqlxResult interface {
	IsNil() bool
	ToSqlxResult(stmt *sqlx.Stmt) (sql.Result, error)
}

type JSON string

func (j JSON) String() string {
	return string(j)
}

type Ponit struct {
	Longitude float64
	Latitude  float64
}

func (p Ponit) String() string {
	return "(" + strconv.FormatFloat(p.Longitude, 'f', 6, 64) + "," + strconv.FormatFloat(p.Latitude, 'f', 6, 64) + ")"
}

func PrepareSqlxField(t reflect.Type) (fields []string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			t1 := field.Type
			if t1.Kind() == reflect.Pointer {
				t1 = t1.Elem()
			}
			return PrepareSqlxField(t1)
		}
		tagv, bJson, exported := FiledDBName(t.Field(i), "db")
		if !exported {
			continue
		}
		_ = bJson
		fields = append(fields, tagv)
	}
	return
}

func PrepareSQLx(t reflect.Type, dbname, tableName string) (sql string) {
	sql = ""
	value := ""
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Anonymous {
			t1 := field.Type
			if t1.Kind() == reflect.Pointer {
				t1 = t1.Elem()
			}
			return PrepareSQLx(t1, dbname, tableName)
		}
		tagv, bJson, exported := FiledDBName(field, "db")
		if !exported {
			continue
		}
		_ = bJson
		if sql != "" {
			sql += ","
			value += ","
		}
		sql += "`" + tagv + "`"
		value += ":" + tagv
	}
	if sql != "" {
		if dbname != "" {
			dbname = "`" + dbname + "`."
		}
		sql = "INSERT INTO " + dbname + tableName + " (" + sql + ") VALUES (" + value + ")"
	}
	return
}

func FiledDBName(field reflect.StructField, tag string) (name string, bJson bool, exported bool) {
	if !field.IsExported() {
		return // 非导出成员不处理
	}
	// if field.Anonymous {
	// }

	tagv := strings.TrimSpace(field.Tag.Get("db"))
	if tagv == "-" {
		return
	}
	exported = true
	if tagv != "" {
		tvs := strings.Split(string(tagv), ",")
		tagv = strings.TrimSpace(tvs[0])
		for _, v := range tvs[1:] {
			if v == "json" {
				bJson = true
			}
		}
	}
	if tagv == "" {
		tagv = config.Camel2Case(field.Name)
	}
	name = tagv
	return
}

func ToP[T any](t T) (p *T) {
	return &t
}

func ToT[T any](p *T) (t T) {
	if p == nil {
		return
	}
	return *p
}

func BsJsonGetStr(bs []byte, path string) (out string) {
	data := unsafe.String(unsafe.SliceData(bs), len(bs))
	return JsonGetStr(data, path)
}

func JsonGetStr(data string, path string) (out string) {
	value := gjson.Get(data, path)
	if value.Exists() && value.Type != gjson.Null {
		return value.String()
	}
	return
}

func BsJsonGetInt64(bs []byte, path string) (out int64) {
	data := unsafe.String(unsafe.SliceData(bs), len(bs))
	return JsonGetInt64(data, path)
}

func JsonGetInt64(data string, path string) (out int64) {
	value := gjson.Get(data, path)
	if value.Exists() && value.Type != gjson.Number {
		return 0
	}
	return int64(value.Num)
}

func JsonGetArrayFirstStr(data string, path string) (out string) {
	value := gjson.Get(data, path)
	if value.Exists() && value.Type != gjson.Null {
		return value.String()
	}
	return
}
