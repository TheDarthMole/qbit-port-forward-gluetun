FROM --platform=${BUILDPLATFORM} golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

WORKDIR /app

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

# Copy the Go module files
COPY go.mod ./
COPY go.sum ./

# Download dependencies (optional, but recommended for caching)
RUN go mod download

# Copy the source code
COPY . .

RUN go test ./...

# Build the Go application
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -o /app/main .

# Use a smaller base image for the final image
FROM scratch AS minimal

# Copy the built binary from the builder stage
COPY --from=builder /app/main /app/main

USER 1000:1000

# Set the entrypoint to the Go application
CMD ["/app/main"]
