# 当前服务 module（必须与你 services/system/go.mod 的 module 一致）
MODULE := appforge/services/system

# 仓库根目录（从 services/system 往上两级）
REPO_ROOT := ../..

# proto 目录（属于 proto/system 模块）
PROTO_DIR := $(REPO_ROOT)/proto/system


PROTO_FILE := $(PROTO_DIR)/system.proto

GOCTL ?= $(shell go env GOPATH)/bin/goctl

.PHONY: gen gen-rpc fix-imports tidy fmt clean-gen gen-model print-protos install-goctl

gen: check-goctl gen-rpc


check-goctl:
	@$(GOCTL) --version | grep "1.9.2" >/dev/null || \
		(echo "❌ 当前项目要求 goctl v1.9.2"; exit 1)

install-goctl:
	go install github.com/zeromicro/go-zero/tools/goctl@v1.9.2

# 查看匹配到的 proto 文件
print-protos:
	@echo "PROTO_DIR=$(PROTO_DIR)"
	@echo "PROTO_FILE=$(PROTO_FILE)"

# 所有 RPC service 统一在 system.proto 中，以 multiple 模式分组生成。
gen-rpc:
	@echo "🔧 [services/system] goctl rpc protoc..."
	@$(GOCTL) rpc protoc $(PROTO_FILE) \
		--multiple \
		--proto_path=$(REPO_ROOT) \
		--module=$(MODULE) \
		--go_out=. \
		--go-grpc_out=. \
		--zrpc_out=. \
		--style=gozero
	@$(MAKE) fix-imports
	@$(MAKE) fmt
	@$(MAKE) clean-gen
	@$(MAKE) tidy
	@echo "✅ [services/system] gen-rpc done"

# 修复 goctl 可能生成的错误 import 路径/别名
fix-imports:
	@echo "🔧 [services/system] fixing imports..."
ifeq ($(shell uname -s),Darwin)
	@find . -name "*.go" -type f -exec sed -i '' -e 's#"$(MODULE)/appforge/proto/system"#"appforge/proto/system"#g' {} +
	@find . -name "*.go" -type f -exec sed -i '' -e 's#"appforge/services/system/appforge/proto/system"#"appforge/proto/system"#g' {} +
	@find . -name "*.go" -type f -exec sed -i '' -e 's/system_system\./system\./g' {} +
else
	@find . -name "*.go" -type f -exec sed -i -e 's#"$(MODULE)/appforge/proto/system"#"appforge/proto/system"#g' {} +
	@find . -name "*.go" -type f -exec sed -i -e 's#"appforge/services/system/appforge/proto/system"#"appforge/proto/system"#g' {} +
	@find . -name "*.go" -type f -exec sed -i -e 's/system_system\./system\./g' {} +
endif
	@echo "✅ [services/system] imports fixed"

fmt:
	@echo "🔧 [services/system] gofmt..."
	@gofmt -w $$(find . -name "*.go" -type f)

tidy:
	@echo "🔧 [services/system] go mod tidy..."
	@go mod tidy

# 谨慎：只清理 goctl 生成的 server 目录（避免误删你写的 logic）
clean-gen:
	@rm -rf appforge/

.PHONY: gen-model
gen-model:
	@$(MAKE) check-goctl
	@echo "🔧 [services/system] generating decimal-safe models..."
	@$(GOCTL) model mysql ddl --src system.sql --dir ./models -c
	@echo "✅ [services/system] models generated (DECIMAL -> decimal.Decimal)"
