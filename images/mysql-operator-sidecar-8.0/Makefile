PLATFORMS := linux_amd64
include ../../build/makelib/common.mk

IMAGE = $(BUILD_REGISTRY)/mysql-operator-sidecar-8.0-$(ARCH)
include ../../build/makelib/image.mk

img.build:
	@$(MAKE) -C ../mysql-operator-sidecar-5.7 IMAGE=$(IMAGE) BUILD_ARGS="--build-arg PERCONA_XTRABACKUP_URL=https://downloads.percona.com/downloads/Percona-XtraBackup-8.0/Percona-XtraBackup-8.0.32-26/binary/debian/bullseye/x86_64/percona-xtrabackup-80_8.0.32-26-1.bullseye_amd64.deb"
