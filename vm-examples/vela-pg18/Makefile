DEBIAN_VERSION ?= bookworm
PG_VERSION ?= v18

.PHONY: build
build: arm64 amd64
	echo "Finished building..."

.PHONY: arm64
arm64: configure-buildkit
	docker buildx build --load --buildkitd-flags --build-arg PG_VERSION=$(PG_VERSION) --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) --build-arg BOOKWORM_SLIM_SHA="sha256:594f2f110240a0a9c94d6e2a1020f6a05b79f713fcacd8c122ad8ff26e31d107" --build-arg TARGETARCH="arm64" --platform=linux/arm64 . -f ./compute-node.Dockerfile

.PHONY: amd64
amd64: configure-buildkit
	docker buildx build --load --build-arg PG_VERSION=$(PG_VERSION) --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) --build-arg BOOKWORM_SLIM_SHA="sha256:ef5c368548841bdd8199a8606f6307402f7f2a2f8edc4acbc9c1c70c340bc023" --build-arg TARGETARCH="amd64" --platform=linux/amd64 . -f ./compute-node.Dockerfile

.PHONY: configure-buildkit
configure-buildkit:
	docker buildx create --buildkitd-config ./buildkitd.toml --driver docker-container --name vela-image-builder --platform linux/arm64,linux/amd64
