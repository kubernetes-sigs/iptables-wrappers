BIN_DIR ?= bin
GO ?= go

all: build

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags='-s -w -extldflags="-static" -buildid=""' -trimpath -o $(BIN_DIR)/iptables-wrapper sigs.k8s.io/iptables-wrappers

clean:
	rm -f $(BIN_DIR)/iptables-wrapper

vet: ## Run go vet against code.
	$(GO) vet ./...

fmt: ## Check formatting
	if [ "$$(gofmt -e -l . | tee /dev/tty | wc -l)" -gt 0 ]; then \
		echo "Go files need formatting"; \
	exit 1; \
	fi

check: check-debian check-fedora check-alpine

check-debian: build
	./test/run-test.sh debian

check-fedora: build
	./test/run-test.sh fedora

check-alpine: build
	./test/run-test.sh alpine
