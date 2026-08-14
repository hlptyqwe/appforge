MODULE := appforge/services/core
REPO_ROOT := ../..
PROTO_DIR := $(REPO_ROOT)/proto/core
PROTO_FILE := $(PROTO_DIR)/core.proto
GOCTL ?= $(shell go env GOPATH)/bin/goctl

.PHONY: gen gen-rpc check-goctl fix-imports fmt tidy clean-gen gen-model

gen: check-goctl gen-rpc

check-goctl:
	@$(GOCTL) --version | grep "1.9.2" >/dev/null || (echo "❌ 当前项目要求 goctl v1.9.2"; exit 1)

gen-rpc:
	@$(GOCTL) rpc protoc $(PROTO_FILE) \
		--proto_path=$(REPO_ROOT) \
		--module=$(MODULE) \
		--go_out=. --go-grpc_out=. --zrpc_out=. --style=gozero
	@$(MAKE) fix-imports
	@$(MAKE) fmt
	@$(MAKE) clean-gen
	@$(MAKE) tidy

fix-imports:
	@find . -name "*.go" -type f -exec sed -i '' -e 's#$(MODULE)/appforge/proto/core#appforge/proto/core#g' {} + 2>/dev/null || true
	@find . -name "*.go" -type f -exec sed -i '' -e 's#appforge/services/core/appforge/proto/core#appforge/proto/core#g' {} + 2>/dev/null || true
	@find . -name "*.go" -type f -exec sed -i '' -e 's/core_core\./core./g' {} + 2>/dev/null || true

fmt:
	@gofmt -w $$(find . -name "*.go" -type f)

tidy:
	@go mod tidy

clean-gen:
	@rm -rf appforge/

gen-model:
	@$(MAKE) check-goctl
	@echo "🔧 [services/core] generating models..."
	@$(GOCTL) model mysql ddl --src core.sql --dir ./models -c
	@echo "✅ [services/core] models generated"
