.PHONY: build clean test build-local

BINARY_NAME=bootstrap
ZIP_NAME=scheduling-deployment.zip

build:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o $(BINARY_NAME) cmd/main.go
	zip $(ZIP_NAME) $(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME) $(ZIP_NAME) $(BINARY_NAME)-local

test:
	go test ./...

build-local:
	go build -o $(BINARY_NAME)-local cmd/main.go
