.PHONY: gen clean

REPO_ROOT := ../..

gen:
	@echo "🔧 [proto/core] generating core pb..."
	@rm -f core*.pb.go
	cd $(REPO_ROOT) && protoc \
		-I . \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/common/common.proto proto/core/core.proto
	@echo "✅ [proto/core] generated"

clean:
	rm -f core*.pb.go
