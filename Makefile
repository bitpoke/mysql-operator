# Project Setup
PROJECT_NAME := mysql-operator
PROJECT_REPO := github.com/bitpoke/mysql-operator

PLATFORMS := darwin_amd64 linux_amd64

DOCKER_REGISTRY := docker.io/bitpoke
IMAGES ?= mysql-operator mysql-operator-sidecar-5.7 mysql-operator-sidecar-8.0

GOLANGCI_LINT_VERSION = 1.25.0
GO111MODULE=on

include build/makelib/common.mk
include build/makelib/golang.mk
include build/makelib/kubebuilder-v2.mk
include build/makelib/image.mk

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/mysql-operator $(GO_PROJECT)/cmd/mysql-operator-sidecar $(GO_PROJECT)/cmd/orc-helper

GO_LDFLAGS += -X github.com/bitpoke/mysql-operator/pkg/version.buildDate=$(BUILD_DATE) \
	       -X github.com/bitpoke/mysql-operator/pkg/version.gitVersion=$(VERSION) \
	       -X github.com/bitpoke/mysql-operator/pkg/version.gitCommit=$(GIT_COMMIT) \
	       -X github.com/bitpoke/mysql-operator/pkg/version.gitTreeState=$(GIT_TREE_STATE)
