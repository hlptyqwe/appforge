package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18088", "fixture listen address")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/system/auth/login", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(writer, map[string]any{"code": 200, "msg": "OK", "data": map[string]string{"token": "synthetic-soak-token"}})
	})
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("/admin/core/enterprise/deployment", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer synthetic-soak-token" {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		components := []map[string]string{
			{"name": "system-rpc", "status": "healthy"},
			{"name": "core-rpc", "status": "healthy"},
			{"name": "builder-rpc", "status": "healthy"},
		}
		writeJSON(writer, map[string]any{"code": 200, "msg": "OK", "data": map[string]any{"components": components}})
	})
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func writeJSON(writer http.ResponseWriter, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(payload)
}
