VERSION=v1.0.0
build:
	@go mod tidy && CGO_ENABLED=0 go build -ldflags="-s -w" -o ./bin/approveflow.bin
