# GreenPass 后端工程 Makefile —— 统一门禁入口
# 范围限定：只扫描后端 Go 代码（cmd/internal/pkg/test），避免混入前端 node_modules
GO      := go
GOSRC   := ./cmd/... ./internal/... ./pkg/... ./test/...
GODIRS  := ./cmd ./internal ./pkg ./test

.PHONY: validate lint test build vuln migcheck check tools clean

validate:
	gofumpt -l $(GODIRS)
	goimports -l $(GODIRS)
	go vet $(GOSRC)
	staticcheck $(GOSRC)

lint:
	golangci-lint run ./cmd/... ./internal/... ./pkg/...

test:
	go test -race $(GOSRC)

build:
	go build $(GOSRC)

vuln:
	govulncheck $(GOSRC)

# 迁移校验：跨平台主脚本（Linux CI 主用）；Windows 原生等价 .ps1
migcheck:
	./scripts/validate/validate-migrations.sh

check: validate lint test build vuln migcheck

tools:
	go install mvdan.cc/gofumpt@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
