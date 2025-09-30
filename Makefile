APP_NAME := wcfc-updater
SHELL := /bin/bash

GO_FILES := $(shell find . -name '*.go' ! -name '*_test.go' ! -name '*_gen.go')
GO_DEPS := Makefile go.mod go.sum $(GO_FILES)
GOOGLE_CLOUD_REGION := us-central1
APP_VERSION := $(shell git describe --tags --dirty)
CONTAINER_TAG := $(GOOGLE_CLOUD_REGION)-docker.pkg.dev/wcfc-apps/wcfc-apps/$(APP_NAME):$(APP_VERSION)
DOCKER_FLAG_FILE_PREFIX := .docker-build
DOCKER_FLAG_FILE := $(DOCKER_FLAG_FILE_PREFIX)-$(APP_VERSION)

ifneq ($(shell which podman),)
	CONTAINER_CMD := podman
else
ifneq ($(shell which docker),)
	CONTAINER_CMD := docker
else
	CONTAINER_CMD := /bin/false  # force error when used
endif
endif

$(APP_NAME): $(GO_DEPS)
	CGO_ENABLED=0 go build -o $(APP_NAME) main.go

$(DOCKER_FLAG_FILE): $(APP_NAME) Dockerfile config.yml
	$(CONTAINER_CMD) build . -t $(CONTAINER_TAG)
	@touch $(DOCKER_FLAG_FILE)

.PHONY: build
build: $(DOCKER_FLAG_FILE)

.PHONY: check-version-not-dirty
check-version-not-dirty:
	@if [[ "$(CONTAINER_TAG)" == *"dirty"* ]]; then echo Refusing to push dirty version; git status; exit 1; fi

.PHONY: push
push: check-version-not-dirty $(DOCKER_FLAG_FILE)
	@echo Pushing $(CONTAINER_TAG)...
	@$(CONTAINER_CMD) push $(CONTAINER_TAG)

.PHONY: deploy
deploy: check-version-not-dirty push
	@gcloud run jobs deploy $(APP_NAME) --image $(CONTAINER_TAG) --region $(GOOGLE_CLOUD_REGION)

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: fmt
fmt:
	@go fmt ./...

.PHONY: check-fmt
check-fmt:
	@if [[ $$(gofmt -l .) ]]; then echo Code needs to be formatted; exit 1; fi

.PHONY: version
version:
	@echo $(APP_VERSION)

.PHONY: clean
clean:
	rm -f $(APP_NAME) $(DOCKER_FLAG_FILE_PREFIX)*

