# FBM — Upgrade Guide

Hướng dẫn xử lý khi upstream GoClaw ra version mới.

---

## Nguyên lý: fork images "dính" qua `docker compose pull`

Bundle FBM dùng `pull_policy: never` cho 2 images:
- `goclaw-fork:X.Y.Z`  — backend có code FBM
- `goclaw-web-fork:X.Y.Z` — UI có dropdown FBM

Khi bạn chạy `docker compose pull`:
- Các image upstream khác (`goclaw-postgres`, `cloakbrowser-*`) được pull mới
- `goclaw-fork` + `goclaw-web-fork` **KHÔNG bị đụng tới** — vẫn là fork images bạn đã load từ bundle

**Kết quả**: Upgrade upstream GoClaw bình thường, FBM tiếp tục chạy **KHÔNG CẦN LÀM GÌ** trong hầu hết trường hợp.

---

## Decision flowchart

```
Bạn vừa chạy `docker compose pull` hoặc goclaw auto-update?
              │
              ▼
    Chạy: fbm-check-upgrade.sh
              │
       ┌──────┼──────┐
       │      │      │
     exit 0  exit 1  exit 2
       │      │      │
       │      │      └── ❗ Cần rebuild bundle (xem Scenario C)
       │      │
       │      └──── ⚠ Có warnings, xem xét (xem Scenario B)
       │
       └──── ✅ Không cần làm gì (Scenario A)
```

---

## Scenario A — Không cần làm gì (99% trường hợp)

Sau `docker compose pull`, chạy:

```bash
sudo bash /path/to/bundle/install/fbm-check-upgrade.sh /opt/goclaw
```

Output:
```
✅ FBM bundle healthy. Không cần hành động.
```

→ Xong. FBM tiếp tục chạy với fork images cũ, các feature khác đã được update.

---

## Scenario B — Warning: compose drift

Output `fbm-check-upgrade.sh` hiện:
```
⚠  Upstream compose files đã thay đổi từ lúc cài FBM (2026-04-18T02:14:00Z)
→ Khuyến nghị: rebuild bundle từ upstream mới nhất, rồi install --force
```

**Nghĩa là**: Upstream version mới có thay đổi format compose. FBM vẫn chạy nhưng có thể gặp vấn đề khi restart. Không urgent, nhưng nên rebuild khi có thời gian.

**Cách xử lý**:

### Option B1: Lấy bundle mới từ người cung cấp

1. Liên hệ người gửi bundle cho bạn
2. Yêu cầu bundle mới build against upstream version hiện tại
3. Tải bundle → install với `--force`:
```bash
tar xzf goclaw-fbm-bundle-v0.2.0.tar.gz
cd goclaw-fbm-bundle-v0.2.0/install
sudo bash install-fbm-bundle.sh --force
```

### Option B2: Tự build từ source tarball

Nếu bạn có source tarball + upstream source:
```bash
tar xzf goclaw-fbm-source-v0.2.0.tar.gz
cd goclaw-fbm-source-v0.2.0
bash install/build-fbm-from-source.sh \
  --upstream-dir /opt/goclaw/.src \
  --version 0.2.0
# Sau đó reinstall bundle với images mới build
sudo bash install/install-fbm-bundle.sh --force
```

---

## Scenario C — Error: cần rebuild ngay

Output `fbm-check-upgrade.sh` hiện:
```
❌ Container goclaw-goclaw-1 uses image 'ghcr.io/nextlevelbuilder/goclaw:latest'
    but FBM expects 'goclaw-fork:0.1.0'
```

**Nghĩa là**: Compose pull với option `--pull always` đã overwrite image fork, hoặc có thay đổi config khiến Docker recreate container với image upstream. FBM **KHÔNG chạy** hiện tại.

**Fix nhanh**:
```bash
sudo bash install-fbm-bundle.sh --force
```

(Installer sẽ re-apply override + restart containers với fork images. Downtime ~10-20s.)

Nếu fail:
```bash
# Verify fork images vẫn tồn tại
docker images | grep -E "goclaw-fork|goclaw-web-fork"

# Nếu missing → cần reload từ bundle tar
zstd -dc /path/to/bundle/images/goclaw-fork.tar.zst | docker load
zstd -dc /path/to/bundle/images/goclaw-web-fork.tar.zst | docker load

# Sau đó install lại
sudo bash install-fbm-bundle.sh --force
```

---

## Rollback: quay về upstream thuần, không có FBM

Nếu muốn tắt FBM tạm thời (hoặc test xem upstream clean có bug không):

```bash
sudo bash /path/to/bundle/install/uninstall-fbm-bundle.sh /opt/goclaw
# GoClaw khởi động lại với upstream images. FBM channel biến mất khỏi dropdown.
```

Để reinstall sau đó:
```bash
sudo bash /path/to/bundle/install/install-fbm-bundle.sh
# Secret file /opt/goclaw/.env.fbm.uninstalled-* sẽ được auto-tìm lại (nếu có)
```

---

## Best practices cho upgrade window

**Trước khi upgrade upstream GoClaw:**
1. Snapshot instance channel config từ UI (screenshot)
2. Verify `.env.fbm` không bị xóa: `ls -la /opt/goclaw/.env.fbm`
3. Backup `docker-compose.override.yml` hiện tại (installer sẽ tự backup nhưng thêm 1 lớp an toàn)

**Sau khi upgrade:**
1. `sudo bash fbm-check-upgrade.sh /opt/goclaw`
2. Nếu cần rebuild: theo Scenario B hoặc C
3. Test: gửi DM từ account khác → agent reply OK?

**Cadence khuyến nghị**:
- Upstream **patch release** (v3.9.x → v3.9.x+1): không cần đụng FBM
- Upstream **minor release** (v3.9.x → v3.10.x): check script; thường không sao
- Upstream **major release** (v3.x → v4.x): **CHỜ bundle mới** trước khi upgrade — có thể break interface

---

## Upgrade bundle FBM (khi có version mới)

```bash
# 1. Tải bundle mới + checksum
# 2. Verify
sha256sum -c goclaw-fbm-bundle-v0.2.0.sha256

# 3. Extract
tar xzf goclaw-fbm-bundle-v0.2.0.tar.gz

# 4. Force install (idempotent)
cd goclaw-fbm-bundle-v0.2.0/install
sudo bash install-fbm-bundle.sh --force

# 5. Verify
sudo bash fbm-check-upgrade.sh /opt/goclaw

# 6. (Tuỳ chọn) Purge bundle cũ
docker rmi goclaw-fork:0.1.0 goclaw-web-fork:0.1.0 fbm-sidecar:0.1.0 2>/dev/null || true
```

`.env.fbm` KHÔNG bị overwrite (installer preserve). Instance channel config trong DB không bị mất.

---

## Nếu cần bundle mới build against upstream cụ thể

Gửi người cung cấp bundle thông tin:
```bash
# Upstream version hiện tại trên máy bạn
docker exec goclaw-goclaw-1 goclaw version 2>/dev/null || \
  docker image inspect ghcr.io/nextlevelbuilder/goclaw:latest --format '{{index .Config.Labels "org.opencontainers.image.version"}}'

# Compose hash
cat /opt/goclaw/docker-compose.yml /opt/goclaw/docker-compose.postgres.yml /opt/goclaw/docker-compose.selfservice.yml | sha256sum
```

Để họ build bundle tương thích.

---

## Liên hệ hỗ trợ

Các trường hợp khác, gửi logs + output `fbm-diagnose --json` cho người cung cấp bundle.
