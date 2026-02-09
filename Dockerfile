# Build stage
FROM --platform=$BUILDPLATFORM golang:latest AS builder

ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build for target architecture
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o manager .

# Runtime stage
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /workspace/manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
