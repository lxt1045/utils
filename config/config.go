package config

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/lxt1045/errors"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
)

type DB struct {
	Host             string
	Port             string
	User             string
	Password         string
	DBName           string
	SSLMode          bool
	WriteConcurrency int
	Span             int
}

type GRPC struct {
	CACert     string
	ServerKey  string
	ServerCert string
	ClientKey  string
	ClientCert string
	Protocol   string
	Addr       string
	Host       string
	HostAddrs  []string
}

type Conn struct {
	TCP              string
	Proto            string
	ProxyAddr        string
	Addr             string
	Host             string
	EnableTLS        bool
	ReadConcurrency  int
	WriteConcurrency int
	ReadWindow       int
	WriteWindow      int
	FlushTime        int
	Heartbeat        int
	Bandwidth        int // 带宽限制：Mbps
	TLS              TLS
}
type TLS struct {
	CACert     string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
}

type Queue struct {
	DBFile     string
	Limit      string
	CacheLimit string
	CAPassword string
	CAIv       string
}

type Log struct {
	StoreLevel string // 写到存储的 level
	LogLevel   string
	ToConsole  bool

	// 以下是 lumberjack 配置

	// 日志大小到达MaxSize(MB)就开始backup，默认值是100.
	MaxSize int
	// 旧日志保存的最大天数，默认保存所有旧日志文件
	MaxAge int
	// 旧日志保存的最大数量，默认保存所有旧日志文件
	MaxBackups int
	// 对backup的日志是否进行压缩，默认不压缩
	Compress bool
	// 是否使用本地时间，否则使用UTC时间
	LocalTime bool
	// 日志文件名，归档日志也会保存在对应目录下
	// 若该值为空，则日志会保存到os.TempDir()目录下，日志文件名为
	// <processname>-lumberjack.log
	Filename string
}

func UnmarshalFS(file string, fsStatic embed.FS, conf interface{}) (err error) {
	bs, err := fs.ReadFile(fsStatic, file)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	err = Unmarshal(bs, conf)
	if err != nil {
		return
	}
	return
}

func Unmarshal(bs []byte, conf interface{}) (err error) {
	m := make(map[string]interface{})
	err = yaml.Unmarshal(bs, m)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	err = yaml.Unmarshal(bs, conf)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	c := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           conf,
		WeaklyTypedInput: true,
		// DecodeHook: mapstructure.ComposeDecodeHookFunc(
		// 	// mapstructure.StringToTimeDurationHookFunc(),
		// 	// mapstructure.StringToSliceHookFunc(","),
		// 	AssignVarsFromEnvHookFunc(),
		// ),
		MatchName: func(mapKey, fieldName string) bool {
			ok := Camel2Case(mapKey) == Camel2Case(fieldName)
			// if ok {
			// 	log.Printf("%+v:%+v", mapKey, fieldName)
			// }
			return ok
		},
	}

	decoder, err := mapstructure.NewDecoder(c)
	if err != nil {
		err = errors.Errorf(err.Error())
		return err
	}
	err = decoder.Decode(m)
	if err != nil {
		err = errors.Errorf(err.Error())
		return err
	}
	// 环境变量处理
	err = AssignVarsFromEnv(conf)
	if err != nil {
		return
	}

	return
}

