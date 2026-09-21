package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/lxt1045/errors"

	"gopkg.in/yaml.v3"
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
	DialTimeout      int
	ReadTimeout      int
	AtlasDB          AtlasDB
}
type AtlasDB struct {
	DBName     string
	MigrateDir string
}

type HTTP struct {
	ServerKey  string
	ServerCert string
	TLS        bool
	Relaese    bool
	Addr       string
	SwagerAddr string
	Static     bool
	Path       string
	Download   bool
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
	Host       string
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

func Env() string {
	for _, a := range os.Args {
		if a == "dev" {
			return "dev"
		}
		if a == "test" {
			return "test"
		}
		if a == "prod" {
			return "prod"
		}
	}
	return ""
}

func Init[T any](ctx context.Context, pfs *embed.FS, file, env string) (conf T, err error) {
	if env != "" {
		i := strings.LastIndexByte(file, '.')
		file = file[:i] + "_" + env + file[i:]
		fmt.Printf("run in %s ...\n", env)
	} else {
		fmt.Println("run in default ...")
	}

	bs, err := fs.ReadFile(pfs, file)
	if err != nil {
		err = errors.Errorf("file:%s, err:%s", file, err.Error())
		return
	}
	err = Unmarshal(bs, &conf)
	if err != nil {
		return
	}
	return
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

// Unmarshal 解析 yaml 到 conf，并处理两种环境变量赋值形式：
//
//  1. 值形式（仅 string）：字段值为 "${VAR}" 时替换为环境变量的值；
//     "${VAR}|default" 在环境变量不存在时取 "|" 后的默认值。
//  2. 注释形式（所有类型）：字段对应 yaml 节点的注释以 "${VAR}" 开头时，
//     用环境变量的值覆盖该字段（按 yaml 解析成字段类型，支持 int/bool/
//     float/slice/map/struct 等）；环境变量不存在时保持 yaml 中的值。
//
// 注释形式优先于值形式。环境变量查找顺序：系统环境变量 > .env 文件。
// yaml 的 key 与结构体成员的映射支持驼峰、下划线、中划线、大小写开头等形式。
func Unmarshal(bs []byte, conf interface{}) (err error) {
	v := reflect.ValueOf(conf)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return errors.Errorf("conf must be pointer")
	}
	var root yaml.Node
	if err = yaml.Unmarshal(bs, &root); err != nil {
		return errors.Errorf(err.Error())
	}
	// 加载 .env 文件中的环境变量；文件不存在或读取失败不影响程序运行
	envMap, _ := loadEnvFile(".env")
	return assignNode(v.Elem(), &root, envMap, "")
}

// loadEnvFile 加载 .env 文件并解析为 map
func loadEnvFile(filename string) (envMap map[string]string, err error) {
	envMap = make(map[string]string)

	data, err := os.ReadFile(filename)
	if err != nil {
		return envMap, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 移除值两端的引号
		value = strings.Trim(value, "\"'")

		envMap[key] = value
	}

	return envMap, nil
}

// AssignVarsFromEnv 对 conf 中的 "${VAR}" 字符串做环境变量替换（兼容旧接口）。
// 注释形式的环境变量赋值只在 Unmarshal/UnmarshalFS 中生效（需要 yaml 注释信息）。
func AssignVarsFromEnv(conf interface{}, envMap map[string]string) (err error) {
	if conf == nil {
		return
	}
	v := reflect.ValueOf(conf)
	if v.Kind() != reflect.Pointer {
		return errors.Errorf("conf must be pointer")
	}
	return assignValue(v, envMap)
}

// assignNode 按 yaml 节点树给 v 赋值，同时处理两种环境变量形式（见 Unmarshal 注释）。
// nil 指针字段在对应 yaml 节点存在（非 null）时按需分配，否则保持 nil。
// path 是到达当前节点的配置路径（如 "a.b[0].c"）：字段类型与 yaml 节点类型不匹配、
// 或值无法解码时，返回带完整路径和行号的错误；yaml 中多出的未知 key 会被忽略
// （同一配置文件可能被不同的 Config 结构体读取，不能因此报错）。
func assignNode(v reflect.Value, n *yaml.Node, envMap map[string]string, path string) (err error) {
	if n == nil {
		return
	}
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return
		}
		n = n.Content[0]
	}
	if n.Kind == yaml.AliasNode {
		n = n.Alias
		if n == nil {
			return
		}
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			if isNullNode(n) {
				return
			}
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			if isNullNode(n) {
				return
			}
			return errNodef(n, path, "expect mapping for struct %s, got %s", v.Type(), nodeDesc(n))
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			kn, vn := n.Content[i], n.Content[i+1]
			idx := fieldIndex(v.Type(), kn.Value)
			if idx == nil {
				continue
			}
			f := v.FieldByIndex(idx)
			if !f.CanSet() {
				continue
			}
			sub := joinPath(path, kn.Value)
			// 注释形式优先：注释以 "${VAR}" 开头时整体覆盖字段值；
			// 环境变量不存在时保持 yaml 中的值（回退到普通值解码）
			if name, ok := envNameFromComment(kn, vn); ok {
				if val, found := lookupEnv(name, envMap); found {
					if err = setEnvValue(f, val); err != nil {
						return errNodef(kn, sub, "env ${%s}: %s", name, err.Error())
					}
					continue
				}
			}
			if err = assignNode(f, vn, envMap, sub); err != nil {
				return
			}
		}
	case reflect.Map:
		if n.Kind != yaml.MappingNode {
			if isNullNode(n) {
				return
			}
			return errNodef(n, path, "expect mapping for %s, got %s", v.Type(), nodeDesc(n))
		}
		if v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			kn, vn := n.Content[i], n.Content[i+1]
			sub := joinPath(path, kn.Value)
			kv := reflect.New(v.Type().Key())
			if err = kn.Decode(kv.Interface()); err != nil {
				return errNodef(kn, sub, "cannot decode map key into %s: %s", v.Type().Key(), err.Error())
			}
			ev := reflect.New(v.Type().Elem())
			if err = assignNode(ev, vn, envMap, sub); err != nil {
				return
			}
			v.SetMapIndex(kv.Elem(), ev.Elem())
		}
	case reflect.Slice:
		if n.Kind != yaml.SequenceNode {
			if isNullNode(n) {
				return
			}
			return errNodef(n, path, "expect sequence for %s, got %s", v.Type(), nodeDesc(n))
		}
		v.Set(reflect.MakeSlice(v.Type(), len(n.Content), len(n.Content)))
		for i, en := range n.Content {
			if err = assignNode(v.Index(i), en, envMap, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return
			}
		}
	case reflect.Array:
		if n.Kind != yaml.SequenceNode {
			if isNullNode(n) {
				return
			}
			return errNodef(n, path, "expect sequence for %s, got %s", v.Type(), nodeDesc(n))
		}
		for i := 0; i < v.Len() && i < len(n.Content); i++ {
			if err = assignNode(v.Index(i), n.Content[i], envMap, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return
			}
		}
	case reflect.Interface:
		if isNullNode(n) {
			return
		}
		if err = n.Decode(v.Addr().Interface()); err != nil {
			return errNodef(n, path, "cannot decode %s: %s", nodeDesc(n), err.Error())
		}
		// 解码后的结构里可能还有 "${VAR}" 字符串，继续替换
		if v.IsNil() {
			return
		}
		tmp := reflect.New(v.Elem().Type())
		tmp.Elem().Set(v.Elem())
		if err = assignValue(tmp, envMap); err != nil {
			return
		}
		v.Set(tmp.Elem())
	case reflect.String:
		if err = n.Decode(v.Addr().Interface()); err != nil {
			return errNodef(n, path, "cannot decode %s into string: %s", nodeDesc(n), err.Error())
		}
		assignString(v, envMap)
	default:
		if isNullNode(n) {
			return
		}
		if err = n.Decode(v.Addr().Interface()); err != nil {
			return errNodef(n, path, "cannot decode %s into %s: %s", nodeDesc(n), v.Type(), err.Error())
		}
	}
	return
}

