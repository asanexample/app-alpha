package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3c is built once at startup. The AWS SDK default credential chain resolves EKS Pod Identity
// credentials automatically via the container-credentials provider (AWS_CONTAINER_CREDENTIALS_FULL_URI
// + the projected token the pod-identity webhook injects) — no static keys, no SA annotation.
var s3c *s3.Client

func initAWS(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}
	s3c = s3.NewFromConfig(cfg)
	return nil
}

func main() {
	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "unknown"
	}

	// Log-and-continue: a failed AWS init must not take down "/" or "/healthz" (probes stay green).
	if err := initAWS(context.Background()); err != nil {
		log.Printf("warning: AWS init failed; /data will be unavailable: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"app":       "app-alpha",
			"version":   version,
			"namespace": namespace,
			"hostname":  r.Host,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Reads an object from the team's S3 bucket using the pod's EKS Pod Identity credentials.
	// Demonstrates per-team isolation: ?bucket= defaults to this team's bucket (DATA_BUCKET) and a
	// successful read proves access; pointing ?bucket= at ANOTHER team's bucket returns AccessDenied
	// (the cross-team negative test).
	mux.HandleFunc("GET /data", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		bucket := r.URL.Query().Get("bucket")
		if bucket == "" {
			bucket = os.Getenv("DATA_BUCKET")
		}
		key := r.URL.Query().Get("key")
		if key == "" {
			key = "hello.txt"
		}
		if s3c == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"error": "s3 client not initialized"})
			return
		}
		out, err := s3c.GetObject(r.Context(), &s3.GetObjectInput{Bucket: &bucket, Key: &key})
		if err != nil {
			// Surface the error verbatim (e.g. AccessDenied) — this is what the negative test reads.
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"bucket": bucket, "key": key, "error": err.Error()})
			return
		}
		defer out.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(out.Body, 1<<20))
		json.NewEncoder(w).Encode(map[string]any{
			"bucket": bucket, "key": key, "bytes": len(body), "content": string(body),
		})
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("starting app-alpha version=%s namespace=%s", version, namespace)
	log.Fatal(srv.ListenAndServe())
}
