APP_VERSION ?= $(shell git describe --abbrev=5 --dirty --tags --always)
REGISTRY := quay.io/presslabs
IMAGE_NAME := mysql-operator
SIDECAR_IMAGE_NAME := mysql-operator-sidecar
BUILD_TAG := build
IMAGE_TAGS := $(APP_VERSION)
PKG_NAME := github.com/presslabs/mysql-operator

BINDIR := $(PWD)/bin
KUBEBUILDER_VERSION ?= 1.0.7
HELM_VERSION ?= 2.11.0

GOOS ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH ?= amd64

PATH := $(BINDIR):$(PATH)
SHELL := env PATH=$(PATH) /bin/sh

# check if kubebuilder is installed in local bin dir and set KUBEBUILDER_ASSETS
ifeq 'yes' "$(shell test -f $(BINDIR)/kubebuilder && echo -n 'yes')"
	KUBEBUILDER_ASSETS ?= $(BINDIR)
endif

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
	go build -o bin/orc-helper github.com/presslabs/mysql-operator/cmd/orc-helper

# skaffold build
bin/mysql-operator_linux_amd64: $(shell hack/development/related-go-files.sh $(PKG_NAME) cmd/mysql-operator/main.go)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/mysql-operator_linux_amd64 github.com/presslabs/mysql-operator/cmd/mysql-operator

bin/mysql-operator-sidecar_linux_amd64: $(shell hack/development/related-go-files.sh $(PKG_NAME) cmd/mysql-operator-sidecar/main.go)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/mysql-operator-sidecar_linux_amd64 github.com/presslabs/mysql-operator/cmd/mysql-operator-sidecar

bin/orc-helper_linux_amd64: $(shell hack/development/related-go-files.sh $(PKG_NAME) cmd/orc-helper/main.go)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o bin/orc-helper_linux_amd64 github.com/presslabs/mysql-operator/cmd/orc-helper

skaffold-build: bin/mysql-operator_linux_amd64 bin/mysql-operator-sidecar_linux_amd64 bin/orc-helper_linux_amd64

skaffold-run: skaffold-build
	skaffold run --cache-artifacts=true

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
	hack/license-check

.PHONY: chart
chart: generate manifests
	cd hack && ./generate_chart.sh $(APP_VERSION)

dependencies:
	test -d $(BINDIR) || mkdir $(BINDIR)
	GOBIN=$(BINDIR) go install ./vendor/github.com/onsi/ginkgo/ginkgo
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $(BINDIR) v1.24.0

dependencies-local: dependencies
	curl -sL https://github.com/mikefarah/yq/releases/download/2.4.0/yq_$(GOOS)_$(GOARCH) -o $(BINDIR)/yq
	chmod +x $(BINDIR)/yq
	curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $(BINDIR) v1.24.0
	curl -sL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v$(KUBEBUILDER_VERSION)/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH).tar.gz | \
				tar -zx -C $(BINDIR) --strip-components=2
	curl -sL https://kubernetes-helm.storage.googleapis.com/helm-v$(HELM_VERSION)-$(GOOS)-$(GOARCH).tar.gz | \
		tar -C $(BINDIR) -xz --strip-components 1 $(GOOS)-$(GOARCH)/helm
	chmod +x $(BINDIR)/helm

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
		--sidecar-image  quay.io/presslabs/mysql-operator-sidecar:$(E2E_IMG_TAG) \
		--orchestrator-image  quay.io/presslabs/mysql-operator-orchestrator:$(E2E_IMG_TAG)
