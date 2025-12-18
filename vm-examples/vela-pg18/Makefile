DEBIAN_VERSION ?= bookworm
PG_VERSION ?= v18

GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)
include ../../versions.env
HOST_ARCH ?= $(GOARCH)
TARGET_ARCH ?= $(HOST_ARCH)

.PHONY: build
build: outer

.PHONY: outer
outer: #inner
	../../bin/vm-builder \
		-src ghcr.io/simplyblock/autoscaling/vela-image:main \
		-dst vela-image-inner \
		-daemon-image ghcr.io/neondatabase/neonvm-daemon-amd64:b8b5d4c.17124015459 \
		-build-arg ALPINE_IMG_TAG=$(ALPINE_IMG_TAG) \
		-build-arg ALPINE_IMG_SHA=$(ALPINE_IMG_SHA) \
		-build-arg RUST_IMG_TAG=$(RUST_IMG_TAG) \
		-build-arg RUST_IMG_SHA=$(RUST_IMG_SHA) \
		-target-arch linux/$(TARGET_ARCH) \
		-size 2G \
		-spec image-spec.yaml

.PHONY: inner
inner:
	docker build -t vela-image-inner --load --build-arg PG_VERSION=$(PG_VERSION) --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) --build-arg BOOKWORM_SLIM_SHA=sha256:ef5c368548841bdd8199a8606f6307402f7f2a2f8edc4acbc9c1c70c340bc023 --build-arg TARGETARCH="amd64" --platform=linux/amd64 . -f ./inner.Dockerfile
