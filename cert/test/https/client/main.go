package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"
)

func init() {
	log.SetFlags(log.Flags() | log.Lmicroseconds | log.Lshortfile) //log.Llongfile
}

func main() {
	dir, _ := filepath.Abs("../ca")
	log.Println("root", dir)

	// 加载CA证书(生产环境不需要这一步)
	caCert, err := os.ReadFile(filepath.Join(dir, "root-cert.pem"))
	if err != nil {
		log.Println("try to load ca err", err)
		return
	}

	cliCert, err := tls.LoadX509KeyPair(filepath.Join(dir, "client-cert.pem"), filepath.Join(dir, "client-key.pem"))
	if err != nil {
		panic("try to load key & key err, " + err.Error())
	}

	host, addr := "lxt1045.com", "127.0.0.1"
	c := New(host, addr, caCert, cliCert)

	c.HelloWorld()
}

type Client struct {
	http.Client
}

func (c *Client) HelloWorld() {
	if c == nil {
		return
	}
	request, _ := http.NewRequest("GET", "https://lxt1045.com:443/hello", nil)
	response, err := c.Do(request)
	if err != nil {
		log.Println("DO err", err)
		return
	}

	data, _ := io.ReadAll(response.Body)
	log.Println("data", string(data))
}

func New(host, addr string, caCert []byte, cliCert tls.Certificate) (c *Client) {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	config := &tls.Config{
		Certificates:       []tls.Certificate{cliCert}, // 客户端证书, 双向认证必须携带
		RootCAs:            caCertPool,                 // 校验服务端证书 [CA证书]
		InsecureSkipVerify: false,                      // 不用校验服务器证书
	}

	c = &Client{
		Client: http.Client{
			Transport: &http.Transport{
				TLSClientConfig: config,
				DialContext: NewDialContext("", map[string][]string{
					host: {addr},
				}),
				TLSHandshakeTimeout: 10 * time.Second,
			},
			Timeout: 10 * time.Second,
		},
	}
	return
}

func NewDialContext(nameserver string, names map[string][]string) (
	fDial func(ctx context.Context, network, address string) (net.Conn, error)) {

	dialer := &net.Dialer{
		Timeout: 1 * time.Millisecond * 100,
	}

	var resolver *net.Resolver
	if nameserver != "" {
		resolver = &net.Resolver{
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp", nameserver)
			},
		}
	}
	fDial = func(ctx context.Context, network, address string) (conn net.Conn, err error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		ips := names[host]
		if len(ips) > 0 {
			ip := ips[0]
			if len(ips) > 1 {
				i := rand.Intn(len(ips))
				ip = ips[i]
			}
			conn, err = dialer.DialContext(ctx, network, ip+":"+port)
			if err == nil {
				return
			}
		}

		if resolver != nil {
			ips, err = resolver.LookupHost(ctx, host) // 通过自定义nameserver查询域名
			for _, ip := range ips {
				conn, err := dialer.DialContext(ctx, network, ip+":"+port) // 创建链接
				if err == nil {
					return conn, nil
				}
			}
		}
		return dialer.DialContext(ctx, network, address)
	}
	return
}

/*
// --cert, --key 是客户端的证书
// --cacert 是服务器的CA证书

curl -X GET \
	 --cert certs/client/client.crt --key certs/client/client.key.text \
	 --cacert certs/ca/ca.crt \
	 'https://localhost:1443/'

*   Trying 127.0.0.1...
* Connected to localhost (127.0.0.1) port 443 (#0)
* found 1 certificates in certs/ca/ca.crt
* found 597 certificates in /etc/ssl/certs
* ALPN, offering http/1.1
* SSL connection using TLS1.2 / ECDHE_RSA_AES_128_GCM_SHA256
* 	 server certificate verification OK
* 	 server certificate status verification SKIPPED
* 	 common name: localhost (matched)
* 	 server certificate expiration date OK
* 	 server certificate activation date OK
* 	 certificate public key: RSA
* 	 certificate version: #1
* 	 subject: C=CN,ST=ZheJiang,L=Hangzhou,O=Broadlink,OU=Broadlink,CN=localhost
* 	 start date: Wed, 01 Jul 2020 07:03:40 GMT
* 	 expire date: Thu, 01 Jul 2021 07:03:40 GMT
* 	 issuer: C=CN,ST=ZheJiang,L=Hangzhou,O=Broadlink,OU=Broadlink,CN=master
* 	 compression: NULL
* ALPN, server accepted to use http/1.1
> GET / HTTP/1.1
> Host: localhost
> User-Agent: curl/7.47.0
>
< HTTP/1.1 200 OK
< Date: Wed, 01 Jul 2020 08:41:20 GMT
< Content-Length: 12
< Content-Type: text/plain; charset=utf-8
<
* Connection #0 to host localhost left intact
{"status":0}

**/
