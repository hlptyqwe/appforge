.PHONY: gen clean

REPO_ROOT := ../..

gen:
	@echo "🔧 [proto/builder] generating builder pb..."
	@rm -f builder*.pb.go
	cd $(REPO_ROOT) && protoc \
		-I . \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/common/common.proto proto/builder/builder.proto
	@echo "✅ [proto/builder] generated"

clean:
	rm -f builder*.pb.go
