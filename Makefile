DOCKER_REPO  ?= ghcr.io/damoun

include Makefile.common

DOCKER_IMAGE_NAME ?= twitch-exporter

docker:
	docker buildx build --load -t $(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(SANITIZED_DOCKER_IMAGE_TAG) .

promu-build:
	@echo ">> running promu crossbuild -v"
	promu crossbuild -v
