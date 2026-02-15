package validator_test

import (
	"context"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/sklrsn/monitor-ingress/internal/validator"
)

func TestDo(t *testing.T) {
	testCases := []struct {
		name          string
		targetAddress string
		trustAnchors  []byte
		tlsInsecure   bool
		timeout       time.Duration
		expectedCode  int
	}{
		{
			name:          "google",
			targetAddress: "https://www.google.com",
			trustAnchors:  make([]byte, 0),
			tlsInsecure:   false,
			timeout:       3 * time.Second,
			expectedCode:  http.StatusOK,
		},
		{
			name:          "httpbin.org with one redirect",
			targetAddress: "http://httpbin.org/redirect/1",
			trustAnchors:  make([]byte, 0),
			tlsInsecure:   false,
			timeout:       3 * time.Second,
			expectedCode:  http.StatusFound,
		},
		{
			name:          "httpbin.org with three redirects",
			targetAddress: "http://httpbin.org/redirect/3",
			trustAnchors:  make([]byte, 0),
			tlsInsecure:   false,
			timeout:       3 * time.Second,
			expectedCode:  http.StatusFound,
		},

		{
			name:          "example.com/status",
			targetAddress: "http://127.0.0.1:8080/status",
			expectedCode:  http.StatusOK,
		},
	}

	srv := SetupTestServer(t.Context())

	for _, tc := range testCases {
		ingressValidator, err := validator.NewIngressValidator(
			t.Context(),
			validator.WithTrustAnchors(tc.trustAnchors),
			validator.WithTimeout(30*time.Second))
		if err != nil {
			t.Fatalf("failed to create a ingress validator:%v", err)
		}
		t.Run(tc.name, func(t *testing.T) {
			resp := ingressValidator.Do(t.Context(), tc.targetAddress)
			t.Logf("%+v", resp)
			if resp.StatusCode != tc.expectedCode {
				t.Fail()
				t.Errorf("targetAddress=%v,expected=%v actual:%v", tc.targetAddress,
					tc.expectedCode, resp.StatusCode)
			}
		})
	}
	_ = srv.Shutdown(t.Context())
}

func SetupTestServer(ctx context.Context) *http.Server {
	h := http.NewServeMux()
	h.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/status", http.StatusMovedPermanently)
	})

	srv := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: h,
	}
	go func() {
		log.Fatalf("%v", srv.ListenAndServe())
	}()

	return srv
}
