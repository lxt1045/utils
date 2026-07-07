package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"
	"time"
)

/*
下面用 Golang 写一个例子，演示三个场景：
	TCP 同时打开成功建立连接（两端直接通信）
	在同时打开的连接上两端都当 TLS 客户端 → 握手失败
	在同时打开的连接上正确分配 TLS 角色 → 握手成功
*/

func main() {
	// 自签名证书，方便测试（实际使用应妥善管理）
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		panic(err)
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM([]byte(certPEM)); !ok {
		panic("failed to append cert")
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      certPool,
		// 跳过主机名验证，因为使用 IP 连接
		InsecureSkipVerify: true,
	}

	// 用于同步两端同时发起连接
	var wg sync.WaitGroup
	wg.Add(2)

	// ---- 场景1：纯 TCP 同时打开 ----
	fmt.Println("=== 场景1: TCP 同时打开 ===")
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10001", "127.0.0.1:10002")
		if err != nil {
			fmt.Println("A dial error:", err)
			return
		}
		defer conn.Close()
		// 发送数据验证连接
		conn.Write([]byte("Hello from A\n"))
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		fmt.Printf("A received: %s", buf[:n])
	}()
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10002", "127.0.0.1:10001")
		if err != nil {
			fmt.Println("B dial error:", err)
			return
		}
		defer conn.Close()
		buf := make([]byte, 1024)
		n, _ := conn.Read(buf)
		fmt.Printf("B received: %s", buf[:n])
		conn.Write([]byte("Hello from B\n"))
	}()
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// ---- 场景2：同时打开后，双方都当 TLS 客户端（错误） ----
	fmt.Println("\n=== 场景2: 双方都发起 TLS 握手（客户端模式） ===")
	wg.Add(2)
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10003", "127.0.0.1:10004")
		if err != nil {
			fmt.Println("A dial error:", err)
			return
		}
		defer conn.Close()
		// 双方都作为 TLS 客户端 -> 都会发送 ClientHello
		tlsConn := tls.Client(conn, config)
		err = tlsConn.Handshake()
		if err != nil {
			fmt.Println("A TLS handshake error (expected):", err)
		} else {
			fmt.Println("A TLS handshake succeeded (unexpected)")
		}
	}()
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10004", "127.0.0.1:10003")
		if err != nil {
			fmt.Println("B dial error:", err)
			return
		}
		defer conn.Close()
		tlsConn := tls.Client(conn, config)
		err = tlsConn.Handshake()
		if err != nil {
			fmt.Println("B TLS handshake error (expected):", err)
		} else {
			fmt.Println("B TLS handshake succeeded (unexpected)")
		}
	}()
	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	// ---- 场景3：同时打开后，一端为 TLS 服务器，一端为客户端（正确） ----
	fmt.Println("\n=== 场景3: 正确分配角色，TLS 握手成功 ===")
	wg.Add(2)
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10005", "127.0.0.1:10006")
		if err != nil {
			fmt.Println("A dial error:", err)
			return
		}
		defer conn.Close()
		// A 作为 TLS 客户端
		tlsConn := tls.Client(conn, config)
		err = tlsConn.Handshake()
		if err != nil {
			fmt.Println("A TLS client error:", err)
			return
		}
		tlsConn.Write([]byte("Secure hello from A\n"))
		buf := make([]byte, 1024)
		n, _ := tlsConn.Read(buf)
		fmt.Printf("A received: %s", buf[:n])
	}()
	go func() {
		defer wg.Done()
		conn, err := simultaneousDial("127.0.0.1:10006", "127.0.0.1:10005")
		if err != nil {
			fmt.Println("B dial error:", err)
			return
		}
		defer conn.Close()
		// B 作为 TLS 服务器
		tlsConn := tls.Server(conn, config)
		err = tlsConn.Handshake()
		if err != nil {
			fmt.Println("B TLS server error:", err)
			return
		}
		buf := make([]byte, 1024)
		n, _ := tlsConn.Read(buf)
		fmt.Printf("B received: %s", buf[:n])
		tlsConn.Write([]byte("Secure hello from B\n"))
	}()
	wg.Wait()
}

