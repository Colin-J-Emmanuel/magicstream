# Build stage: compile a static Go binary.
FROM golang:1.27-alpine AS build
WORKDIR /src

# Download dependencies first so this layer is cached between builds
# when only application code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO off produces a fully static binary that runs on a minimal image.
# -trimpath and -ldflags shrink the binary and drop local build paths.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app .

# Runtime stage: no shell, no package manager, runs as a non-root user.
# distroless/static includes the CA certificates needed for TLS to
# MongoDB Atlas and the LLM provider.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app

ENV PORT=8080
ENV GIN_MODE=release
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]