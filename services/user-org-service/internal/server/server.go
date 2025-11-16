package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Options configure the HTTP server instance.
type Options struct {
	Port           int
	Logger         *zap.Logger
	ServiceName    string
	Readiness      func(context.Context) error
	RegisterRoutes func(chi.Router)
}

// New constructs an http.Server pre-configured with health and readiness routes.
func New(opts Options) *http.Server {
	if opts.Readiness == nil {
		opts.Readiness = func(context.Context) error { return nil }
	}

	router := chi.NewRouter()

	// Helper function to check if origin is allowed
	isAllowedOrigin := func(origin string) bool {
		if origin == "" {
			return false
		}
		return origin == "http://localhost:5173" || origin == "https://localhost:5173" ||
			(len(origin) >= 17 && origin[:17] == "http://localhost:") ||
			(len(origin) >= 18 && origin[:18] == "https://localhost:")
	}
	
	// CORS middleware for local development - must be first to handle OPTIONS
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Handle preflight OPTIONS requests - intercept before route matching
			if r.Method == "OPTIONS" {
				if isAllowedOrigin(origin) {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, X-Correlation-ID, X-API-Key")
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Access-Control-Max-Age", "3600")
					
					opts.Logger.Debug("CORS preflight request handled",
						zap.String("method", r.Method),
						zap.String("path", r.URL.Path),
						zap.String("origin", origin))
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// For actual requests, add CORS headers
			if isAllowedOrigin(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			next.ServeHTTP(w, r)
		})
	})

	// Helper function to add CORS headers (reuses isAllowedOrigin for consistency)
	addCORSHeaders := func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-CSRF-Token, X-Correlation-ID, X-API-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}
	}
	
	// Set MethodNotAllowed handler to handle OPTIONS and add CORS to error responses
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w, r)
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		opts.Logger.Warn("method not allowed",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("request_id", middleware.GetReqID(r.Context())))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed","method":"` + r.Method + `","path":"` + r.URL.Path + `"}`))
	})
	
	// Set NotFound handler to add CORS headers and log missing routes
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		addCORSHeaders(w, r)
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		
		opts.Logger.Warn("route not found",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("request_id", middleware.GetReqID(r.Context())))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"route not found","method":"` + r.Method + `","path":"` + r.URL.Path + `"}`))
	})

	// Request logging and recovery middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	
	// Comprehensive request/response logging middleware for debugging
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := middleware.GetReqID(r.Context())
			
			// Log incoming request (key headers only to avoid verbosity)
			fields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("request_id", requestID),
				zap.String("origin", r.Header.Get("Origin")),
			}
			
			// Only log user agent if present
			if ua := r.Header.Get("User-Agent"); ua != "" {
				fields = append(fields, zap.String("user_agent", ua))
			}
			
			// Log key headers for debugging
			if auth := r.Header.Get("Authorization"); auth != "" {
				fields = append(fields, zap.String("has_auth", "true"))
			}
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				fields = append(fields, zap.String("has_api_key", "true"))
			}
			
			opts.Logger.Info("incoming request", fields...)
			
			// Wrap response writer to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			
			next.ServeHTTP(ww, r)
			
			// Log response
			duration := time.Since(start)
			responseFields := []zap.Field{
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.statusCode),
				zap.Duration("duration_ms", duration),
				zap.String("request_id", requestID),
			}
			
			// Log CORS headers if present
			if corsOrigin := w.Header().Get("Access-Control-Allow-Origin"); corsOrigin != "" {
				responseFields = append(responseFields, zap.String("cors_origin", corsOrigin))
			}
			
			opts.Logger.Info("request completed", responseFields...)
		})
	})

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := opts.Readiness(ctx); err != nil {
			opts.Logger.Warn("readiness check failed", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// Prometheus metrics endpoint
	router.Get("/metrics", promhttp.Handler().ServeHTTP)
	
	// Debug endpoint to list registered routes (development only)
	router.Get("/debug/routes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		routes := []map[string]string{}
		
		walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			routes = append(routes, map[string]string{
				"method": method,
				"route":  route,
			})
			return nil
		}
		
		if err := chi.Walk(router, walkFunc); err != nil {
			opts.Logger.Error("failed to walk routes", zap.Error(err))
			http.Error(w, "failed to list routes", http.StatusInternalServerError)
			return
		}
		
		response := map[string]interface{}{
			"routes": routes,
			"count":  len(routes),
		}
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	if opts.RegisterRoutes != nil {
		opts.RegisterRoutes(router)
	}

	addr := fmt.Sprintf(":%d", opts.Port)
	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}
