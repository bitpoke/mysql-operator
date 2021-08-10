# Project Setup
PROJECT_NAME := presslabs-build
PROJECT_REPO := github.com/presslabs/build

PLATFORMS = linux_amd64

# this is required, since by default, the makelib files are under a ./build path prefix, but here,
# they are under root
ROOT_DIR := $(abspath $(shell cd ./ && pwd -P))

include makelib/common.mk

IMAGES ?= build
DOCKER_REGISTRY ?= docker.io/bitpoke

include makelib/image.mk
