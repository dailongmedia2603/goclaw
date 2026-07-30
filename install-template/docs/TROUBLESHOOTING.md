# FBM Bundle — Troubleshooting

Lỗi thường gặp + cách sửa. Mỗi vấn đề: **triệu chứng → nguyên nhân → fix 3 bước**.

---

## Lỗi khi install

### ❌ "Docker version X.Y quá cũ (cần ≥ 24.0.0)"

**Nguyên nhân**: Docker engine cũ, không hỗ trợ `pull_policy: never` đúng cách.

**Fix**:
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# macOS: update Docker Desktop từ UI
```

---

### ❌ "Port 29320 đang bị chiếm"

**Nguyên nhân**: Service khác đang nghe port 29320 (sidecar FBM).

**Fix**:
```bash
# Xem ai đang dùng
sudo ss -tlnp | grep :29320    # Linux
sudo lsof -i :29320             # macOS

# Option A: dừng service kia
sudo systemctl stop <service-name>

# Option B: đổi port FBM trong .env.fbm rồi install lại với --force
echo "FBM_PORT=29420" >> /opt/goclaw/.env.fbm
sudo bash install-fbm-bundle.sh --force
```

---

### ❌ "Bundle integrity check FAILED"

**Nguyên nhân**: File bundle corrupt khi tải về.

**Fix**:
```bash
# So sánh checksum
sha256sum -c goclaw-fbm-bundle-v0.1.0.sha256
# → nếu FAIL → tải lại file bundle
```

---

### ❌ "Không tìm thấy GoClaw installation"

**Nguyên nhân**: GoClaw cài ở path không chuẩn.

**Fix**:
```bash
# Truyền đường dẫn trực tiếp
sudo bash install-fbm-bundle.sh --goclaw-dir /your/custom/path

# Hoặc set env var trước
export GOCLAW_DIR=/your/path
sudo -E bash install-fbm-bundle.sh
```

---

### ❌ "range of CPUs is from 0.01 to 1.00, as there are only 1 CPUs available"

**Nguyên nhân**: VPS 1-core nhưng base compose declare `cpus: '2.0'`.

**Fix**: Installer tự xử lý từ version 0.1.0 trở lên. Nếu bạn dùng installer cũ:
```bash
cat > /opt/goclaw/docker-compose.fbm-cpu.yml <<EOF
services:
  goclaw:
    deploy:
      resources:
        limits:
          cpus: "0.95"
  goclaw-ui:
    deploy:
      resources:
        limits:
          cpus: "0.95"
EOF
# Restart với override
cd /opt/goclaw && docker compose ... -f docker-compose.fbm-cpu.yml up -d
```

---

## Lỗi khi chạy

### ❌ fbm-sidecar "unhealthy" hoặc restart loop

**Triệu chứng**:
```
docker ps → fbm-sidecar          Up X minutes (unhealthy)
docker ps → fbm-sidecar          Restarting (1) X seconds ago
```

**Nguyên nhân có thể**:
1. Synapse bên trong chưa boot xong (cần ~60-90s cho lần đầu)
2. Config YAML lỗi
3. Thiếu env vars

**Fix**:
```bash
# 1. Xem logs
docker logs fbm-sidecar --tail 50

# 2. Nếu thấy "Legacy bridge config detected" → bundle version cũ, update bundle

# 3. Nếu thấy "FBM_AUTH_TOKEN required" → .env.fbm thiếu key
sudo bash /path/to/bundle/install/setup-secrets.sh /opt/goclaw --force

# 4. Restart
docker compose -f /opt/goclaw/docker-compose.fbm.yml restart fbm-sidecar

# 5. Chờ 90s cho Synapse boot
sleep 90 && docker ps | grep fbm-sidecar
```

---

### ❌ "signature invalid" trong goclaw logs

**Triệu chứng**: goclaw logs lặp `security.facebook_personal.webhook.signature_failed`.

**Nguyên nhân**: `FBM_HMAC_SECRET` ở sidecar env **KHÔNG KHỚP** với giá trị đã nhập vào UI khi tạo instance channel.

**Fix**:
```bash
# Option A: lấy secret thật từ .env.fbm rồi update UI
sudo cat /opt/goclaw/.env.fbm | grep FBM_HMAC_SECRET
# → copy value, paste vào Channels → edit instance → Webhook HMAC Secret field

