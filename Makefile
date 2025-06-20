BINARY_NAME=rq
EXAMPLES_DIR=examples

.PHONY: test staticcheck examples all clean build

build:
	go build -o $(BINARY_NAME) cmd/rq/main.go

test:
	go test ./...

staticcheck:
	staticcheck ./...

examples: build
	@for example in $(shell ls $(EXAMPLES_DIR)/*.yaml | sort); do \
		./$(BINARY_NAME) "$$example" || exit 1; \
	done

all: test staticcheck examples

clean:
	rm -f $(BINARY_NAME) coverage.out 