DEBIAN_VERSION ?= bookworm
PG_VERSION ?= v18

.PHONY: build
build:
	#curl -L -o pg18.tar.gz https://github.com/postgres/postgres/archive/refs/tags/REL_18_1.tar.gz
	#tar xvfz pg18.tar.gz
	docker build --build-arg PG_VERSION=$(PG_VERSION) --build-arg DEBIAN_VERSION=$(DEBIAN_VERSION) . -f ./compute-node.Dockerfile
