.PHONY: gen clean

REPO_ROOT := ../..

gen:
	@echo "🔧 [proto/system] generating system pb..."
	@rm -f system_*.pb.go
	cd $(REPO_ROOT) && protoc \
		-I . \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/common/common.proto \
		proto/system/*.proto
	@echo "✅ [proto/system] generated"

clean:
	rm -f *.pb.go
