package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type Config struct {
	ListenPort string            `json:"listen_port"`
	TargetURL  string            `json:"target_url"`
	ApiKeys    map[string]string `json:"api_keys"`
}

func main() {
	log.Printf("OllamaProxy https://github.com/xiaoyaoking/OllamaProxy ")
	confData, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatalf("无法读取配置文件: %v", err)
	}
	var cfg Config
	json.Unmarshal(confData, &cfg)

	target, _ := url.Parse(cfg.TargetURL)

	// 使用更严谨的代理构造方式
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			// 关键：不要重复拼接路径，直接替换
			req.URL.Path = target.Path + req.URL.Path
			req.Host = target.Host
			
			// 鉴权通过后必须删除此 Header，否则 Ollama 可能返回 403
			req.Header.Del("Authorization")
		},
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 1. 优先处理跨域请求，否则 VS Code 插件会拦截
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 2. 鉴权逻辑
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		key := strings.TrimPrefix(authHeader, "Bearer ")
		clientName, exists := cfg.ApiKeys[key]
		if !exists {
			log.Printf("拒绝非法访问 | IP: %s", r.RemoteAddr)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Printf("授权通过 | 客户端: %-15s | 路径: %s", clientName, r.URL.Path)
		
		// 3. 执行转发 (ServeHTTP 会自动处理响应头，不要再手动写 w.WriteHeader)
		proxy.ServeHTTP(w, r)
	})

	log.Printf("Ollama 网关已启动: %s -> %s", cfg.ListenPort, cfg.TargetURL)
	log.Fatal(http.ListenAndServe(cfg.ListenPort, nil))
}