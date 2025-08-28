FROM fnproject/go:1.19-dev as build-stage

WORKDIR /function

# Copy Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy Go source
COPY video_analyzer.go .

# Build the function
RUN CGO_ENABLED=0 GOOS=linux go build -o video_analyzer video_analyzer.go

# Runtime image
FROM fnproject/go:1.19

# Copy binary from build stage
COPY --from=build-stage /function/video_analyzer /function/video_analyzer

# Make executable
RUN chmod +x /function/video_analyzer

# Set as entrypoint
ENTRYPOINT ["/function/video_analyzer"]