// errNodef 构造带配置路径和行号的错误。
func errNodef(n *yaml.Node, path, format string, args ...interface{}) error {
	if path == "" {
		path = "<root>"
	}
	loc := ""
	if n != nil && n.Line > 0 {
		loc = fmt.Sprintf(" (line %d)", n.Line)
	}
	return errors.Errorf("config %s%s: %s", path, loc, fmt.Sprintf(format, args...))
}

// nodeDesc 简要描述 yaml 节点（节点类型/标量值），用于错误信息。
func nodeDesc(n *yaml.Node) string {
	if n == nil {
		return "null"
	}
	kind := "unknown"
	switch n.Kind {
	case yaml.DocumentNode:
		kind = "document"
	case yaml.MappingNode:
		kind = "mapping"
	case yaml.SequenceNode:
		kind = "sequence"
	case yaml.ScalarNode:
		val := n.Value
		if len(val) > 32 {
			val = val[:32] + "..."
		}
		return fmt.Sprintf("scalar(%s %q)", n.Tag, val)
	case yaml.AliasNode:
		kind = "alias"
	}
	return kind
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// assignValue 不依赖 yaml 节点树，仅对已有的值做 "${VAR}" 字符串替换，
// 供 AssignVarsFromEnv 等兼容包装和 interface{} 解码后的二次替换使用。
func assignValue(v reflect.Value, envMap map[string]string) (err error) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		assignString(v, envMap)
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if f := v.Field(i); f.CanSet() {
				if err = assignValue(f, envMap); err != nil {
					return
				}
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err = assignValue(v.Index(i), envMap); err != nil {
				return
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			ev := reflect.New(v.Type().Elem())
			ev.Elem().Set(v.MapIndex(k))
			if err = assignValue(ev, envMap); err != nil {
				return
			}
			v.SetMapIndex(k, ev.Elem())
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		tmp := reflect.New(v.Elem().Type())
		tmp.Elem().Set(v.Elem())
		if err = assignValue(tmp, envMap); err != nil {
			return
		}
		v.Set(tmp.Elem())
	}
	return
}

// assignString 值形式的环境变量替换："${VAR}" 替换为环境变量的值；
// 环境变量不存在时取 "${VAR}|default" 中 "|" 后的默认值（保持旧行为）。
func assignString(v reflect.Value, envMap map[string]string) {
	str1 := v.String()
	str, ok := AssignVarFromEnv(str1, envMap)
	if !ok {
		return
	}
	v.SetString(str)
	if str == "" {
		i := strings.IndexByte(str1, '}')
		str1 = strings.TrimSpace(str1[i+1:])
		str1 = strings.TrimLeft(str1, "|")
		str1 = strings.TrimSpace(str1)
		if str1 != "" {
			v.SetString(str1)
		}
	}
}

// setEnvValue 把环境变量的字符串值赋给字段：string 直接赋值；
// 其它类型按 yaml 解析，天然支持 int/bool/float/slice/map/struct 等所有类型。
func setEnvValue(f reflect.Value, val string) (err error) {
	for f.Kind() == reflect.Pointer {
		if f.IsNil() {
			f.Set(reflect.New(f.Type().Elem()))
		}
		f = f.Elem()
	}
	if f.Kind() == reflect.String {
		f.SetString(val)
		return nil
	}
	if err = yaml.Unmarshal([]byte(val), f.Addr().Interface()); err != nil {
		return errors.Errorf("decode env value %q: %s", val, err.Error())
	}
	return
}

// envNameFromComment 从节点的注释中提取 "${NAME}" 形式的环境变量名：
// 只要 HeadComment/LineComment/FootComment 中的某一条以 "${" 开头即生效。
// 注意 yaml.v3 的注释文本带有 "#" 前缀，需先剥掉。
func envNameFromComment(nodes ...*yaml.Node) (name string, ok bool) {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		for _, c := range []string{n.HeadComment, n.LineComment, n.FootComment} {
			c = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(c), "#"))
			if !strings.HasPrefix(c, "${") {
				continue
			}
			if i := strings.IndexByte(c, '}'); i > 2 {
				return c[2:i], true
			}
		}
	}
	return
}

