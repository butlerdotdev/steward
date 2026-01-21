# Build stage
FROM golang:latest AS builder

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build from root where main.go is
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o manager .

# Runtime stage
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /workspace/manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
