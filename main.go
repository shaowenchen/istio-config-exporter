package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "istio_config"
)

var (
	listenAddress  = flag.String("web.listen-address", ":9102", "Address on which to expose metrics and web interface.")
	metricsPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	kubeconfigPath = flag.String("kubeconfig", "", "Path to kubeconfig file (default: in-cluster or $HOME/.kube/config).")
	namespacesFlag = flag.String("namespaces", "", "Comma-separated namespaces to scrape (default: all).")
)

func main() {
	flag.Parse()

	var namespacesList []string
	if *namespacesFlag != "" {
		for _, s := range strings.Split(*namespacesFlag, ",") {
			if t := strings.TrimSpace(s); t != "" {
				namespacesList = append(namespacesList, t)
			}
		}
	}

	collector, err := NewIstioConfigCollector(*kubeconfigPath, namespacesList)
	if err != nil {
		log.Fatalf("Failed to create Istio config collector: %v", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	reg.MustRegister(prometheus.NewGoCollector())

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Istio Config Exporter</title></head>
			<body>
			<h1>Istio Config Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/live", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := &http.Server{
		Addr:              *listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
		<-sig
		log.Print("Shutting down...")
		collector.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown: %v", err)
		}
	}()

	log.Printf("Starting Istio config exporter on %s", *listenAddress)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