// lookupEnv 依次从系统环境变量和 .env 文件的 map 中查找。
func lookupEnv(name string, envMap map[string]string) (val string, ok bool) {
	if val, ok = os.LookupEnv(name); ok {
		return
	}
	val, ok = envMap[name]
	return
}

func isNullNode(n *yaml.Node) bool {
	return n == nil || (n.Kind == yaml.ScalarNode && n.Tag == "!!null")
}

// fieldIndex 按 Camel2Case 规范化后的名字查找字段，支持驼峰/下划线/中划线/
// 大小写开头等形式；依次匹配 yaml tag、mapstructure tag、字段名，
// 内嵌匿名结构体递归查找。返回字段的索引路径（供 FieldByIndex 使用）。
func fieldIndex(t reflect.Type, key string) []int {
	nk := Camel2Case(key)
	var walk func(t reflect.Type, prefix []int) []int
	walk = func(t reflect.Type, prefix []int) []int {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue // 未导出字段
			}
			names := make([]string, 0, 3)
			skip := false
			for _, tagName := range []string{"yaml", "mapstructure"} {
				name := strings.Split(f.Tag.Get(tagName), ",")[0]
				if name == "-" {
					skip = true
					break
				}
				if name != "" {
					names = append(names, name)
				}
			}
			if skip {
				continue
			}
			names = append(names, f.Name)
			for _, name := range names {
				if Camel2Case(name) == nk {
					return append(append([]int{}, prefix...), i)
				}
			}
			// 内嵌匿名结构体递归查找
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if f.Anonymous && ft.Kind() == reflect.Struct {
				if idx := walk(ft, append(append([]int{}, prefix...), i)); idx != nil {
					return idx
				}
			}
		}
		return nil
	}
	return walk(t, nil)
}