// 环境变量处理
func AssignVarsFromEnv(conf interface{}) (err error) {
	if conf == nil {
		return
	}
	valueIn := reflect.ValueOf(conf)
	if valueIn.Type().Kind() != reflect.Pointer {
		err = errors.Errorf("conf must be pointer")
		return
	}
	valueIn = valueIn.Elem()

	switch valueIn.Type().Kind() {
	case reflect.Pointer:
		err = AssignVarsFromEnv(valueIn.Interface())
		if err != nil {
			return
		}
		return
	case reflect.Interface:
		vUnderlying := valueIn.Elem()            // interface 底层的值
		vpNew := reflect.New(vUnderlying.Type()) // 不能直接取地址，所以需要先创建一个新的变量，再做修改
		vpNew.Elem().Set(vUnderlying)
		err = AssignVarsFromEnv(vpNew.Interface())
		if err != nil {
			return
		}

		// 获取一个底层类型是 interface{} 类型的 interface{}
		ifaces := []interface{}{vpNew.Elem().Interface()}
		vIfaces := reflect.ValueOf(ifaces)
		valueIn.Set(vIfaces.Index(0))
		return
	case reflect.String:
		str1 := valueIn.String()
		str, ok := AssignVarFromEnv(str1)
		if ok {
			if !valueIn.CanSet() {
				err = errors.Errorf("conf is can not set")
				return
			}
			valueIn.Set(reflect.ValueOf(str))
			if str == "" {
				i := strings.IndexByte(str1, '}')
				str1 = strings.TrimSpace(str1[i+1:])
				str1 = strings.TrimLeft(str1, "|")
				str1 = strings.TrimSpace(str1)
				if str1 != "" {
					valueIn.Set(reflect.ValueOf(str1))
				}
			}
		} else if valueIn.CanSet() {
		}
		return
	case reflect.Slice, reflect.Array:
		err = AssignSliceFromEnv(valueIn.Addr().Interface())
		return
	case reflect.Map:
		// map 在 golang 中是一个指针，所以这里不需要重新给 conf 赋值
		_, err = AssignMapFromEnv(valueIn.Interface())
		return
	case reflect.Struct:
		for i := 0; i < valueIn.NumField(); i++ {
			v := valueIn.Field(i)
			if !v.IsValid() {
				continue // 跳过0值
			}
			for v.Type().Kind() == reflect.Pointer {
				v = v.Elem()
			}
			if !v.IsValid() {
				continue // 跳过0值
			}
			if v.Addr().CanInterface() {
				err = AssignVarsFromEnv(v.Addr().Interface())
				if err != nil {
					return
				}
			}
		}
		return
	default:
	}
	return
}

// 从环境变量给变量赋值，变量格式为: ${Var}
func AssignVarFromEnv(v string) (out string, ok bool) {
	v = strings.TrimSpace(v)
	i := strings.IndexByte(v, '}')
	if i < 0 || len(v) <= 3 || v[0] != '$' || v[1] != '{' {
		return
	}
	ok = true
	v = v[2:i]
	out, _ = os.LookupEnv(v)
	return
}

func AssignMapFromEnv(m interface{}) (out interface{}, err error) {
	vm := reflect.ValueOf(m)
	if !vm.IsValid() {
		return
	}
	for vm.Type().Kind() == reflect.Pointer {
		vm = vm.Elem()
	}
	if !vm.IsValid() {
		return
	}
	if vm.Type().Kind() != reflect.Map {
		err = errors.Errorf("m must be map")
		return
	}
	iter := vm.MapRange()
	for iter.Next() {
		k := iter.Key()
		v := iter.Value()
		if !v.IsValid() {
			continue
		}
		for v.Type().Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if !v.IsValid() {
			return
		}

		vpNew := reflect.New(v.Type()) // 不能直接取地址，所以需要先创建一个新的变量，再做修改
		vpNew.Elem().Set(v)
		err = AssignVarsFromEnv(vpNew.Interface())
		if err != nil {
			return
		}

		vm.SetMapIndex(k, vpNew.Elem())
	}
	return m, nil
}

