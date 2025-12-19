DEBIAN_VERSION ?= bookworm
PG_VERSION ?= v18

GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
include ../../versions.env
HOST_ARCH ?= $(GOARCH)
TARGET_ARCH ?= $(HOST_ARCH)

AVAIL_TARGETS = amd64 arm64

.PHONY: build
build: outer

.PHONY: outer
outer: inner
	if [ "$(TARGET_ARCH)" = "amd64"]; then \
      DAEMON_IMAGE="ghcr.io/neondatabase/neonvm-daemon-amd64:b8b5d4c.17124015459"; \
    else \
      DAEMON_IMAGE=""; \
    fi; \
	../../bin/vm-builder \
		-src ghcr.io/simplyblock/autoscaling/vela-image:main \
		-dst vela-image-$(TARGET_ARCH)-inner \
		-daemon-image $(DAEMON_IMAGE) \
		-build-arg ALPINE_IMG_TAG=$(ALPINE_IMG_TAG) \
		-build-arg ALPINE_IMG_SHA=$(ALPINE_IMG_SHA) \
		-build-arg RUST_IMG_TAG=$(RUST_IMG_TAG) \
		-build-arg RUST_IMG_SHA=$(RUST_IMG_SHA) \
		-target-arch linux/$(TARGET_ARCH) \
		-size 2G \
		-spec image-spec.yaml

.PHONY: inner
.ONESHELL:
inner:
	@set -e; \
	if [[ "$(TARGET_ARCH)" == "amd64" ]]; then \
		DEBIAN_IMAGE="sha256:ef5c368548841bdd8199a8606f6307402f7f2a2f8edc4acbc9c1c70c340bc023"; \
	else \
	  	DEBIAN_IMAGE="sha256:594f2f110240a0a9c94d6e2a1020f6a05b79f713fcacd8c122ad8ff26e31d107"; \
	fi; \
	docker build -t vela-image-$(TARGET_ARCH)-inner --load --build-arg PG_VERSION=$(PG_VERSION) --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) --build-arg BOOKWORM_SLIM_SHA="$$DEBIAN_IMAGE" --build-arg TARGET_ARCH=$(TARGET_ARCH) --platform=linux/$(TARGET_ARCH) . -f ./inner.Dockerfile
