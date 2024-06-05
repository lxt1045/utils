package config

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"io/fs"
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
	Addr             string
	Host             string
	EnableTLS        bool
	ReadConcurrency  int
	WriteConcurrency int
	ReadWindow       int
	WriteWindow      int
	FlushTime        int
	Heartbeat        int
	TLS              struct {
		CACert     string
		ServerCert string
		ServerKey  string
		ClientCert string
		ClientKey  string
	}
}

type Queue struct {
	DBFile     string
	Limit      string
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

	c := &mapstructure.DecoderConfig{
		Metadata:         nil,
		Result:           conf,
		WeaklyTypedInput: true,
		// DecodeHook: mapstructure.ComposeDecodeHookFunc(
		// 	mapstructure.StringToTimeDurationHookFunc(),
		// 	mapstructure.StringToSliceHookFunc(","),
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
	err = yaml.Unmarshal(bs, conf)
	if err != nil {
		err = errors.Errorf(err.Error())
		return
	}
	return
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
