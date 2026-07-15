GOBIN := $(shell go env GOPATH)/bin

.PHONY: test vet race parity security check report tools help

help: ## list targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*## "}{printf "  %-12s %s\n",$$1,$$2}'

test: ## run the hermetic test suite
	go test ./...

vet: ## go vet
	go vet ./...

race: ## race detector on engine/optimize/report
	go test -race ./...

bench: ## run benchmarks (bar loop, stats, optimize)
	go test -bench=. -benchmem -run=^$$ ./

parity: ## parity tests vs backtesting.py (needs .venv: see scripts/)
	go test -tags parity -run 'TestParityMatrix|TestOptimizeParity' ./...

tools: ## install local security scanners
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/google/osv-scanner/cmd/osv-scanner@latest

security: ## local CVE + static + dependency scan
	@echo "== govulncheck (reachable CVEs) =="; $(GOBIN)/govulncheck ./...
	@echo "== gosec (static security) =="; $(GOBIN)/gosec -quiet ./...
	@echo "== osv-scanner (dependency vulns) =="; $(GOBIN)/osv-scanner --lockfile go.mod

check: vet test security ## the full local gate

report: ## generate the demo HTML report
	go run ./cmd/genreport
