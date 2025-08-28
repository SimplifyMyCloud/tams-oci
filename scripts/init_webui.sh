#!/bin/bash
set -e

API_URL="${API_URL}"
INTERNAL_API_URL="${INTERNAL_API_URL}"
WEB_DOMAIN="${WEB_DOMAIN}"

sudo apt-get update
sudo apt-get install -y nginx certbot python3-certbot-nginx golang-go git make

cd /opt
git clone https://github.com/your-org/tams-web-ui.git || {
    mkdir -p tams-web-ui
    cd tams-web-ui
    
    cat > main.go << 'EOF'
package main
import "net/http"
func main() {
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("OK"))
    })
    http.ListenAndServe(":8090", nil)
}
EOF
}

cd /opt/tams-web-ui

export TAMS_API_URL="$INTERNAL_API_URL"
export PORT=8090

if [ -f "go.mod" ]; then
    go build -o tams-web-ui main.go
else
    go mod init tams-web-ui
    go build -o tams-web-ui main.go
fi

sudo tee /etc/systemd/system/tams-webui.service > /dev/null << EOF
[Unit]
Description=TAMS Web UI
After=network.target

[Service]
Type=simple
User=www-data
Group=www-data
WorkingDirectory=/opt/tams-web-ui
Environment="PORT=8090"
Environment="TAMS_API_URL=$INTERNAL_API_URL"
ExecStart=/opt/tams-web-ui/tams-web-ui
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable tams-webui
sudo systemctl start tams-webui

sudo tee /etc/nginx/sites-available/tams-webui > /dev/null << 'NGINX_CONFIG'
server {
    listen 80;
    server_name _;
    
    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name _;
    
    ssl_certificate /etc/ssl/certs/ssl-cert-snakeoil.pem;
    ssl_certificate_key /etc/ssl/private/ssl-cert-snakeoil.key;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Content-Security-Policy "default-src 'self' https:; script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://cdnjs.cloudflare.com; style-src 'self' 'unsafe-inline' https://cdnjs.cloudflare.com" always;
    
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    location /api/ {
        deny all;
        return 403;
    }
    
    location /health {
        proxy_pass http://127.0.0.1:8090/health;
        access_log off;
    }
}
NGINX_CONFIG

sudo ln -sf /etc/nginx/sites-available/tams-webui /etc/nginx/sites-enabled/
sudo rm -f /etc/nginx/sites-enabled/default

sudo nginx -t
sudo systemctl restart nginx
sudo systemctl enable nginx

if [ ! -z "$WEB_DOMAIN" ]; then
    sudo certbot --nginx -d $WEB_DOMAIN --non-interactive --agree-tos --email admin@$WEB_DOMAIN || true
fi

sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 22/tcp
sudo ufw --force enable

cat > /usr/local/bin/health_check.sh << 'EOF'
#!/bin/bash
if ! curl -f -s http://localhost:8090/health > /dev/null; then
    systemctl restart tams-webui
fi
EOF

sudo chmod +x /usr/local/bin/health_check.sh
echo "*/5 * * * * /usr/local/bin/health_check.sh" | sudo crontab -

echo "Web UI initialization complete"