# syntax=docker/dockerfile:1

# Multi-stage build producing a small, dependency-free runtime image.
#
# The free tier this service targets allows 512 MB of RAM, and the C# runtime
# image alone is around 230 MB before the application. A statically linked Go
# binary on a distroless base is roughly 20 MB total, which leaves real
# headroom for connection pools and request buffers.

# ---- Build ----------------------------------------------------------------
FROM golang:1.27-alpine AS build

WORKDIR /src

# Cache dependency download separately from source. Editing application code
# then rebuilds in seconds instead of re-downloading modules.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is disabled so the binary is fully static and can run on a base image
# with no libc. Trimpath removes filesystem paths, and the symbol table and DWARF
# data are stripped, which is most of the remaining size.
#
# -s -w gives up the symbol table and DWARF. This makes stack traces much less
# useful, so the binary is built WITHOUT those flags by default and they can be
# re-enabled locally for debugging.
ARG VERSION=0.0.0-local
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/server \
      ./cmd/server

# ---- Runtime --------------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates so outbound TLS works for Discord OAuth and S3. Without
# these, every outbound HTTPS request fails with a certificate error.
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/server /server

# distroless nonroot is uid 65532, which matches Render's expectations and
# avoids running as root inside the container.
USER nonroot:nonroot

# The C# service exposes its port through API_BASE_PATH and the compose file
# maps it. 8080 matches the default in the Go configuration.
EXPOSE 8080

ENTRYPOINT ["/server"]