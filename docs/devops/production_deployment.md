# Production Deployment & Operations Guide

**Document Type:** Production Readiness, Infrastructure, & Operations Guide
**Target Audience:** DevOps Engineers, System Administrators, and Backend Developers
**Environments:** NAT VPS / Cloud VPS (Ubuntu/Debian), Docker Compose, GitHub Actions (CI/CD), Cloudflare Tunnel (`cloudflared`), Cloudflare R2 / AWS S3, Resend

---

## 1. System Architecture Overview

```text
[ Pengunjung Internet ]
          │ (HTTPS Port 443 / SSL Terminated)
          ▼
[ Cloudflare Global Edge ] (DDoS Shield, WAF, Global CDN)
          │ (Encrypted Tunnel / Zero Open Ports on VPS)
          ▼
[ NAT VPS: Container `cloudflared` ]
          │ (Internal Docker Network: `mmdbconnect_backend`)
          ▼
[ NAT VPS: Container `mmdbconnect-go-service` ] (Port 8080, User: appuser)
          │ (Internal Connection: host `mysql`, port 3306)
          ▼
[ NAT VPS: Container `mmdbconnect-go-service-mysql` ] (Port 3306, Volume: `mysql_data`)
```

---

## 2. Production Readiness Checklist

Before exposing the API to public traffic, ensure the following items are verified:

- [ ] **Generate High-Entropy Secret Key**: Set `JWT_SECRET` to a 64+ character random hex string (`openssl rand -hex 32` minimum 32 characters enforced by application fail-fast validation in `APP_ENV=production`).
- [ ] **Enforce Allowed Origins**: Set `ALLOWED_ORIGINS` to your production frontend domain (e.g., `https://mmdbconnect.site`). Never leave it wildcard in production.
- [ ] **Switch Mailer Driver**: Set `MAILER_DRIVER=resend` with verified domain credentials, or keep `MAILER_DRIVER=log` if transactional emails are not yet required.
- [ ] **Switch Storage Driver**: Set `STORAGE_DRIVER=r2` or `s3` with cloud object storage credentials, or `local` using persistent Docker volumes.
- [ ] **Cookie Security**: Set `COOKIE_SECURE=true` because traffic passes through Cloudflare HTTPS.
- [ ] **Disable `-fresh` Flag**: Never pass `-fresh` in production commands as it will drop all existing tables and trigger irreversible data loss.

---

## 3. Environment Variables Reference

| Variable                        | Description                               | Development Default      | Production Recommended                          |
| :------------------------------ | :---------------------------------------- | :----------------------- | :---------------------------------------------- |
| `PORT`                          | HTTP port the server listens on           | `8080`                   | `8080` (behind reverse proxy / tunnel)          |
| `APP_ENV`                       | Application runtime environment           | `development`            | `production`                                    |
| `LOG_LEVEL`                     | Logging severity filter                   | `debug`                  | `info`                                          |
| `LOG_FORMAT`                    | Structured log format                     | `text`                   | `json` (standard structured logging)            |
| `FRONTEND_URL`                  | Base URL used for email action links      | `http://localhost:3000`  | `https://mmdbconnect.site`                      |
| `DB_HOST`                       | MySQL hostname or Docker service name     | `127.0.0.1`              | `mysql` (Docker network service name)           |
| `DB_PORT`                       | MySQL connection port                     | `3306`                   | `3306`                                          |
| `DB_USER`                       | MySQL database user                       | `root`                   | `mmdbconnect_go_service`                        |
| `DB_PASSWORD`                   | MySQL user password                       | `""`                     | Strong entropy password                         |
| `DB_NAME`                       | MySQL database schema name                | `mmdbconnect_go_service` | `mmdbconnect_go_service`                        |
| `DB_MAX_OPEN_CONNS`             | Maximum open database connections         | `100`                    | `25` - `50` (based on VPS RAM)                  |
| `DB_MAX_IDLE_CONNS`             | Maximum idle database connections         | `25`                     | `10` - `25`                                     |
| `DB_CONN_MAX_LIFETIME_MINUTES`  | Connection maximum lifetime duration      | `60`                     | `60`                                            |
| `DB_CONN_MAX_IDLE_TIME_MINUTES` | Connection idle timeout duration          | `10`                     | `10`                                            |
| `COOKIE_SECURE`                 | Enable Secure flag on HTTP cookies        | `false`                  | `true` (enforces HTTPS)                         |
| `COOKIE_DOMAIN`                 | Domain attribute for cookies              | `""`                     | `""` or `.mmdbconnect.site`                     |
| `ADMIN_NAME`                    | Initial administrator display name        | `Administrator`          | Your Name / Display Admin                       |
| `ADMIN_EMAIL`                   | Initial administrator login email         | `admin@example.com`      | `contact.mochammadbagas@gmail.com`              |
| `ADMIN_PASSWORD`                | Initial administrator password            | `adminpassword123`       | Strong 16+ char password                        |
| `JWT_SECRET`                    | HMAC SHA-256 signing key                  | `super-secret-key...`    | `openssl rand -hex 32` (>= 32 chars)            |
| `JWT_ACCESS_TTL_MINUTES`        | Access token lifespan                     | `15`                     | `15`                                            |
| `JWT_REFRESH_TTL_DAYS`          | Refresh token lifespan                    | `7`                      | `7`                                             |
| `ALLOWED_ORIGINS`               | Comma-separated allowed origins           | `http://localhost:3000`  | `https://mmdbconnect.site`                      |
| `MAILER_DRIVER`                 | Active email driver (`log` / `resend`)    | `log`                    | `resend` (or `log`)                             |
| `RESEND_API_KEY`                | Resend REST API authentication key        | `""`                     | `re_xxxxxxxxxxxx`                               |
| `MAIL_FROM`                     | RFC 5322 sender display and address       | `no-reply@localhost`     | `mmdbconnect <no-reply@mail.mmdbconnect.site>`  |
| `STORAGE_DRIVER`                | Active storage driver (`local`/`r2`/`s3`) | `local`                  | `r2` or `local`                                 |
| `UPLOAD_DIR`                    | Local disk directory for uploads          | `./uploads`              | `/app/uploads`                                  |
| `S3_BUCKET`                     | S3 / R2 bucket name                       | `""`                     | `mmdbconnect-bucket`                            |
| `S3_REGION`                     | S3 region code (`auto` for R2)            | `auto`                   | `auto`                                          |
| `S3_ENDPOINT`                   | Custom endpoint URL for Cloudflare R2     | `""`                     | `https://<account_id>.r2.cloudflarestorage.com` |
| `S3_ACCESS_KEY_ID`              | Cloudflare R2 / AWS Access Key ID         | `""`                     | Your R2 Access Key                              |
| `S3_SECRET_ACCESS_KEY`          | Cloudflare R2 / AWS Secret Access Key     | `""`                     | Your R2 Secret Key                              |
| `S3_PUBLIC_URL`                 | Public CDN base URL for uploaded files    | `""`                     | `https://media.mmdbconnect.site`                |

