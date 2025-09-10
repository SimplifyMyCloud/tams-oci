#!/bin/bash
set -e

DB_HOST="${DB_HOST}"
DB_PASSWORD="${DB_PASSWORD}"
MEDIA_BUCKET="${MEDIA_BUCKET}"
ARCHIVE_BUCKET="${ARCHIVE_BUCKET}"
TEMP_BUCKET="${TEMP_BUCKET}"
NAMESPACE="${NAMESPACE}"
REGION="${REGION}"

sudo apt-get update
sudo apt-get install -y docker.io docker-compose nginx certbot python3-certbot-nginx

sudo systemctl enable docker
sudo systemctl start docker

sudo usermod -aG docker ubuntu

mkdir -p /home/ubuntu/tams-api
cd /home/ubuntu/tams-api

cat <<EOF > docker-compose.yml
version: '3.8'
services:
  tams-api:
    image: tams-api:latest
    restart: always
    environment:
      - DB_HOST=${DB_HOST}
      - DB_PORT=5432
      - DB_NAME=tamsdb
      - DB_USER=tamsapi
      - DB_PASSWORD=${DB_PASSWORD}
      - OCI_NAMESPACE=${NAMESPACE}
      - OCI_REGION=${REGION}
      - MEDIA_BUCKET=${MEDIA_BUCKET}
      - ARCHIVE_BUCKET=${ARCHIVE_BUCKET}
      - TEMP_BUCKET=${TEMP_BUCKET}
      - API_PORT=8080
    ports:
      - "8080:8080"
    volumes:
      - /var/log/tams:/app/logs
      - /home/ubuntu/.oci:/root/.oci:ro
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 3s
      retries: 3
EOF

cat <<'NGINX_CONFIG' | sudo tee /etc/nginx/sites-available/tams-api
upstream tams_backend {
    server 127.0.0.1:8080;
}

server {
    listen 80;
    server_name _;
    
    client_max_body_size 5G;
    
    location / {
        proxy_pass http://tams_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 600;
        proxy_send_timeout 600;
        proxy_read_timeout 600;
        send_timeout 600;
    }
    
    location /health {
        proxy_pass http://tams_backend/health;
        access_log off;
    }
}
NGINX_CONFIG

sudo ln -s /etc/nginx/sites-available/tams-api /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default
sudo nginx -t
sudo systemctl restart nginx
sudo systemctl enable nginx

sudo apt-get install -y python3-pip
pip3 install oci-cli

mkdir -p /home/ubuntu/.oci

cat <<'MONITORING_SCRIPT' | sudo tee /usr/local/bin/monitor_tams.sh
#!/bin/bash
HEALTH_URL="http://localhost:8080/health"
MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -f -s $HEALTH_URL > /dev/null; then
        echo "TAMS API is healthy"
        exit 0
    else
        echo "TAMS API health check failed, attempt $((RETRY_COUNT + 1))"
        RETRY_COUNT=$((RETRY_COUNT + 1))
        sleep 10
    fi
done

echo "TAMS API is unhealthy, restarting..."
cd /home/ubuntu/tams-api && docker-compose restart tams-api
MONITORING_SCRIPT

sudo chmod +x /usr/local/bin/monitor_tams.sh

echo "*/5 * * * * /usr/local/bin/monitor_tams.sh" | sudo crontab -

echo "TAMS API initialization complete"