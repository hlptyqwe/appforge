package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"appforge/common/observability"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/trace"
	"github.com/zeromicro/go-zero/rest"
)

const collectorAddress = "127.0.0.1:14318"

func main() {
	conf := rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "appforge-observability-acceptance", Mode: service.ProMode},
		Host:        "127.0.0.1",
		Port:        18888,
		Middlewares: rest.MiddlewaresConf{Trace: true, Prometheus: true, Recover: true},
	}
	if err := observability.ApplyEnvironment(&conf.ServiceConf); err != nil {
		panic(err)
	}

	var exportedSpans atomic.Int64
	collectorMux := http.NewServeMux()
	collectorMux.HandleFunc("/v1/traces", func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, 4<<20))
		if err != nil || len(body) == 0 {
			http.Error(writer, "empty trace payload", http.StatusBadRequest)
			return
		}
		exportedSpans.Add(1)
		writer.WriteHeader(http.StatusOK)
	})
	collectorMux.HandleFunc("/count", func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(writer, "%d", exportedSpans.Load())
	})
	collector := &http.Server{
		Addr:              collectorAddress,
		Handler:           collectorMux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		if err := collector.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	server := rest.MustNewServer(conf)
	server.AddRoute(rest.Route{
		Method: http.MethodGet,
		Path:   "/probe",
		Handler: func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"status":"ok"}`))
		},
	})
	go server.Start()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	<-shutdown
	trace.StopAgent()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = collector.Shutdown(ctx)
}
