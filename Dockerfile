###############################################################################
#  Build the mysql-oerator related binaries
###############################################################################
FROM golang:1.11.2 as builder

# Copy in the go src
WORKDIR /go/src/github.com/presslabs/mysql-operator
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o mysql-operator github.com/presslabs/mysql-operator/cmd/mysql-operator
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o mysql-operator-sidecar github.com/presslabs/mysql-operator/cmd/mysql-operator-sidecar
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o orc-helper github.com/presslabs/mysql-operator/cmd/orc-helper

###############################################################################
#  Docker image for operator
###############################################################################
FROM scratch

# Copy the mysql-operator into its own image
COPY --from=builder /etc/ssl/certs /etc/ssl/certs
COPY --from=builder /go/src/github.com/presslabs/mysql-operator/mysql-operator /mysql-operator

ENTRYPOINT ["/mysql-operator"]
