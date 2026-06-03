#!/bin/bash
# DNS.1 部署脚本 — 腾讯云 Ubuntu / CentOS
set -e

APP_DIR="/opt/dns-pc"
DOMAIN="dnsway.asia"

echo "=== 1. 创建应用目录 ==="
mkdir -p $APP_DIR/data
cp dns-pc-linux $APP_DIR/
cp -r templates $APP_DIR/
cp -r static $APP_DIR/
cp data/state.json $APP_DIR/data/

echo "=== 2. 创建环境变量 ==="
cat > $APP_DIR/.env << 'EOF'
PORT=8081
DNS_PORT=8053
BASE_URL=https://dnsway.asia
JWT_SECRET=dns1-jwt-secret-$(openssl rand -hex 16)
DATABASE_URL=postgres://dnspc:dnspc@localhost:5432/dnspc?sslmode=disable
EOF

# Generate random JWT secret
sed -i "s/\$(openssl rand -hex 16)/$(openssl rand -hex 16)/" $APP_DIR/.env

echo "=== 3. 创建 systemd 服务 ==="
cat > /etc/systemd/system/dns-pc.service << 'SERVICEEOF'
[Unit]
Description=DNS.1 Parental Control
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/dns-pc
ExecStart=/opt/dns-pc/dns-pc-linux
Restart=always
RestartSec=5
EnvironmentFile=/opt/dns-pc/.env

[Install]
WantedBy=multi-user.target
SERVICEEOF

echo "=== 4. 安装 Nginx + SSL ==="
apt-get update && apt-get install -y nginx certbot python3-certbot-nginx

cat > /etc/nginx/sites-available/dns-pc << 'NginxEOF'
server {
    listen 80;
    server_name dnsway.asia;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name dnsway.asia;

    ssl_certificate /etc/letsencrypt/live/dnsway.asia/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/dnsway.asia/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/v1/stripe/webhook {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_buffering off;
    }
}
NginxEOF

ln -sf /etc/nginx/sites-available/dns-pc /etc/nginx/sites-enabled/
rm -f /etc/nginx/sites-enabled/default

echo "=== 5. 申请 SSL 证书 ==="
certbot --nginx -d $DOMAIN --non-interactive --agree-tos --email admin@$DOMAIN || echo "请手动运行: certbot --nginx -d $DOMAIN"

echo "=== 6. 启动服务 ==="
systemctl daemon-reload
systemctl enable dns-pc
systemctl start dns-pc
systemctl reload nginx

echo ""
echo "✅ 部署完成！"
echo "   访问: https://$DOMAIN"
echo "   查看状态: systemctl status dns-pc"
echo "   查看日志: journalctl -u dns-pc -f"
EOF
