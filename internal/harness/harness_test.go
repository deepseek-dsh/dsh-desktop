package harness

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestServicePresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	port := portOf(t, srv.URL)
	if !servicePresent(port) {
		t.Fatalf("有服务在跑时 servicePresent 应返回 true, 端口=%d", port)
	}

	srv.Close()
	if servicePresent(port) {
		t.Fatalf("服务已关闭后 servicePresent 应返回 false, 端口=%d", port)
	}
}

func TestWaitReadyReturnsWhenServiceUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := &Harness{port: portOf(t, srv.URL)}
	done := make(chan struct{})
	var waitErr error
	if err := h.waitReady(done, &waitErr); err != nil {
		t.Fatalf("服务已在就绪时 waitReady 应返回 nil, 得到: %v", err)
	}
}

func TestWaitReadyDetectsExit(t *testing.T) {
	// 无服务 + 子进程退出 → waitReady 应报错。
	h := &Harness{port: closedPort(t)}
	done := make(chan struct{})
	close(done) // 模拟子进程意外退出
	var waitErr error
	if err := h.waitReady(done, &waitErr); err == nil {
		t.Fatalf("子进程退出且无服务时 waitReady 应返回错误")
	}
}

// portOf 从 httptest server URL 解析端口号。
func portOf(t *testing.T, url string) int {
	t.Helper()
	addr := url[len("http://127.0.0.1:"):]
	n, err := strconv.Atoi(addr)
	if err != nil {
		t.Fatalf("解析端口失败: url=%s", url)
	}
	return n
}

// closedPort 返回一个当前空闲的端口(用于模拟无服务场景)。
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取空闲端口失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}
