module github.com/otherjamesbrown/ai-aas/services/analytics-service/test/perf

go 1.24.0

toolchain go1.24.2

require (
	github.com/google/uuid v1.6.0
	github.com/jackc/pgx/v5 v5.7.6
	github.com/otherjamesbrown/ai-aas/services/analytics-service v0.0.0-00010101000000-000000000000
	github.com/redis/go-redis/v9 v9.16.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.30.0 // indirect
)

replace github.com/otherjamesbrown/ai-aas/services/analytics-service => ../..
