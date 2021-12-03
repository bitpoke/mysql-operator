# Copy the mysql-operator binary into a thin image
# The image is pinned to the nonroot tag
FROM gcr.io/distroless/base-debian11@sha256:e76722f06f7c15e0076072fb02782ec59923b0d658b8a3d80bb79deaee6fb44d

COPY rootfs /
ENTRYPOINT ["/mysql-operator"]
CMD ["help"]
