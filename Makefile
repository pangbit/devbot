VERSION    ?= 0.1.0
COMMIT     := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS    := -s -w \
	-X devbot/internal/version.Version=$(VERSION) \
	-X devbot/internal/version.Commit=$(COMMIT) \
	-X devbot/internal/version.BuildTime=$(BUILD_TIME)

GOOS      := $(shell go env GOOS)
GOARCH    := $(shell go env GOARCH)
PKG_NAME  := devbot-v$(VERSION)-$(GOOS)-$(GOARCH)
DIST_DIR  := dist
STAGE_DIR := $(DIST_DIR)/$(PKG_NAME)
BINARY    := $(STAGE_DIR)/devbot

.PHONY: build package test clean

build:
	mkdir -p $(STAGE_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

package: build
	cp deploy/config.example.yaml $(STAGE_DIR)/
	cp deploy/devbot.service $(STAGE_DIR)/
	cp deploy/install.sh $(STAGE_DIR)/
	tar -czf $(DIST_DIR)/$(PKG_NAME).tar.gz -C $(DIST_DIR) $(PKG_NAME)
	@echo "打包完成: $(DIST_DIR)/$(PKG_NAME).tar.gz"

test:
	go test ./...

clean:
	rm -rf $(DIST_DIR)
