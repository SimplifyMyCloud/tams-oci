FROM fnproject/go:1.19-dev as build-stage

WORKDIR /function

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy Go source
COPY video_processor.go .

# Build the function
RUN CGO_ENABLED=0 GOOS=linux go build -o video_processor video_processor.go

# Runtime image
FROM fnproject/go:1.19

# Copy binary from build stage
COPY --from=build-stage /function/video_processor /function/video_processor

# Make executable
RUN chmod +x /function/video_processor

# Set as entrypoint
ENTRYPOINT ["/function/video_processor"]