func AssignSliceFromEnv(m interface{}) (err error) {
	vm := reflect.ValueOf(m)
	if !vm.IsValid() {
		return
	}
	for vm.Type().Kind() == reflect.Pointer {
		vm = vm.Elem()
	}
	if !vm.IsValid() {
		return
	}
	if vm.Type().Kind() != reflect.Slice && vm.Type().Kind() != reflect.Array {
		err = errors.Errorf("m must be slice or array")
		return
	}
	for i, n := 0, vm.Len(); i < n; i++ {
		v := vm.Index(i)
		if !v.IsValid() {
			continue
		}

		err = AssignVarsFromEnv(v.Addr().Interface())
		if err != nil {
			return
		}
	}
	return
}
func AssignVarsFromEnvHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Kind,
		t reflect.Kind,
		data interface{}) (interface{}, error) {
		if f != reflect.String || t != reflect.String {
			return data, nil
		}

		raw, ok := data.(string)
		if !ok || raw == "" {
			return data, nil
		}
		raw, ok = AssignVarFromEnv(raw)
		if !ok {
			return data, nil
		}

		return raw, nil
	}
}

// 驼峰式写法转为下划线写法
func Camel2Case(name string) string {
	buf := bytes.NewBuffer(make([]byte, 0, len(name)*3/2))

	var lastUpperRun rune
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i == 0 {
				lastUpperRun = r
				continue
			}
			if lastUpperRun == 0 {
				if bs := buf.Bytes(); len(bs) == 0 || bs[len(bs)-1] != '_' {
					buf.WriteByte('_')
				}
				lastUpperRun = r
				continue
			}
			buf.WriteRune(unicode.ToLower(lastUpperRun))
			lastUpperRun = r
			continue
		}
		if lastUpperRun > 0 {
			var lastByte byte
			if bs := buf.Bytes(); len(bs) > 0 {
				lastByte = bs[len(bs)-1]
			}
			if lastByte != 0 && lastByte != '_' && r != '-' {
				buf.WriteByte('_')
			}
			buf.WriteRune(unicode.ToLower(lastUpperRun))
			lastUpperRun = 0
		}
		if r == '-' {
			buf.WriteRune('_')
			continue
		}
		buf.WriteRune(r)
	}
	if lastUpperRun > 0 {
		buf.WriteRune(unicode.ToLower(lastUpperRun))
		lastUpperRun = 0
	}
	return buf.String()
}

func LoadTLSConfig(efs embed.FS, certFile, keyFile, caFile string) (c *tls.Config, err error) {
	if keyFile == "" && certFile == "" {
		return nil, nil
	}
	if keyFile == "" || certFile == "" {
		return nil, errors.Errorf(`keyFile == "" || certFile == "" `)
	}

	certPEM, err := fs.ReadFile(efs, certFile)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	keyPEM, err := fs.ReadFile(efs, keyFile)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}

	var caPEM []byte
	if caFile != "" {
		caPEM, err = fs.ReadFile(efs, caFile)
		if err != nil {
			err = errors.Errorf(err.Error())
			return
		}
	}

	return BytesToTLSConfig(certPEM, keyPEM, caPEM)
}

func BytesToTLSConfig(certPEM, keyPEM, caPEM []byte) (c *tls.Config, err error) {
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		err = errors.Errorf("certPEM or keyPEM is empty")
		return
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}

	if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM(caPEM)
		if !ok {
			return nil, errors.Errorf("err")
		}

		tlsConfig.RootCAs = pool
		tlsConfig.ClientCAs = pool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func ParseBytes(str string, defaultValue int64) (bytes int64, err error) {
	str = strings.ToUpper(str)
	str = strings.TrimSuffix(str, "B")

	var unit int64 = 1
	switch {
	case strings.HasSuffix(str, "G"):
		str = str[:len(str)-1]
		unit = 1024 * 1024 * 1024
	case strings.HasSuffix(str, "M"):
		str = str[:len(str)-1]
		unit = 1024 * 1024
	case strings.HasSuffix(str, "K"):
		str = str[:len(str)-1]
		unit = 1024
	}
	str = strings.TrimSpace(str)
	bytes, err = strconv.ParseInt(str, 10, 64)
	if err != nil {
		bytes = defaultValue
		return
	}
	bytes *= unit
	return
}
