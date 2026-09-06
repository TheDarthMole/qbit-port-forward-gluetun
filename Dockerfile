FROM --platform=${BUILDPLATFORM} golang:alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build -o main .

# Use a smaller base image for the final image
FROM scratch AS minimal

# Copy the built binary from the builder stage
COPY --from=builder /app/main .

# Set the entrypoint to the Go application
CMD ["/app/main"]
