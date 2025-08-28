FROM fnproject/go:1.19-dev as build-stage

WORKDIR /function

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy Go source
COPY chunk_notifier.go .

# Build the function
RUN CGO_ENABLED=0 GOOS=linux go build -o chunk_notifier chunk_notifier.go

# Runtime image
FROM fnproject/go:1.19

# Copy binary from build stage
COPY --from=build-stage /function/chunk_notifier /function/chunk_notifier

# Make executable
RUN chmod +x /function/chunk_notifier

# Set as entrypoint
ENTRYPOINT ["/function/chunk_notifier"]