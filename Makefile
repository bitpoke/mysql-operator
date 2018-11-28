APP_VERSION ?= $(shell git describe --abbrev=5 --dirty --tags --always)
REGISTRY := quay.io/presslabs
IMAGE_NAME := mysql-operator
SIDECAR_IMAGE_NAME := mysql-operator-sidecar
BUILD_TAG := build
IMAGE_TAGS := $(APP_VERSION)

BINDIR := $(PWD)/bin
KUBEBUILDER_VERSION ?= 1.0.5
HELM_VERSION ?= 2.11.0

GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64

PATH := $(BINDIR):$(PATH)
SHELL := env PATH=$(PATH) /bin/sh

all: test build

# Run tests
test: generate fmt vet manifests
	ginkgo --randomizeAllSpecs --randomizeSuites --failOnPending \
			--cover --coverprofile cover.out --trace --race --progress  $(TEST_ARGS)\
			./pkg/... ./cmd/...

# Build mysql-operator binary
build: generate fmt vet
	go build -o bin/mysql-operator github.com/presslabs/mysql-operator/cmd/mysql-operator
	go build -o bin/mysql-operator-sidecar github.com/presslabs/mysql-operator/cmd/mysql-operator-sidecar

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/mysql-operator/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	cd hack && ./generate_chart_manifests.sh

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

lint:
	$(BINDIR)/golangci-lint run ./pkg/... ./cmd/...

.PHONY: chart
chart: generate manifests
	cd hack && ./generate_chart.sh $(APP_VERSION)

dependencies:
	test -d $(BINDIR) || mkdir $(BINDIR)
	GOBIN=$(BINDIR) go install ./vendor/github.com/onsi/ginkgo/ginkgo
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $(BINDIR) v1.10.2

# Build the docker image
.PHONY: images
images:
	docker build . -f Dockerfile -t $(REGISTRY)/$(IMAGE_NAME):$(BUILD_TAG)
	docker build . -f Dockerfile.sidecar -t $(REGISTRY)/$(SIDECAR_IMAGE_NAME):$(BUILD_TAG)
	set -e; \
		for tag in $(IMAGE_TAGS); do \
			docker tag $(REGISTRY)/$(IMAGE_NAME):$(BUILD_TAG) $(REGISTRY)/$(IMAGE_NAME):$${tag}; \
			docker tag $(REGISTRY)/$(SIDECAR_IMAGE_NAME):$(BUILD_TAG) $(REGISTRY)/$(SIDECAR_IMAGE_NAME):$${tag}; \
	done

# Push the docker image
.PHONY: publish
publish: images
	set -e; \
		for tag in $(IMAGE_TAGS); do \
		docker push $(REGISTRY)/$(IMAGE_NAME):$${tag}; \
		docker push $(REGISTRY)/$(SIDECAR_IMAGE_NAME):$${tag}; \
	done

# E2E tests
###########

KUBECONFIG ?= ~/.kube/config
K8S_CONTEXT ?= minikube

e2e-local: images
	go test ./test/e2e -v $(G_ARGS) -timeout 20m --pod-wait-timeout 60 \
		-ginkgo.slowSpecThreshold 300 \
		--kubernetes-config $(KUBECONFIG) --kubernetes-context $(K8S_CONTEXT) \
		--report-dir ../../e2e-reports

E2E_IMG_TAG ?= latest
e2e-remote:
	go test ./test/e2e -v $(G_ARGS) -timeout 50m --pod-wait-timeout 200 \
		-ginkgo.slowSpecThreshold 300 \
		--kubernetes-config $(KUBECONFIG) --kubernetes-context $(K8S_CONTEXT) \
		--report-dir ../../e2e-reports \
		--operator-image quay.io/presslabs/mysql-operator:$(E2E_IMG_TAG) \
		--sidecar-image  quay.io/presslabs/mysql-operator-sidecar:$(E2E_IMG_TAG)
