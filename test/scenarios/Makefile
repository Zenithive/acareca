# Makefile for k6 Load Tests

# Default configuration
BASE_URL ?= http://localhost:8080
TEST_EMAIL ?= mihir@yopmail.com
TEST_PASSWORD ?= @Demo1234
ADMIN_EMAIL ?= admin@acareca.com
ADMIN_PASSWORD ?= @Admin1234

# Export environment variables
export BASE_URL
export TEST_EMAIL
export TEST_PASSWORD
export ADMIN_EMAIL
export ADMIN_PASSWORD

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-25s$(NC) %s\n", $$1, $$2}'

.PHONY: test-practitioner
test-practitioner: ## Run practitioner load tests
	@echo "$(GREEN)Running Practitioner Load Tests...$(NC)"
	k6 run practitioner-test.js

.PHONY: test-accountant
test-accountant: ## Run accountant load tests
	@echo "$(GREEN)Running Accountant Load Tests...$(NC)"
	k6 run accountant-test.js

.PHONY: test-admin
test-admin: ## Run admin load tests
	@echo "$(GREEN)Running Admin Load Tests...$(NC)"
	k6 run admin-test.js

.PHONY: test-auth
test-auth: ## Run authentication flow tests
	@echo "$(GREEN)Running Authentication Flow Tests...$(NC)"
	k6 run auth-flow-test.js

.PHONY: test-business
test-business: ## Run business operations tests
	@echo "$(GREEN)Running Business Operations Tests...$(NC)"
	k6 run business-operations-test.js

.PHONY: test-api
test-api: ## Run API endpoint tests
	@echo "$(GREEN)Running API Endpoint Tests...$(NC)"
	k6 run api-endpoint-test.js

.PHONY: test-all
test-all: ## Run all test scenarios sequentially
	@echo "$(GREEN)Running All Test Scenarios...$(NC)"
	@$(MAKE) test-practitioner
	@$(MAKE) test-accountant
	@$(MAKE) test-admin
	@$(MAKE) test-auth
	@$(MAKE) test-business
	@$(MAKE) test-api
	@echo "$(GREEN)All tests completed!$(NC)"

.PHONY: test-core
test-core: ## Run core tests (practitioner, accountant, admin)
	@echo "$(GREEN)Running Core Test Scenarios...$(NC)"
	@$(MAKE) test-practitioner
	@$(MAKE) test-accountant
	@$(MAKE) test-admin
	@echo "$(GREEN)Core tests completed!$(NC)"

.PHONY: test-smoke
test-smoke: ## Run smoke tests (quick validation)
	@echo "$(GREEN)Running Smoke Tests...$(NC)"
	k6 run --duration 30s --vus 5 practitioner-test.js
	k6 run --duration 30s --vus 5 accountant-test.js
	k6 run --duration 30s --vus 5 admin-test.js

.PHONY: test-stress
test-stress: ## Run stress tests (high load)
	@echo "$(YELLOW)Running Stress Tests (High Load)...$(NC)"
	k6 run --duration 5m --vus 100 practitioner-test.js
	k6 run --duration 5m --vus 100 accountant-test.js
	k6 run --duration 5m --vus 100 admin-test.js

.PHONY: test-soak
test-soak: ## Run soak tests (extended duration)
	@echo "$(YELLOW)Running Soak Tests (Extended Duration)...$(NC)"
	k6 run --duration 30m --vus 20 practitioner-test.js
	k6 run --duration 30m --vus 20 accountant-test.js
	k6 run --duration 30m --vus 20 admin-test.js

.PHONY: clean
clean: ## Clean up generated test files
	@echo "$(GREEN)Cleaning up test artifacts...$(NC)"
	rm -f practitioner-summary.json
	rm -f accountant-summary.json
	rm -f admin-summary.json
	rm -f summary.json
	rm -f report.html
	@echo "$(GREEN)Cleanup completed!$(NC)"

.PHONY: check-k6
check-k6: ## Check if k6 is installed
	@which k6 > /dev/null || (echo "$(RED)k6 is not installed. Please install it from https://k6.io/docs/getting-started/installation/$(NC)" && exit 1)
	@echo "$(GREEN)k6 is installed: $$(k6 version)$(NC)"

.PHONY: setup
setup: check-k6 ## Verify test environment setup
	@echo "$(GREEN)Checking test environment...$(NC)"
	@echo "BASE_URL: $(BASE_URL)"
	@echo "TEST_EMAIL: $(TEST_EMAIL)"
	@echo "ADMIN_EMAIL: $(ADMIN_EMAIL)"
	@echo "$(GREEN)Environment setup complete!$(NC)"

.PHONY: view-results
view-results: ## View test results
	@echo "$(GREEN)Test Results:$(NC)"
	@if [ -f practitioner-summary.json ]; then \
		echo "$(YELLOW)Practitioner Test Results:$(NC)"; \
		cat practitioner-summary.json | jq '.metrics | keys'; \
	fi
	@if [ -f accountant-summary.json ]; then \
		echo "$(YELLOW)Accountant Test Results:$(NC)"; \
		cat accountant-summary.json | jq '.metrics | keys'; \
	fi
	@if [ -f admin-summary.json ]; then \
		echo "$(YELLOW)Admin Test Results:$(NC)"; \
		cat admin-summary.json | jq '.metrics | keys'; \
	fi

# Custom test runs
.PHONY: test-practitioner-quick
test-practitioner-quick: ## Quick practitioner test (1 minute)
	@echo "$(GREEN)Running Quick Practitioner Test...$(NC)"
	k6 run --duration 1m --vus 10 practitioner-test.js

.PHONY: test-accountant-quick
test-accountant-quick: ## Quick accountant test (1 minute)
	@echo "$(GREEN)Running Quick Accountant Test...$(NC)"
	k6 run --duration 1m --vus 10 accountant-test.js

.PHONY: test-admin-quick
test-admin-quick: ## Quick admin test (1 minute)
	@echo "$(GREEN)Running Quick Admin Test...$(NC)"
	k6 run --duration 1m --vus 10 admin-test.js

.PHONY: test-practitioner-heavy
test-practitioner-heavy: ## Heavy practitioner test (high load)
	@echo "$(YELLOW)Running Heavy Practitioner Test...$(NC)"
	k6 run --duration 5m --vus 50 practitioner-test.js

.PHONY: test-accountant-heavy
test-accountant-heavy: ## Heavy accountant test (high load)
	@echo "$(YELLOW)Running Heavy Accountant Test...$(NC)"
	k6 run --duration 5m --vus 50 accountant-test.js

.PHONY: test-admin-heavy
test-admin-heavy: ## Heavy admin test (high load)
	@echo "$(YELLOW)Running Heavy Admin Test...$(NC)"
	k6 run --duration 5m --vus 50 admin-test.js

# Development targets
.PHONY: dev-test
dev-test: ## Run quick tests for development
	@echo "$(GREEN)Running Development Tests...$(NC)"
	@$(MAKE) test-practitioner-quick
	@$(MAKE) test-accountant-quick
	@$(MAKE) test-admin-quick

.PHONY: ci-test
ci-test: ## Run tests suitable for CI/CD
	@echo "$(GREEN)Running CI/CD Tests...$(NC)"
	k6 run --duration 2m --vus 20 practitioner-test.js
	k6 run --duration 2m --vus 20 accountant-test.js
	k6 run --duration 2m --vus 20 admin-test.js

# Default target
.DEFAULT_GOAL := help
