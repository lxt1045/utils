package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"golang.org/x/net/http2"
)

func init() {
	log.SetFlags(log.Flags() | log.Lmicroseconds | log.Lshortfile) //log.Llongfile
}

func main() {

	dir, _ := filepath.Abs("../ca")
	caCert, err := os.ReadFile(filepath.Join(dir, "root-cert.pem")) // 加载CA, 添加进 caCertPool
	if err != nil {
		log.Println("try to load ca err", err)
		return
	}
	// 加载服务端证书(生产环境当中, 证书是第三方进行签名的, 而非自定义CA)
	srvCert, err := tls.LoadX509KeyPair(filepath.Join(dir, "server-cert.pem"), filepath.Join(dir, "server-key.pem"))
	if err != nil {
		log.Println("try to load key & crt err", err)
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", handle)
	mux.Handle("/", http.FileServer(http.Dir("./")))

	Server(mux, caCert, srvCert)

}

func handle(w http.ResponseWriter, req *http.Request) {
	log.Println("hello, world!")
	io.WriteString(w, "hello, world!\n")
}

func Server(mux *http.ServeMux, caCert []byte, srvCert tls.Certificate) {

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	config := &tls.Config{
		Certificates:       []tls.Certificate{srvCert},     // 服务器证书
		ClientCAs:          caCertPool,                     // 专门校验客户端的证书 [CA证书]
		InsecureSkipVerify: false,                          // 必须校验
		ClientAuth:         tls.RequireAndVerifyClientCert, // 校验客户端证书

		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}

	server := &http.Server{
		Addr:      ":443",
		Handler:   mux,
		TLSConfig: config,
		ErrorLog:  log.New(os.Stdout, "", log.Lshortfile|log.Ldate|log.Ltime),
	}

	http2.ConfigureServer(server, &http2.Server{})

	err := server.ListenAndServeTLS("", "")
	if err != nil {
		log.Println("ListenAndServeTLS err", err)
		return
	}

	return
}
