# syntax=docker/dockerfile:1

# Multi-stage build producing a small, dependency-free runtime image.
#
# The free tier this service targets allows 512 MB of RAM, and the C# runtime
# image alone is around 230 MB before the application. The Go image is roughly
# 79 MB, which still leaves real headroom for connection pools and request
# buffers. It was closer to 34 MB while the binary was fully static; the WebP
# encoder's dependency on libwebp-through-WASM moved it back onto glibc.

# ---- Build ----------------------------------------------------------------
FROM golang:1.27-bookworm AS build

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
#
# NOTE: this is `base`, not `static`, and the distinction is load-bearing.
#
# The WebP encoder (github.com/gen2brain/webp) depends on ebitengine/purego,
# which ships a fake cgo shim. That forces the external linker even under
# CGO_ENABLED=0, so the binary keeps a PT_INTERP and cannot start on a glibc-free
# base — it fails with `exec /server: no such file or directory`.
#
# -linkmode=internal does not undo it; the shim is what keeps the linker out.
# The glibc base accepts the dynamic binary, so that is what this stage uses.
# Switching back to `static` requires dropping the WebP dependency first.
FROM gcr.io/distroless/base-debian12:nonroot

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