# disable default rules
.SUFFIXES:
MAKEFLAGS+=-r -R
export GOBIN = $(CURDIR)/bin
BINARY_NAME=rq
PM_BINARY_NAME=pm2rq
EXAMPLES_DIR=examples
EXAMPLE_FILES=$(sort $(wildcard $(EXAMPLES_DIR)/*.yaml))
CURL_HEALTH_CHECK=curl --connect-timeout 5 --max-time 10 --retry 5 --retry-delay 0 --retry-max-time 40 --retry-all-errors -s

.PHONY: test staticcheck examples all all-examples clean build build-pm2rq httpbin start-httpbin stop-httpbin

build:
	go build -o $(BINARY_NAME) cmd/rq/main.go

build-pm2rq:
	go build -o $(PM_BINARY_NAME) cmd/pm2rq/main.go

test:
	go test ./...

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

staticcheck: $(GOBIN)/staticcheck
	$(GOBIN)/staticcheck ./...

httpbin:
	@if docker ps -a --format '{{.Names}}' | grep -q '^httpbin$$'; then \
		echo "Container httpbin already exists, removing it first..."; \
		docker rm -f httpbin; \
	fi
	docker run -d --name httpbin -p 8080:80 kennethreitz/httpbin
	@echo "Waiting for httpbin container to be ready..."
	@if $(CURL_HEALTH_CHECK) http://localhost:8080/get >/dev/null 2>&1; then \
		echo "httpbin container is ready!"; \
	else \
		echo "httpbin container failed to start"; \
		exit 1; \
	fi

start-httpbin: httpbin

stop-httpbin:
	docker stop httpbin || true
	docker rm httpbin || true

examples: build start-httpbin
	@echo "Running examples against local httpbin container..."
	@if [ -z "$(EXAMPLE_FILES)" ]; then \
		echo "No example files found in $(EXAMPLES_DIR)"; \
		exit 1; \
	fi
	@for example in $(EXAMPLE_FILES); do \
		./$(BINARY_NAME) "$$example" || exit 1; \
	done

all: test staticcheck

all-examples: all examples

clean: stop-httpbin
	rm -f $(BINARY_NAME) $(PM_BINARY_NAME)
