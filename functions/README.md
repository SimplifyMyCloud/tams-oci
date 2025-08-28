# TAMS Video Chunker Function

Serverless video chunking function for Oracle Cloud Infrastructure (OCI) Functions.

## Overview

This function processes video files by:
1. Downloading video from a URL
2. Detecting scene changes using FFmpeg
3. Intelligently chunking video into ~60 second segments at scene boundaries
4. Uploading chunks to OCI Object Storage

## Features

- **FFmpeg-powered**: Industry-standard video processing
- **Scene-aware chunking**: Splits at natural scene boundaries
- **Configurable**: Adjustable duration, thresholds, and tolerance
- **Serverless**: Scales automatically with OCI Functions
- **Container-native**: Docker-based deployment

## Deployment

### Prerequisites

1. **OCI CLI configured**
2. **Fn CLI installed**
3. **Docker running**

### Steps

```bash
# 1. Setup Fn CLI context
fn create context tams-context --provider oracle

# 2. Configure context
fn use context tams-context
fn update context oracle.compartment-id <your-compartment-ocid>
fn update context api-url https://functions.<region>.oraclecloud.com
fn update context registry <region>.ocir.io/<namespace>/<repo-name>

# 3. Build and deploy
cd functions/video-chunker
fn deploy --app tams-functions-app

# 4. Test function
echo '{"video_url": "https://example.com/video.mp4", "target_duration": 60}' | fn invoke tams-functions-app video-chunker
```

## API Usage

### Request Format

```json
{
  "video_url": "https://example.com/sample-video.mp4",
  "target_duration": 60,
  "scene_threshold": 0.3,
  "max_deviation": 0.2
}
```

### Response Format

```json
{
  "status": "success",
  "message": "Video successfully chunked",
  "chunks_created": 12,
  "chunks": [
    {
      "id": 1,
      "start_time": 0,
      "end_time": 58.5,
      "duration": 58.5,
      "url": "https://objectstorage.region.oraclecloud.com/bucket/chunk_001.mp4"
    }
  ],
  "processing_time": "2m15s"
}
```

### Via API Gateway

```bash
curl -X POST \
  https://your-api-gateway-endpoint/tams/chunk-video \
  -H "Content-Type: application/json" \
  -d '{
    "video_url": "https://example.com/video.mp4",
    "target_duration": 60
  }'
```

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `target_duration` | 60 | Target chunk duration in seconds |
| `scene_threshold` | 0.3 | Scene detection sensitivity (0.0-1.0) |
| `max_deviation` | 0.2 | Allowable deviation from target (20%) |

### Environment Variables

- `TARGET_DURATION`: Default target duration
- `SCENE_THRESHOLD`: Default scene detection threshold  
- `MAX_DEVIATION`: Default maximum deviation
- `OCI_NAMESPACE`: Object Storage namespace
- `MEDIA_BUCKET`: Target bucket for chunks

## Architecture

```mermaid
sequenceDiagram
    participant Client
    participant Gateway as API Gateway
    participant Function as OCI Function
    participant Storage as Object Storage
    participant FFmpeg
    
    Client->>Gateway: POST /chunk-video
    Gateway->>Function: Invoke function
    Function->>Storage: Download video
    Function->>FFmpeg: Detect scenes
    FFmpeg-->>Function: Scene timestamps
    Function->>Function: Group into chunks
    Function->>FFmpeg: Split video
    FFmpeg-->>Function: Chunk files
    Function->>Storage: Upload chunks
    Storage-->>Function: Chunk URLs
    Function-->>Gateway: Response with URLs
    Gateway-->>Client: JSON response
```

## Performance

- **Timeout**: 5 minutes maximum
- **Memory**: 2048 MB allocated
- **Processing Speed**: ~3 minutes per hour of video
- **Concurrency**: Auto-scales based on demand

## Integration with TAMS

### Via Web UI
The TAMS Web UI can trigger chunking:

```javascript
// In TAMS Web UI
async function chunkVideo(assetId, videoUrl) {
    const response = await fetch('/api/functions/chunk-video', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            video_url: videoUrl,
            target_duration: 60
        })
    });
    
    const result = await response.json();
    console.log(`Created ${result.chunks_created} chunks`);
}
```

### Via API
Direct integration with TAMS API:

```go
// In TAMS API server
func chunkVideoHandler(w http.ResponseWriter, r *http.Request) {
    // Call OCI Functions via API Gateway
    resp, err := http.Post(
        "https://api-gateway-endpoint/tams/chunk-video",
        "application/json",
        bytes.NewBuffer(requestJSON),
    )
    // Handle response...
}
```

## Monitoring

- **Function Metrics**: Available in OCI Console
- **Logs**: Streamed to OCI Logging
- **Tracing**: APM integration available
- **Health Check**: `/health` endpoint

## Troubleshooting

### Common Issues

1. **Out of Memory**
   - Increase function memory allocation
   - Process smaller video files
   - Optimize FFmpeg parameters

2. **Timeout**
   - Increase function timeout
   - Split processing into smaller chunks
   - Use asynchronous processing

3. **FFmpeg Errors**
   - Check video format compatibility
   - Verify FFmpeg installation in container
   - Review scene detection parameters

### Debug Mode

Set environment variable `DEBUG=true` for verbose logging:

```bash
fn invoke tams-functions-app video-chunker --env DEBUG=true
```

## Cost Optimization

- **Free Tier**: 2M invocations/month
- **Pay-per-use**: Only charged during execution
- **Auto-scaling**: No idle costs
- **Container reuse**: Faster cold starts

## Security

- Functions run in private subnet
- Access via IAM policies
- Container image scanning
- Encrypted storage