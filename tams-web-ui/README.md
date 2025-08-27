# TAMS Web UI

A clean, modern web interface for the Television Archive Management System (TAMS) API, built with Go and vanilla JavaScript.

## Features

### 📊 Dashboard
- Real-time system metrics
- Asset and chunk statistics  
- Storage usage tracking
- Migration job status
- Recent asset activity

### 📼 Asset Management
- Browse and search media assets
- View detailed asset information
- See chunk distribution and metadata
- Create new asset entries
- Start migration jobs directly

### ☁️ Migration Tracking
- Monitor active migrations
- View job progress in real-time
- Retry failed migrations
- Support for multiple source types (tape, NAS, cloud)

## Tech Stack

- **Backend**: Go 1.21+ (standard library only - zero dependencies!)
- **Frontend**: HTML5, Tailwind CSS (CDN), Vanilla JavaScript
- **Templates**: Go html/template
- **Styling**: Tailwind CSS via CDN
- **Icons**: Font Awesome

## Quick Start

### Local Development

```bash
# Run directly with Go
go run main.go

# Or use make
make dev

# Build and run
make run
```

### Docker

```bash
# Build image
docker build -t tams-web-ui .

# Run with docker-compose
docker-compose up -d

# Or use make
make docker-build
make docker-run
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Web UI port | 8090 |
| `TAMS_API_URL` | TAMS API base URL | http://localhost:8080 |
| `TAMS_API_KEY` | API authentication key | (empty) |

## Project Structure

```
tams-web-ui/
├── main.go              # Go server and API client
├── templates/           # HTML templates
│   ├── base.html       # Layout template
│   ├── dashboard.html  # Dashboard page
│   ├── assets.html     # Assets list
│   ├── asset-detail.html
│   ├── migrations.html # Migration jobs
│   ├── new-asset.html  # Asset creation form
│   └── new-migration.html
├── static/             # Static assets
│   └── app.js         # Frontend JavaScript
├── Dockerfile         # Multi-stage Docker build
├── docker-compose.yml # Docker Compose config
└── Makefile          # Build automation
```

## API Endpoints

### Web Pages
- `GET /` - Dashboard
- `GET /assets` - Assets list
- `GET /asset/:id` - Asset detail
- `GET /assets/new` - New asset form
- `POST /assets/new` - Create asset
- `GET /migrations` - Migration jobs
- `GET /migrations/new` - New migration form
- `POST /migrations/new` - Start migration
- `GET /health` - Health check

### Features

#### Real-time Updates
- Auto-refresh for running migration jobs
- API connection status monitoring
- Progress bar animations

#### Keyboard Shortcuts
- `Alt+A` - Go to Assets
- `Alt+M` - Go to Migrations  
- `Alt+D` - Go to Dashboard
- `Alt+N` - New Asset (when on assets page)

#### Responsive Design
- Mobile-friendly interface
- Adaptive grid layouts
- Touch-friendly controls

## Development

### Prerequisites
- Go 1.21 or higher
- Docker (optional)
- Make (optional)

### Building

```bash
# Build binary
go build -o bin/tams-web-ui main.go

# Cross-platform builds
make build-all
```

### Testing

```bash
# Run tests
go test -v ./...

# Format code
go fmt ./...
```

## Deployment

### Standalone Binary

```bash
# Build
go build -o tams-web-ui main.go

# Run
PORT=8090 TAMS_API_URL=https://api.example.com ./tams-web-ui
```

### Docker

```bash
# Build image
docker build -t tams-web-ui:latest .

# Run container
docker run -d \
  -p 8090:8090 \
  -e TAMS_API_URL=https://api.example.com \
  -e TAMS_API_KEY=your-api-key \
  tams-web-ui:latest
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tams-web-ui
spec:
  replicas: 2
  selector:
    matchLabels:
      app: tams-web-ui
  template:
    metadata:
      labels:
        app: tams-web-ui
    spec:
      containers:
      - name: tams-web-ui
        image: tams-web-ui:latest
        ports:
        - containerPort: 8090
        env:
        - name: TAMS_API_URL
          value: "http://tams-api:8080"
```

## Security Notes

- No authentication built-in (relies on TAMS API authentication)
- Recommended to run behind a reverse proxy with TLS
- Add authentication layer if exposing to internet
- API key stored in environment variable

## Performance

- Zero Go dependencies = fast build times
- Single binary deployment (~10MB)
- Low memory footprint (~20MB runtime)
- Template caching for performance
- CDN-hosted assets reduce bandwidth

## Browser Support

- Chrome/Edge 90+
- Firefox 88+
- Safari 14+
- Mobile browsers supported

## License

See LICENSE file in parent directory.