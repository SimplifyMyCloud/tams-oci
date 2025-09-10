FROM fnproject/go:1.19-dev as build-stage

WORKDIR /function

# Copy Go source
COPY video_chunker.go .

# Build the function
RUN go build -o video_chunker video_chunker.go

# Runtime image with FFmpeg
FROM fnproject/go:1.19

# Install FFmpeg
RUN apt-get update && \
    apt-get install -y ffmpeg && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Copy binary from build stage
COPY --from=build-stage /function/video_chunker /function/video_chunker

# Make executable
RUN chmod +x /function/video_chunker

# Set as entrypoint
ENTRYPOINT ["/function/video_chunker"]