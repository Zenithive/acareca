.PHONY: help install smoke load spike stress soak all clean results

# Configuration
BASE_URL ?= http://localhost:8080
RESULTS_DIR = results/$(shell date +%Y%m%d_%H%M%S)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

install: ## Install K6 (macOS only)
	@echo "Installing K6..."
	@brew install k6 || echo "Please install K6 manually: https://k6.io/docs/getting-started/installation/"

check: ## Check if K6 is installed and API is running
	@echo "Checking K6 installation..."
	@k6 version || (echo "K6 is not installed" && exit 1)
	@echo "Checking API availability at $(BASE_URL)..."
	@curl -s -f $(BASE_URL)/health > /dev/null || (echo "API is not accessible at $(BASE_URL)" && exit 1)
	@echo "✓ All checks passed"

smoke: check ## Run smoke test (1 min, 1 user)
	@echo "Running smoke test..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-smoke-test.js \
		-e BASE_URL=$(BASE_URL) \
		--out json=$(RESULTS_DIR)/smoke-results.json \
		--summary-export=$(RESULTS_DIR)/smoke-summary.json

load: check ## Run load test (5 min, 10-100 users)
	@echo "Running load test..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-load-test.js \
		-e BASE_URL=$(BASE_URL) \
		--out json=$(RESULTS_DIR)/load-results.json \
		--summary-export=$(RESULTS_DIR)/load-summary.json

spike: check ## Run spike test (3 min, sudden spike to 200 users)
	@echo "Running spike test..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-spike-test.js \
		-e BASE_URL=$(BASE_URL) \
		--out json=$(RESULTS_DIR)/spike-results.json \
		--summary-export=$(RESULTS_DIR)/spike-summary.json

stress: check ## Run stress test (13 min, 50-400 users)
	@echo "Running stress test..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-stress-test.js \
		-e BASE_URL=$(BASE_URL) \
		--out json=$(RESULTS_DIR)/stress-results.json \
		--summary-export=$(RESULTS_DIR)/stress-summary.json

soak: check ## Run soak test (40 min, 50 users sustained)
	@echo "Running soak test (this will take ~40 minutes)..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-soak-test.js \
		-e BASE_URL=$(BASE_URL) \
		--out json=$(RESULTS_DIR)/soak-results.json \
		--summary-export=$(RESULTS_DIR)/soak-summary.json

all: smoke load spike ## Run all quick tests (smoke, load, spike)
	@echo "All quick tests completed!"

full: smoke load spike stress ## Run all tests except soak
	@echo "All tests completed!"

complete: smoke load spike stress soak ## Run all tests including soak
	@echo "Complete test suite finished!"

clean: ## Clean up results directory
	@echo "Cleaning up results..."
	@rm -rf results/
	@echo "✓ Results cleaned"

results: ## Show latest test results
	@echo "Latest test results:"
	@ls -lt results/ | head -n 10

# Environment-specific targets
local: ## Run smoke test against local environment
	@$(MAKE) smoke BASE_URL=http://localhost:8080

dev: ## Run smoke test against dev environment
	@$(MAKE) smoke BASE_URL=https://acareca-bam8.onrender.com

# Custom targets
custom: ## Run load test with custom parameters (use: make custom BASE_URL=... VUS=... DURATION=...)
	@echo "Running custom load test..."
	@mkdir -p $(RESULTS_DIR)
	@k6 run k6-load-test.js \
		-e BASE_URL=$(BASE_URL) \
		$(if $(VUS),--vus $(VUS),) \
		$(if $(DURATION),--duration $(DURATION),) \
		--out json=$(RESULTS_DIR)/custom-results.json \
		--summary-export=$(RESULTS_DIR)/custom-summary.json

# Quick test with minimal output
quick: ## Quick smoke test with minimal output
	@k6 run k6-smoke-test.js -e BASE_URL=$(BASE_URL) --quiet

# Test with specific number of users
test-users: ## Test with specific number of users (use: make test-users VUS=50 DURATION=2m)
	@k6 run k6-load-test.js \
		-e BASE_URL=$(BASE_URL) \
		--vus $(or $(VUS),10) \
		--duration $(or $(DURATION),1m)
