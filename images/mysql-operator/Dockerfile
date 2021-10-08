# Copy the mysql-operator binary into a thin image
# The image is pinned to the nonroot tag
FROM gcr.io/distroless/base-debian11@sha256:56d73a61ea1135c28f2be9afe2be88fc360e5fa1a892d600512a10eb2e028fa5

COPY rootfs /
USER nonroot
ENTRYPOINT ["/mysql-operator"]
CMD ["help"]
