VERSION ?= 0.1.0
PROJECT := github.com/brimble/pulumi-nobus
PROVIDER := pulumi-resource-nobus
ROOT := $(abspath .)
BIN := $(ROOT)/bin/$(PROVIDER)
TOOLS_BIN := $(abspath ../.tools/bin)
export PATH := $(TOOLS_BIN):$(abspath ../.tools/bin-pulumi):$(PATH)
export GOSUMDB ?= off
export GOTOOLCHAIN ?= local

.PHONY: build schema sdk install test tidy clean example

build:
	cd provider && go build -o $(BIN) ./cmd/pulumi-resource-nobus

schema: build
	pulumi package get-schema $(BIN) > $(ROOT)/schema.json

sdk: schema
	rm -rf sdk/go/nobus
	pulumi package gen-sdk $(ROOT)/schema.json --language go --out sdk
	cp $(ROOT)/scripts/sdk-constants.go $(ROOT)/sdk/go/nobus/constants.go
	@test -f $(ROOT)/sdk/go/go.mod || printf '%s\n' 'module github.com/brimble/pulumi-nobus/sdk/go' 'go 1.22.0' 'require github.com/pulumi/pulumi/sdk/v3 v3.142.0' > $(ROOT)/sdk/go/go.mod
	bash $(ROOT)/scripts/use-local-deps.sh
	cd sdk/go && go mod tidy

install: build
	@mkdir -p "$(HOME)/.pulumi/plugins/resource-nobus-v$(VERSION)"
	cp "$(BIN)" "$(HOME)/.pulumi/plugins/resource-nobus-v$(VERSION)/$(PROVIDER)"
	@mkdir -p "$(ROOT)/.pulumi-plugins/resource-nobus-v$(VERSION)"
	cp "$(BIN)" "$(ROOT)/.pulumi-plugins/resource-nobus-v$(VERSION)/$(PROVIDER)"
	@echo "Installed local plugin: ~/.pulumi/plugins/resource-nobus-v$(VERSION)/$(PROVIDER)"
	@echo "Or use: PULUMI_DEBUG_PROVIDERS=nobus=$(BIN)"

test:
	cd provider && go test ./pkg/...

cover:
	cd provider && go test ./pkg/... -coverprofile=coverage.out -covermode=atomic
	cd provider && go tool cover -func=coverage.out | grep total
	@echo "HTML: cd provider && go tool cover -html=coverage.out"

tidy:
	cd provider && go mod tidy
	cd sdk/go && go mod tidy

clean:
	rm -rf bin schema.json

example:
	@echo "cd examples/go/basic && PULUMI_DEBUG_PROVIDERS=nobus=$(BIN) pulumi up"
