BINARY := cherry-agent-manager
DIST   := dist
# 去掉符号表与调试信息；-trimpath 让构建可复现
LDFLAGS := -s -w

.PHONY: build test vet fmt release clean regen-vectors

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

# 需要真实环境变量的集成测试（都会先复制副本，不动正式数据）
test-real:
	CHERRY_ASAR="$(CHERRY_ASAR)" CHERRY_DB="$(CHERRY_DB)" go test ./... -v

vet:
	go vet ./...

fmt:
	gofmt -w .

# 交叉编译出各平台单文件二进制
release: clean
	@mkdir -p $(DIST)
	@for t in windows/amd64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; \
		out=$(DIST)/$(BINARY)_$${os}_$${arch}; \
		[ "$$os" = "windows" ] && out=$$out.exe; \
		echo "  building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS)" -o $$out . || exit 1; \
	done
	@echo "\n完成，产物在 $(DIST)/"

# 用 Cherry Studio 内置的那份 fractional-indexing 重新生成排序键测试向量
regen-vectors:
	node tools/orderkey-vectors/gen.mjs

clean:
	rm -rf $(DIST) $(BINARY) $(BINARY).exe
