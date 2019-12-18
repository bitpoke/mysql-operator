# Project Setup
PROJECT_NAME := presslabs-build
PROJECT_REPO := github.com/presslabs/build

PLATFORMS = linux_amd64

include makelib/common.mk
include makelib/gcp.mk

IMAGES ?= bfc
DOCKER_REGISTRY ?= gcr.io/$(GOOGLE_CLOUD_PROJECT)/$(PROJECT_NAME)
include makelib/image.mk
