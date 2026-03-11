DOCKER_REPO  ?= ghcr.io/damoun
BINARY_NAME  ?= twitch_exporter

include Makefile.common

DOCKER_IMAGE_NAME ?= twitch-exporter

docker:
	docker buildx build --load -t $(DOCKER_REPO)/$(DOCKER_IMAGE_NAME):$(SANITIZED_DOCKER_IMAGE_TAG) .