---

## 4. GitHub Actions CI/CD Pipeline Setup

Sistem deployment diotomatisasi penuh menggunakan GitHub Actions dan GitHub Container Registry (GHCR):

- **CI (`.github/workflows/ci.yml`)**: Berjalan saat Pull Request ke `develop` / `main`. Memverifikasi kerapian `go.mod`, menjalankan seluruh unit test (`go test`), dan menguji kompilasi multi-stage `Dockerfile`.
- **CD (`.github/workflows/cd.yml`)**: Berjalan saat push/merge ke `main` atau rilis tag `v*`. Mem-build Docker image ke `ghcr.io/mochammadbagas/mmdbconnect-go-service:latest`, lalu melakukan remote SSH ke NAT VPS untuk me-refresh container secara instan tanpa downtime database.

### Repository Secrets Configuration:

Daftarkan 4 secret ini di GitHub (**Settings $\rightarrow$ Secrets and variables $\rightarrow$ Actions**):

| Secret Name   | Deskripsi                                                |
| :------------ | :------------------------------------------------------- |
| `VPS_HOST`    | IP Publik Shared NAT VPS Anda                            |
| `VPS_PORT`    | Port Forwarding SSH NAT VPS Anda (misal: `28451`)        |
| `VPS_USER`    | Username SSH login VPS Anda (biasanya `root`)            |
| `VPS_SSH_KEY` | Kunci Privat SSH khusus deploy (`~/.ssh/vps_deploy_key`) |

### Membuat Kunci SSH Khusus Deployment (Laptop):

```bash
ssh-keygen -t ed25519 -f "$HOME/.ssh/vps_deploy_key" -C "github-actions-deploy"
```

- Pasang isi `vps_deploy_key.pub` ke file `~/.ssh/authorized_keys` di VPS.
- Masukkan isi `vps_deploy_key` ke GitHub Secret `VPS_SSH_KEY`.

---

## 5. NAT VPS Setup & Docker Operations (`/opt/mmdbconnect`)

Di server VPS Anda, seluruh project dijalankan di dalam satu direktori terisolasi:

```bash
mkdir -p /opt/mmdbconnect
cd /opt/mmdbconnect
```

Di dalam `/opt/mmdbconnect/`, buat 3 file:

### 1. `compose.yaml` (Production)

