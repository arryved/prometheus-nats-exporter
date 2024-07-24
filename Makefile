drepo ?= natsio

PACKAGE_NAME = arryved-nats-exporter
VERSION      = 0.15.1

prometheus-nats-exporter.docker:
	CGO_ENABLED=0 GOOS=linux go build -o $@ -v -a \
		-tags netgo -tags timetzdata \
		-installsuffix netgo -ldflags "-s -w"

.PHONY: dockerx
dockerx:
	docker buildx bake --load

.PHONY: build
build:
	go build

build-linux:
	GOOS=linux GOARCH=amd64 go build

.PHONY:
package: build-linux
	mkdir -p build
	nfpm pkg --packager deb --target build/

.PHONY: test
test:
	go test -v -race -count=1 -parallel=1 ./test/...
	go test -v -race -count=1 -parallel=1 ./collector/...
	go test -v -race -count=1 -parallel=1 ./exporter/...

.PHONY: test-cov
test-cov:
	go test -v -race -count=1 -parallel=1 ./test/...
	go test -v -race -count=1 -parallel=1 -coverprofile=collector.out ./collector/...
	go test -v -race -count=1 -parallel=1 -coverprofile=exporter.out ./exporter/...

.PHONY: lint
lint:
	@PATH=$(shell go env GOPATH)/bin:$(PATH)
	@if ! which  golangci-lint >/dev/null; then \
		echo "golangci-lint is required and was not found"; \
		exit 1; \
	fi
	go vet ./...
	$(shell go env GOPATH)/bin/golangci-lint run ./...

.PHONY: deploy
deploy: package
	gcloud artifacts apt upload arryved-apt --location=us-central1 --project=arryved-tools --source=build/$(PACKAGE_NAME)_$(VERSION)_amd64.deb