# Option B: regenerate + update cả 2 bên
sudo bash setup-secrets.sh /opt/goclaw --force
# (restart sidecar + update UI với secret mới)
```

---

### ❌ "facebook_personal" KHÔNG hiện trong dropdown "Tạo channel"

**Nguyên nhân**: Browser cache UI bundle cũ.

**Fix**:
1. **Hard refresh**: Cmd/Ctrl + Shift + R
2. Nếu vẫn không có: open DevTools → Network tab → tick "Disable cache" → F5
3. Verify backend đã thấy: `docker exec goclaw-goclaw-1 grep facebook_personal /app/data/ -r 2>/dev/null || docker logs goclaw-goclaw-1 | grep facebook_personal`
4. Nếu backend vẫn không có: reinstall bundle với `--force`

---

### ❌ Cookie hết hạn — "Facebook cookies have expired"

**Triệu chứng**: Channel hiện trạng thái "Disconnected" + sidecar logs `401 Unauthorized` từ Facebook.

**Fix**:
1. Vào **Channels** → instance FBM của bạn → **⋯** → **Re-authenticate**
2. Mở messenger.com (incognito), login lại
3. Copy cookies mới (c_user, xs, datr, sb, fr)
4. Paste vào dialog → Login

Nếu không có nút Re-authenticate: bundle version quá cũ — update bundle.

---

### ❌ FB Checkpoint ngay lần login đầu tiên

**Triệu chứng**: Sidecar logs "checkpoint" / "error_subcode: 1348023" ngay sau khi paste cookies.

**Nguyên nhân**:
- VPS datacenter IP (AWS, DO, Hetzner...) → Meta flag ngay
- Account quá mới (< 30 ngày)
- Account có 2FA nhưng mautrix/meta không chạy 2FA flow đúng

**Fix**:
1. **DỪNG NGAY** — không retry login trong 72h từ cùng IP (tăng fingerprint negative)
2. Dùng tài khoản FB khác (aged ≥ 30 ngày)
3. Chuyển sang residential IP (VPN residential, hoặc host tại nhà)
4. Login qua messenger.com từ browser REAL trước → hoạt động bình thường 1 ngày → rồi mới extract cookies

---

### ❌ Sidecar logs "Matrix API status=500" / "/sync error"

**Nguyên nhân**: Synapse crashed hoặc chưa start đúng.

**Fix**:
```bash
# Xem Synapse logs (chạy trong container fbm-sidecar)
docker exec fbm-sidecar tail -100 /data/homeserver.log

# Nếu Synapse DB corrupt → xóa volume + reinstall
docker compose -f /opt/goclaw/docker-compose.fbm.yml down -v
docker volume rm goclaw_fbm-sidecar-data
sudo bash install-fbm-bundle.sh --force
```

---

### ❌ Agent không trả lời DM đến

**Triệu chứng**: Gửi DM từ account khác, sidecar log nhận được message, nhưng agent không respond.

**Fix**:
```bash
# 1. Kiểm tra agent channel policy
# Vào Channels → instance FBM → Config tab
# - DM Policy không nên là "disabled" hoặc "allowlist" nếu chưa thêm sender
# - Đổi sang "open" để test

# 2. Xem goclaw log
docker logs goclaw-goclaw-1 --tail 100 | grep -iE "fbm|facebook_personal"

# 3. Kiểm tra agent có enabled + có agent gán cho channel
# Channels → instance → Agent field (phải không trống)
```

---

### ❌ Installer báo "FBM bundle đã cài" dù tôi muốn reinstall

**Fix**:
```bash
sudo bash install-fbm-bundle.sh --force
```

---

### ❌ `uninstall-fbm-bundle.sh` báo "cannot restore override"

**Nguyên nhân**: Backup file `.bak-fbm-*` bị xóa.

**Fix**:
```bash
# Viết lại override.yml từ đầu (chỉ giữ phần resources limits nếu bạn muốn)
sudo rm /opt/goclaw/docker-compose.override.yml
# → GoClaw dùng upstream defaults
cd /opt/goclaw
docker compose -f docker-compose.yml -f docker-compose.postgres.yml \
  -f docker-compose.selfservice.yml up -d goclaw goclaw-ui
```

---

### ❌ Sau `docker compose pull`, FBM "biến mất"

**Nguyên nhân**: Hiếm gặp — thường `pull_policy: never` giữ nguyên fork images. Nếu thấy triệu chứng này, có thể override file đã bị overwritten.

**Fix**:
```bash
# 1. Check status
sudo bash install/fbm-check-upgrade.sh /opt/goclaw

# 2. Nếu báo Errors → reinstall bundle
sudo bash install-fbm-bundle.sh --force
```

---

### ❌ "Cannot reach sidecar at http://fbm-sidecar:29320"

**Nguyên nhân**: Container `goclaw-goclaw-1` và `fbm-sidecar` không cùng network.

**Fix**:
```bash
# Kiểm tra network
docker network ls | grep goclaw
docker inspect fbm-sidecar | grep -A5 Networks

# Nếu fbm-sidecar không trong "goclaw-net":
docker network connect goclaw-net fbm-sidecar
```

---

## Lỗi khi upgrade

### ❌ `fbm-check-upgrade.sh` exit 2

**Xử lý**: Xem [UPGRADE-GUIDE.md](UPGRADE-GUIDE.md).

---

## Debug commands nhanh

```bash
# Trạng thái tổng quan
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}" | grep -E "goclaw|fbm"

# Logs 3 container
docker logs goclaw-goclaw-1 --tail 50
docker logs goclaw-goclaw-ui-1 --tail 20
docker logs fbm-sidecar --tail 50

# Health endpoint
source /opt/goclaw/.env.fbm
curl -sI -H "Authorization: Bearer $FBM_AUTH_TOKEN" http://localhost:29320/healthz

# Full diagnostic
sudo /path/to/bundle/bin/fbm-diagnose

# Channel instances in DB
docker exec goclaw-postgres-1 psql -U goclaw -d goclaw \
  -c "SELECT name, channel_type, enabled FROM channel_instances WHERE channel_type='facebook_personal';"
```

---

## Không tìm thấy lỗi bạn gặp?

1. Copy log từ cả 3 container (goclaw, goclaw-ui, fbm-sidecar) — tail 100 dòng
2. Chạy `fbm-diagnose --json > diag.json`
3. Gửi `diag.json` + logs cho người cung cấp bundle
