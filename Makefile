APP_NAME ?= diskagent

clean:
	@rm -rf ./bin

build: clean
	@mkdir -p ./bin
	@go mod tidy
	@go build -o ./bin/$(APP_NAME) ./cmd/$(APP_NAME)

run: build
	@./bin/$(APP_NAME)