```yaml
services:
  mysql:
    image: mysql:8.4
    container_name: mmdbconnect-go-service-mysql
    restart: unless-stopped
    env_file:
      - .env.mysql
    volumes:
      - mysql_data:/var/lib/mysql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "--silent"]
      interval: 5s
      timeout: 5s
      retries: 20
    networks:
      - backend

  api:
    image: ghcr.io/mochammadbagas/mmdbconnect-go-service:latest
    container_name: mmdbconnect-go-service
    restart: unless-stopped
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
    env_file:
      - .env.docker
    depends_on:
      mysql:
        condition: service_healthy
    ports:
      - "8080:8080"
    volumes:
      - uploads_data:/app/uploads
    networks:
      - backend

volumes:
  mysql_data:
  uploads_data:

networks:
  backend:
    driver: bridge
```

### 2. `.env.mysql`

```ini
MYSQL_RANDOM_ROOT_PASSWORD=yes
MYSQL_DATABASE=mmdbconnect_go_service
MYSQL_USER=mmdbconnect_go_service
MYSQL_PASSWORD=GantiDenganPasswordDatabaseKuat
```

### 3. `.env.docker`

```ini
PORT=8080
APP_ENV=production
LOG_LEVEL=info
LOG_FORMAT=json

DB_HOST=mysql
DB_PORT=3306
DB_USER=mmdbconnect_go_service
DB_PASSWORD=GantiDenganPasswordDatabaseKuat
DB_NAME=mmdbconnect_go_service
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME_MINUTES=60
DB_CONN_MAX_IDLE_TIME_MINUTES=10

ADMIN_NAME=Administrator
ADMIN_EMAIL=contact.mochammadbagas@gmail.com
ADMIN_PASSWORD=GantiDenganPasswordAdminKuat

COOKIE_SECURE=true
COOKIE_DOMAIN=
UPLOAD_DIR=/app/uploads
ALLOWED_ORIGINS=https://mmdbconnect.site

JWT_SECRET=GantiDenganHexAcakMin32Karakter
JWT_ACCESS_TTL_MINUTES=15
JWT_REFRESH_TTL_DAYS=7

MAILER_DRIVER=log
FRONTEND_URL=https://mmdbconnect.site

STORAGE_DRIVER=local
```

Kunci izin keamanan file `.env`:

```bash
chmod 600 .env.docker .env.mysql
```

### Cold Start (Pertama Kali Dinyalakan di VPS):

```bash
docker compose up -d
```

_(MySQL akan menyala, diverifikasi oleh healthcheck, skema tabel dimigrasikan otomatis, dan data awal di-seed otomatis oleh Go binary)._

---

## 6. Cloudflare Tunnel & Reverse Proxy

### A. Menggunakan Cloudflare Tunnel (Direkomendasikan untuk NAT VPS)

Karena NAT VPS tidak memiliki port 80/443 publik langsung, Cloudflare Tunnel menghubungkan container ke internet melalui koneksi terenkripsi keluar (_outbound_).

1. Pastikan container `cloudflared` terhubung ke jaringan Docker project:
   ```bash
   docker network connect mmdbconnect_backend cloudflared
   ```
2. Di dashboard **Cloudflare Zero Trust** ([one.dash.cloudflare.com](https://one.dash.cloudflare.com)):
   - Buka **Networks** $\rightarrow$ **Tunnels** $\rightarrow$ Pilih Tunnel $\rightarrow$ **Public Hostnames**.
   - Tambahkan hostname:
     - **Subdomain:** `api`
     - **Domain:** `mmdbconnect.site`
     - **Type:** `HTTP`
     - **URL:** `api:8080`

### B. Alternatif: Nginx (Jika Menggunakan Dedicated Public IP)

```nginx
server {
    server_name api.mmdbconnect.site;
    client_max_body_size 10M;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    listen 443 ssl;
    # SSL directives...
}
```

---

## 7. Observability, Health Checks & Monitoring

The application provides three specialized probes:

- **Deep Health Check**: `GET /health` mengeksekusi `db.PingContext` dengan timeout 2 detik. Mengembalikan HTTP 200 `{"status":"ok","database":"healthy"}` jika normal, atau HTTP 503 jika database terputus.
- **Liveness Probe**: `GET /health/live` mengembalikan HTTP 200 `{"status":"live"}` instan untuk memverifikasi container Go masih hidup.
- **Readiness Probe**: `GET /health/ready` memvalidasi kesiapan database untuk melayani trafik sebelum ingress routing dibuka.

### Perintah Pemeliharaan Cepat di VPS:

```bash
# Cek container aktif
docker ps

# Lihat live log aplikasi Go
docker compose -f /opt/mmdbconnect/compose.yaml logs -f api

# Lihat live log MySQL
docker compose -f /opt/mmdbconnect/compose.yaml logs -f mysql

# Test endpoint lokal
curl -i http://localhost:8080/health

# Bersihkan Docker cache image lama
docker image prune -f
```
