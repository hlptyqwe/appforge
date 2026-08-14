API_DIR := api
API_FILE := $(API_DIR)/core.api

# 被 core.api import 的子文件，只用于检查
SUB_APIS := auth_public auth_private system platform open

GOCTL ?= $(shell go env GOPATH)/bin/goctl
SERVICE_NAME ?= admin
ENTRY_FILE := $(SERVICE_NAME).go

.PHONY: gen regen check check-goctl install-goctl fmt verify clean fmt-api

# 一键生成
gen: check check-goctl
	@echo "🔧 generating from $(API_FILE) ..."
	@$(GOCTL) api go -api $(API_FILE) -dir .
	@echo "✅ generate done"

# 重新生成
regen: clean gen fmt verify

# 检查 api 结构
check:
	@test -f $(API_FILE) || (echo "❌ missing api file: $(API_FILE)"; exit 1)

	@for m in $(SUB_APIS); do \
		f="$(API_DIR)/$$m.api"; \
		test -f "$$f" || { echo "❌ missing api file: $$f"; exit 1; }; \
		if grep -nE '^[[:space:]]*syntax[[:space:]]*=' "$$f" >/dev/null 2>&1; then \
			echo "❌ $$f contains 'syntax = ...' (must be only in $(API_FILE))"; exit 1; \
		fi; \
		if grep -nE '^[[:space:]]*info[[:space:]]*\(' "$$f" >/dev/null 2>&1; then \
			echo "❌ $$f contains 'info(...)' (must be only in $(API_FILE))"; exit 1; \
		fi; \
	done

	@names=$$(grep -hE '^[[:space:]]*service[[:space:]]+[A-Za-z0-9_]+' $(API_DIR)/*.api \
		| sed -E 's/.*service[[:space:]]+([A-Za-z0-9_]+).*/\1/' \
		| sort -u); \
	cnt=$$(printf "%s\n" "$$names" | sed '/^$$/d' | wc -l); \
	if [ "$$cnt" -gt 1 ]; then \
		echo "❌ multiple service names found:"; \
		printf "%s\n" "$$names"; \
		exit 1; \
	fi

	@echo "✅ api files check ok"

check-goctl:
	@test -x "$(GOCTL)" || \
		(echo "❌ goctl not found: $(GOCTL)"; echo "   run: make install-goctl"; exit 1)
	@$(GOCTL) --version | grep "1.9.2" >/dev/null || \
		(echo "❌ 当前项目要求 goctl v1.9.2"; exit 1)

install-goctl:
	go install github.com/zeromicro/go-zero/tools/goctl@v1.9.2

# 格式化
fmt:
	@echo "🔧 gofmt ..."
	@gofmt -w $$(find . -name "*.go" -type f)

# 校验生成结果
verify:
	@echo "🔎 verifying generated files..."
	@test -f $(ENTRY_FILE) || (echo "❌ $(ENTRY_FILE) not found"; exit 1)
	@test -f internal/handler/routes.go || (echo "❌ internal/handler/routes.go not found"; exit 1)
	@test -f internal/types/types.go || (echo "❌ internal/types/types.go not found"; exit 1)
	@echo "✅ verify ok"

# 只清理“肯定是 goctl 生成且可安全重建”的文件
clean:
	@echo "⚠️ cleaning generated entry files..."
	@rm -f $(ENTRY_FILE)
	@rm -f internal/handler/routes.go
	@rm -f internal/types/types.go
	@echo "✅ clean done"

fmt-api:
	$(GOCTL) api format --dir ./api
