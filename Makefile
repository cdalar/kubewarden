SOURCE_FILES := $(shell find . -type f -name '*.go' -not -name '*_test.go')

# Same image the official kubewarden/go-policy-template pins.
TINYGO_VERSION ?= 0.39.0
CONTAINER_RUNTIME ?= docker

# -scheduler=none: the policy never spawns goroutines, and waPC calls the
# exported functions from the host after main() returned. With the default
# (asyncify) scheduler, newer TinyGo/Go combinations (e.g. TinyGo 0.43 +
# Go 1.27) panic in runtime.setupDeferFrame on the first json.Unmarshal.
# Verified to work on TinyGo 0.39/Go 1.25 and TinyGo 0.43/Go 1.27.
TINYGO_FLAGS := -target=wasip1 -scheduler=none -no-debug

.PHONY: all build test e2e-tests fmt lint clean

all: policy.wasm

# Uses a local tinygo when available, otherwise the official container image.
policy.wasm: $(SOURCE_FILES) go.mod go.sum
	@if command -v tinygo >/dev/null 2>&1; then \
		tinygo build -o policy.wasm $(TINYGO_FLAGS) . ; \
	else \
		$(CONTAINER_RUNTIME) run --rm \
			-e GOFLAGS="-buildvcs=false" \
			-v "$(PWD)":/src -w /src \
			tinygo/tinygo:$(TINYGO_VERSION) \
			tinygo build -o policy.wasm $(TINYGO_FLAGS) . ; \
	fi

build: policy.wasm

# Attach metadata.yml so kwctl / Kubewarden know the policy is mutating.
annotated-policy.wasm: policy.wasm metadata.yml README.md
	kwctl annotate -m metadata.yml -u README.md -o annotated-policy.wasm policy.wasm

test:
	go test -v ./...

e2e-tests: annotated-policy.wasm
	bats e2e.bats

fmt:
	gofmt -l -w .

lint:
	go vet ./...

clean:
	go clean
	rm -f policy.wasm annotated-policy.wasm