// simultaneousDial 模拟 TCP 同时打开：绑定本地地址并连接远程地址，
// 对方也进行同样的操作，双方的 SYN 会在网络上交叉。
func simultaneousDial(localAddr, remoteAddr string) (net.Conn, error) {
	// 解析地址
	laddr, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	raddr, err := net.ResolveTCPAddr("tcp", remoteAddr)
	if err != nil {
		return nil, err
	}
	// 创建一个 TCP 连接器，并指定本地地址（即我们想要绑定的端口）
	d := net.Dialer{
		LocalAddr: laddr,
		Timeout:   5 * time.Second,
	}
	// 当对端也同时 Dial 时，内核会处理同时打开。
	return d.Dial("tcp", raddr.String())
}

// 用于测试的自签名证书和密钥（PEM 格式）
const certPEM = `-----BEGIN CERTIFICATE-----
MIIDBjCCAe6gAwIBAgIUL8O0K0JqGJmZtH6cF7o3W1Kp+6cwDQYJKoZIhvcNAQEL
BQAwFDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI0MDEwMTAwMDAwMFoXDTI1MDEw
MTAwMDAwMFowFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEA4uGv7k6QI0a7Q+2k0nVHbpVk0/7kxPqIOgYiWQF6hQ4c
TnFZNqKnTqQ7Y8QZ2HxZBZiBRSRvH9QGsqBtSxG1hAaZDj5C5QGmF6Y2w2m5FrY
G+NJKm8vBt6vnTJ2gkUH4W+GX6CVB7gbfJ0MWcD/J/LZR0kQ9mWb4G5xX6b4W7w
IDAQABo4GDMIGAMAkGA1UdEwQCMAAwCwYDVR0PBAQDAgXgMB0GA1UdDgQWBBRl
0iFfSgLq8wU4WtK3qPuVvjvZ+zAfBgNVHSMEGDAWgBRl0iFfSgLq8wU4WtK3qP
uVvjvZ+zAPBgNVHREECDAGgglsb2NhbGhvc3QwDQYJKoZIhvcNAQELBQADggEB
AIb0V7sG7A6pLGkYFZL+4XUZI8R1OyU5kQ/XKv3L2f6Hc8l5i4+2aD4kN8rI5o
w2rN6u9Qd1fW3v4o6dL4u7+5Q8r9z0fHc8l5i4+2aD4kN8rI5ow2rN6u9Qd1f
W3v4o6dL4u7+5Q8r9z0fHc8l5i4+2aD4kN8rI5ow2rN6u9Qd1fW3v4o6dL4u7
+5Q8r9z0fHc8l5i4+2aD4kN8rI5ow2rN6u9Qd1fW3v4o6dL4u7+5Q8r9z0fHc8
l5i4+2aD4kN8rI5ow2rN6u9Qd1fW3v4o6dL4u7+5Q8r9z0fHc8l5i4+2aD4k=
-----END CERTIFICATE-----`

const keyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA4uGv7k6QI0a7Q+2k0nVHbpVk0/7kxPqIOgYiWQF6hQ4cTnF
ZNqKnTqQ7Y8QZ2HxZBZiBRSRvH9QGsqBtSxG1hAaZDj5C5QGmF6Y2w2m5FrYG+N
JKm8vBt6vnTJ2gkUH4W+GX6CVB7gbfJ0MWcD/J/LZR0kQ9mWb4G5xX6b4W7wIDAQ
ABAoIBAQDS1nD7h3fRkS4B+6eQmJpT8hLc1VxK4jX5k2m+6G+6Q5t2nLbK8aTy
...
-----END RSA PRIVATE KEY-----`