// 从环境变量给变量赋值，变量格式为: ${Var}
func AssignVarFromEnv(v string, envMap map[string]string) (out string, ok bool) {
	v = strings.TrimSpace(v)
	i := strings.IndexByte(v, '}')
	if i < 0 || len(v) <= 3 || v[0] != '$' || v[1] != '{' {
		return
	}
	ok = true
	v = v[2:i]
	// 优先从系统环境变量查找
	out, found := os.LookupEnv(v)
	if !found {
		// 如果系统环境变量不存在，则从 .env 文件的 map 中查找
		out = envMap[v]
	}
	fmt.Printf("config from env: %s: %s\n", v, out)
	return
}

// AssignMapFromEnv 对 map 中的 "${VAR}" 字符串做环境变量替换（兼容旧接口）。
func AssignMapFromEnv(m interface{}, envMap map[string]string) (out interface{}, err error) {
	v := reflect.ValueOf(m)
	if !v.IsValid() {
		return m, nil
	}
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Map {
		return m, errors.Errorf("m must be map")
	}
	return m, assignValue(reflect.ValueOf(m), envMap)
}

// AssignSliceFromEnv 对 slice/array 中的 "${VAR}" 字符串做环境变量替换（兼容旧接口）。
func AssignSliceFromEnv(m interface{}, envMap map[string]string) (err error) {
	v := reflect.ValueOf(m)
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Array) {
		return errors.Errorf("m must be slice or array")
	}
	return assignValue(reflect.ValueOf(m), envMap)
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
