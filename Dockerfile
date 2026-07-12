# Usage:
#   $ docker build -t jira-cli:latest .
#   $ docker run --rm -it -v ~/.netrc:/root/.netrc -v ~/.config/.jira:/root/.config/.jira jira-cli

# Pinned to the build platform so Go cross-compiles for the target instead of
# the whole toolchain running emulated under QEMU. The image is built for four
# platforms on every merge to main, and emulation makes that take hours.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# CI passes the branch or tag being published. Left empty, the Makefile falls
# back to deriving it from git, which is what a local `docker build` wants.
ARG VERSION=

ENV CGO_ENABLED=0

WORKDIR /app

COPY . .

RUN apk add -U --no-cache make git

# `go install` refuses to cross-compile into GOBIN, hence the explicit
# `go build`. The linker flags still come from the Makefile so the version
# stamped here matches the one in a release binary.
RUN set -eux; \
    export GOOS="$TARGETOS" GOARCH="$TARGETARCH"; \
    if [ -n "$TARGETVARIANT" ]; then export GOARM="${TARGETVARIANT#v}"; fi; \
    if [ -z "$VERSION" ]; then unset VERSION; fi; \
    make deps; \
    go build -trimpath -ldflags="$(make -s ldflags)" -o /out/jira ./cmd/jira

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /out/jira /bin/jira

ENTRYPOINT ["/bin/sh"]
