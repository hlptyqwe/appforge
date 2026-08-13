.PHONY: gen clean

REPO_ROOT := ../..

gen:
	@echo "🔧 [proto/user] generating user pb..."
	cd $(REPO_ROOT) && protoc \
		-I . \
		--go_out=. --go_opt=paths=source_relative \
		proto/common/common.proto
	@echo "✅ [proto/user] generated"

clean:
	rm -f *.pb.go