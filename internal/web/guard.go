package web

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// guard 是本地 HTTP 服务的一道防线。
//
// 这个服务监听 127.0.0.1 且不需要认证。本机其他进程能调它是可以接受的——它本来就
// 是个本机工具。但**浏览器**不一样：一个恶意网页可以用「简单请求」向
// http://127.0.0.1:端口/api/... 发 POST。`Content-Type: text/plain` 不触发 CORS
// 预检，浏览器会照发不误；那个网页虽然读不到响应，但服务端的副作用
// （删 Agent、打补丁、恢复备份）已经发生了。这就是 CSRF。
//
// 两道校验：
//
//  1. **Host 必须是回环地址** —— 挡 DNS rebinding。攻击者把自己的域名解析到
//     127.0.0.1，浏览器便会认为它与本工具同源、放行请求；但此时 Host 头里是
//     攻击者的域名，会被这一关拒掉。
//
//  2. **改数据的请求若带 Origin，必须与自身同源** —— 浏览器对跨源 POST 一定会
//     带 Origin；工具自己的页面发出的是同源请求，不受影响。不带 Origin 的请求
//     （curl、脚本）放行，因为那不是 CSRF 的形态。
//
// 只读请求不做 Origin 校验：浏览器的同源 GET 本来就不带 Origin，强求会误伤自己。
// 读接口泄露不了什么——跨源脚本读不到响应，而 DNS rebinding 已被第一关拦住。
func guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackHost(r.Host) {
			http.Error(w, "forbidden: non-loopback host", http.StatusForbidden)
			return
		}
		if mutates(r.Method) {
			if origin := r.Header.Get("Origin"); origin != "" && !sameOrigin(origin, r.Host) {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// mutates 判断该方法是否会改变状态。
func mutates(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// isLoopbackHost 判断 Host 头指向的是不是本机。
func isLoopbackHost(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]") // IPv6 字面量写作 [::1]
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true // 覆盖 127.0.0.0/8 与 ::1
	}
	return false
}

// sameOrigin 判断 Origin 头是否与当前请求同源。
func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// "null"（沙箱 iframe、file:// 页面）解析出来的 Host 是空的，自然不匹配。
	return u.Scheme == "http" && u.Host == host
}
