
# Image URL to use all building/pushing image targets
IMG ?= quay.io/presslabs/mysql-operator:build
HELPER_IMG ?= quay.io/presslabs/mysql-helper:build

KUBEBUILDER_VERSION ?= 1.0.0

CMDS     := $(shell find ./cmd/ -maxdepth 1 -type d -exec basename {} \; | grep -v cmd)
GOFILES  := $(shell find cmd/ -name 'main.go' -type f )

all: test build

# Build binaries tag
build: $(patsubst %, bin/%_linux_amd64, $(CMDS))

# Run tests
test: generate fmt vet manifests
	go test ./pkg/... ./cmd/... -coverprofile cover.out

# Build binaries
bin/%: $(GOFILES) Makefile
	CGO_ENABLED=0 \
	GOOS=$(shell echo "$*" | cut -d'_' -f2) \
	GOARCH=$(shell echo "$*" | cut -d'_' -f3) \
		go build $(GOFLAGS) \
			-v -o $@ cmd/$(shell echo "$*" | cut -d'_' -f1)/main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Build the docker image
docker-build: $(patsubst %, bin/%_linux_amd64, $(CMDS))
	cp bin/mysql-operator_linux_amd64 hack/docker/mysql-operator/mysql-operator
	docker build -t ${IMG} hack/docker/mysql-operator/

	cp bin/mysql-helper_linux_amd64 hack/docker/mysql-helper/mysql-helper
	docker build  -t ${HELPER_IMG} hack/docker/mysql-helper/

# Push the docker image
docker-push:
	docker push ${IMG}

lint: vet
	gometalinter.v2 --disable-all --deadline 5m \
	--enable=vetshadow \
	--enable=misspell \
	--enable=structcheck \
	--enable=golint \
	--enable=deadcode \
	--enable=goimports \
	--enable=errcheck \
	--enable=varcheck \
	--enable=goconst \
	--enable=gas \
	--enable=unparam \
	--enable=ineffassign \
	--enable=nakedret \
	--enable=interfacer \
	--enable=misspell \
	--enable=gocyclo \
	--line-length=170 \
	--enable=lll \
	--dupl-threshold=400 \
	--enable=dupl \
	--enable=maligned \
./pkg/... ./cmd/...


dependencies:
	go get -u gopkg.in/alecthomas/gometalinter.v2
	gometalinter.v2 --install

	# install Kubebuilder
	curl -L -O https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
	tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64.tar.gz
	mv kubebuilder_${KUBEBUILDER_VERSION}_linux_amd64 -T /usr/local/kubebuilder
