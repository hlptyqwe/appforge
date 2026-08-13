MODULE := appforge/services/builder
REPO_ROOT := ../..
PROTO_DIR := $(REPO_ROOT)/proto/builder
PROTO_FILE := $(PROTO_DIR)/builder.proto
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
	@$(MAKE) tidy

fix-imports:
	@find . -name "*.go" -type f -exec sed -i '' -e 's#$(MODULE)/appforge/proto/builder#appforge/proto/builder#g' {} + 2>/dev/null || true
	@find . -name "*.go" -type f -exec sed -i '' -e 's#appforge/services/builder/appforge/proto/builder#appforge/proto/builder#g' {} + 2>/dev/null || true
	@find . -name "*.go" -type f -exec sed -i '' -e 's/builder_builder\./builder./g' {} + 2>/dev/null || true

fmt:
	@gofmt -w $$(find . -name "*.go" -type f)

tidy:
	@go mod tidy

clean-gen:
	@rm -rf appforge/

# Builder V1 不拥有业务表，构建任务模型由 services/core 生成并维护。
gen-model:
	@echo "ℹ️ [services/builder] no models: build_task belongs to services/core"
