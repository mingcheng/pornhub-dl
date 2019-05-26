.PHONY: build clean test test-race

VERSION=0.0.1
BIN=pornhub-dl

GO_ENV=CGO_ENABLED=1
GO_FLAGS=-ldflags="-X main.version=$(VERSION) -X 'main.buildTime=`date`' -extldflags -static"
GO=env $(GO_ENV) go

build: main.go
	@$(GO) build $(GO_FLAGS) -o $(BIN) $<

test:
	@$(GO) test .

test-race:
	@$(GO) test -race .

# clean all build result
clean:
	@$(GO) clean ./...
	@rm -f $(BIN)
	@rm -f *.tmp
