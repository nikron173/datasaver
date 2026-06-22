APP_NAME ?= datasaver
APP_DISK_AGENT ?= diskagent
APP_MEDIA_AGENT ?= mediaagent

prepare:
	go mod tidy

gen-proto:
	protoc --go_out=. --go-grpc_out=. api/proto/backup.proto

clean-diskagent:
	@rm -f ./bin/$(APP_DISK_AGENT)

clean-mediaagent:
	@rm -f ./bin/$(APP_MEDIA_AGENT)

clean-all: clean-diskagent clean-mediaagent

build-diskagent: clean-diskagent
	@mkdir -p ./bin
	@go build -o ./bin/$(APP_DISK_AGENT) ./cmd/$(APP_DISK_AGENT)

build-mediaagent: clean-mediaagent
	@mkdir -p ./bin
	@go build -o ./bin/$(APP_MEDIA_AGENT) ./cmd/$(APP_MEDIA_AGENT)

build-all: build-diskagent build-mediaagent
