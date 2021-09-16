# build
bitpoke GNU make based build system

## Goals

1. Allow building locally the same way the project is build on CI
2. Provide a sane test, build, publish flow
3. Provide stable toolchain for building (eg. pinned tool versions)
4. Enables caching for speeding up builds.

## Quickstart

```sh
git subtree add -P build https://github.com/bitpoke/build.git

cat <<EOF > Makefile
# Project Setup
PROJECT_NAME := mysql-operator
PROJECT_REPO := github.com/bitpoke/mysql-operator

include build/makelib/common.mk
```

## Usage

```
Usage: make [make-options] <target> [options]

Common Targets:
    build              Build source code and other artifacts for host platform.
    build.all          Build source code and other artifacts for all platforms.
    build.tools        Install the required build tools.
    clean              Remove all files created during the build.
    distclean          Remove all files created during the build including cached tools.
    generate           Run code generation tools.
    fmt                Run code auto-formatting tools.
    lint               Run lint and code analysis tools.
    test               Runs unit tests.
    e2e                Runs end-to-end integration tests.
    translate          Collect translation strings and post-process the .pot/.po files.
    help               Show this help info.
```

## Acknowledgement

This work is based on https://github.com/upbound/build.
