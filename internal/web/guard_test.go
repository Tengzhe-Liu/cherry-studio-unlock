package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newGuarded() http.Handler {
	return guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("passed"))
	}))
}

func TestGuard(t *testing.T) {
	const host = "127.0.0.1:8765"

	cases := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		// ---- 正常使用：工具自己的页面 ----
		{"本机 GET", "GET", host, "", 200},
		{"本机 POST（同源页面发起）", "POST", host, "http://" + host, 200},
		{"本机 DELETE（同源页面发起）", "DELETE", host, "http://" + host, 200},
		{"用 localhost 访问", "POST", "localhost:8765", "http://localhost:8765", 200},
		{"IPv6 回环", "POST", "[::1]:8765", "http://[::1]:8765", 200},

		// ---- 不带 Origin 的请求：curl / 脚本 / 本机进程 ----
		// 这不是 CSRF 的形态（浏览器发跨源 POST 一定会带 Origin），放行。
		{"不带 Origin 的 POST", "POST", host, "", 200},

		// ---- 攻击场景一：恶意网页直接打本地端口（CSRF）----
		// Content-Type: text/plain 不触发 CORS 预检，浏览器会真的把请求发出来。
		// 必须靠 Origin 校验拦下。
		{"跨源 POST —— 恶意网页删 Agent", "POST", host, "https://evil.example", 403},
		{"跨源 POST —— 恶意网页打补丁", "POST", host, "http://evil.example", 403},
		{"跨源 PUT", "PUT", host, "https://evil.example", 403},
		{"跨源 DELETE", "DELETE", host, "https://evil.example", 403},
		{"Origin 为 null（沙箱 iframe / file://）", "POST", host, "null", 403},
		// 端口不同也算跨源
		{"同主机不同端口", "POST", host, "http://127.0.0.1:9999", 403},

		// ---- 攻击场景二：DNS rebinding ----
		// 攻击者把自己的域名解析到 127.0.0.1，浏览器会认为它与工具同源，
		// 连 Origin 都是"同源"的。但 Host 头里是攻击者的域名。
		{"DNS rebinding —— Host 是外部域名", "POST", "evil.example", "http://evil.example", 403},
		{"DNS rebinding —— 即使是只读请求也拦", "GET", "evil.example", "", 403},
		{"Host 为内网地址", "GET", "192.168.1.10:8765", "", 403},
		{"Host 为空", "GET", "", "", 403},

		// ---- 只读请求不校验 Origin（同源 GET 本就不带 Origin）----
		{"跨源 GET 放行（读不到响应，且敏感信息不在此）", "GET", host, "https://evil.example", 200},
	}

	h := newGuarded()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "http://"+host+"/api/agents", nil)
			req.Host = c.host
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Errorf("%s %s  Host=%q Origin=%q  得到 %d，期望 %d",
					c.method, req.URL.Path, c.host, c.origin, rec.Code, c.want)
			}
		})
	}
}

// TestIsLoopbackHost 覆盖各种 Host 写法。
func TestIsLoopbackHost(t *testing.T) {
	ok := []string{"127.0.0.1:8765", "127.0.0.1", "localhost:8765", "localhost", "[::1]:8765", "127.5.5.5:1"}
	no := []string{"evil.example", "evil.example:80", "192.168.1.10:8765", "", "10.0.0.1", "[2001:db8::1]:80"}
	for _, h := range ok {
		if !isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) 应为 true", h)
		}
	}
	for _, h := range no {
		if isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) 应为 false", h)
		}
	}
}

// TestSameOrigin 覆盖 Origin 解析的边界。
func TestSameOrigin(t *testing.T) {
	const host = "127.0.0.1:8765"
	yes := []string{"http://127.0.0.1:8765"}
	no := []string{
		"https://127.0.0.1:8765", // 协议不同
		"http://127.0.0.1:9999",  // 端口不同
		"http://evil.example",    // 主机不同
		"null",                   // 沙箱 / file://
		"",                       // 空
		"://坏掉的 url",
	}
	for _, o := range yes {
		if !sameOrigin(o, host) {
			t.Errorf("sameOrigin(%q, %q) 应为 true", o, host)
		}
	}
	for _, o := range no {
		if sameOrigin(o, host) {
			t.Errorf("sameOrigin(%q, %q) 应为 false", o, host)
		}
	}
}
