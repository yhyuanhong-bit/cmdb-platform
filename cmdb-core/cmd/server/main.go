package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/cmdb-platform/cmdb-core/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("starting cmdb-core on %s (deploy_mode=%s)", addr, cfg.DeployMode)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
