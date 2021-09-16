PLATFORMS := linux_amd64
include ../../build/makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/mysql-operator-sidecar-8.0-$(ARCH)
include ../../build/makelib/image.mk

img.build:
	@$(MAKE) -C ../mysql-operator-sidecar-5.7 IMAGE=$(IMAGE) BUILD_ARGS="--build-arg XTRABACKUP_PKG=percona-xtrabackup-80"
