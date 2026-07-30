# FBM Bundle Installer — Hướng Dẫn Nhanh

Bundle thêm kênh **Facebook Messenger (Personal)** vào GoClaw hiện có của bạn. Sau khi cài, dropdown "Tạo phiên bản channel" sẽ có thêm entry **"Facebook Messenger (Personal)"**.

---

## ⚠️ Đọc trước khi cài

> Kênh này sử dụng automation **không chính thức** cho Facebook Messenger cá nhân.
>
> - **Vi phạm ToS Meta** §3.2.3
> - **Tài khoản Facebook CÓ THỂ BỊ KHÓA VĨNH VIỄN** — không phục hồi được
> - **KHÔNG dùng cho tài khoản chính** — chỉ dùng tài khoản test/burn đã hoạt động ≥ 30 ngày
> - **Không có SLA, không có bảo hành**
> - Nếu VPS bạn dùng có datacenter IP (AWS, DigitalOcean, v.v.) → rủi ro ban rất cao. Khuyến nghị residential IP.
> - Bundle embed **mautrix/meta (AGPL-3.0)** — nếu phân phối tiếp, phải tuân thủ AGPL §13.

Nếu bạn đồng ý các rủi ro trên, tiếp tục.

---

## 1. Yêu cầu hệ thống

| Thành phần | Tối thiểu |
|------------|-----------|
| OS | Linux (Ubuntu 22.04+, Debian 12+) hoặc macOS với Docker Desktop |
| Docker | ≥ 24.0 |
| Docker Compose | ≥ 2.20 (v2) |
| RAM | ≥ 2 GB free |
| Disk | ≥ 3 GB free |
| GoClaw đang chạy | qua `docker-compose` (thường ở `/opt/goclaw`) |
| Ports free | 29319 và 29320 |
| Tài khoản FB test | aged ≥ 30 ngày, có avatar + vài post + friends |

Kiểm tra nhanh:

```bash
docker version --format '{{.Server.Version}}'     # cần ≥ 24.0
docker compose version --short                     # cần ≥ 2.20
df -h /opt/goclaw | tail -1                        # cần ≥ 3G free
free -m | awk '/Mem:/ {print $7 "MB available"}'   # cần ≥ 2048
```

---

## 2. Cài đặt — 5 bước

### Bước 1. Verify bundle integrity

```bash
cd /path/to/downloaded/
sha256sum -c goclaw-fbm-bundle-v0.1.0.sha256
# Expected: goclaw-fbm-bundle-v0.1.0.tar.gz: OK
```

Nếu lỗi → file tải bị corrupt, tải lại.

### Bước 2. Extract bundle

```bash
tar xzf goclaw-fbm-bundle-v0.1.0.tar.gz
cd goclaw-fbm-bundle-v0.1.0/install
```

### Bước 3. Chạy installer

```bash
sudo bash install-fbm-bundle.sh
```

Installer sẽ tự động:
- Phát hiện GoClaw (thường `/opt/goclaw`)
- Pre-flight checks (Docker, disk, RAM, ports)
- Backup `docker-compose.override.yml` (nếu có)
- Load 3 Docker images (~400 MB)
- Tạo secrets: `/opt/goclaw/.env.fbm` (chmod 600)
- Thêm `docker-compose.fbm.yml` overlay
- Restart `goclaw` + `goclaw-ui`, start `fbm-sidecar`
- Chạy health check

Thời gian: **~3 phút** nếu mọi thứ OK.

Nếu muốn chạy không hỏi (CI): thêm `--skip-interactive`.
Nếu muốn xem sẽ làm gì mà không thực thi: thêm `--dry-run`.

### Bước 4. Mở UI + tạo instance channel

1. Mở trình duyệt → URL GoClaw (thường `http://localhost:3000` hoặc domain của bạn)
2. **Hard refresh** trình duyệt để load UI bundle mới:
   - Chrome/Edge: **Cmd+Shift+R** (Mac) / **Ctrl+Shift+R** (Windows)
   - Firefox: **Cmd+Shift+R**
3. Vào **Channels** → **Tạo phiên bản channel**
4. Chọn **"Facebook Messenger (Personal)"** từ dropdown
5. Điền thông tin xác thực:

   | Trường | Lấy từ đâu |
   |---|---|
   | Sidecar URL | `http://fbm-sidecar:29320` |
   | Sidecar Auth Token | File `/opt/goclaw/.env.fbm`, dòng `FBM_AUTH_TOKEN=...` |
   | Webhook HMAC Secret | File `/opt/goclaw/.env.fbm`, dòng `FBM_HMAC_SECRET=...` |

   Lấy giá trị nhanh bằng:
   ```bash
   sudo cat /opt/goclaw/.env.fbm | grep -E '^(FBM_AUTH_TOKEN|FBM_HMAC_SECRET)='
   ```

6. Tab **Cấu hình**:
   - **Account Label**: đặt tên dễ nhớ (VD: `Alice Test FB`)
   - **DM Policy**: `open` cho test, `pairing` cho production
   - **Rate limit**: để mặc định 20 msg/phút
   - **Tích vào ô "I acknowledge the risks"** (bắt buộc)
7. Submit → wizard sẽ hiện dialog yêu cầu **FB cookies**

### Bước 5. Đăng nhập Facebook bằng cookies

**⚠️ Chỉ dùng tài khoản test/burn!**

1. Mở `https://www.messenger.com` trong cửa sổ **incognito/private**
2. Đăng nhập bằng tài khoản FB **test** (KHÔNG phải main account)
3. Nhấn **F12** (DevTools) → tab **Application** (Chrome) / **Storage** (Firefox)
4. Mở **Cookies** → `https://www.messenger.com`
5. Copy **giá trị** của 5 cookies sau (chỉ value, không lấy tên):

   | Cookie | Ví dụ |
   |---|---|
   | `c_user` | `100012345678901` (15-16 số) |
   | `xs` | `43%3A...` (chuỗi dài có `%3A`) |
   | `datr` | chuỗi base64 |
   | `sb` | chuỗi base64 |
   | `fr` | optional, có thể trống |

6. Paste vào wizard → **Login**

Nếu thành công: status hiển thị **"Connected"** (màu xanh) + tên FB của bạn.

---

## 3. Kiểm tra hoạt động

### Test nhanh

```bash
# 1. Container chạy
docker ps | grep -E "goclaw-goclaw|fbm-sidecar"
# Expected 3 containers healthy

# 2. Sidecar health
source /opt/goclaw/.env.fbm
curl -s -H "Authorization: Bearer $FBM_AUTH_TOKEN" http://localhost:29320/healthz
# Expected: {"status":"ok","synapse":"reachable"}

# 3. Bundle diagnostics
sudo /path/to/bundle/bin/fbm-diagnose
```

### Test end-to-end

Nhờ một người bạn gửi DM tới tài khoản FB test của bạn → agent GoClaw sẽ tự động trả lời trong vài giây.

Xem logs:
```bash
docker logs fbm-sidecar --tail 50 -f    # sidecar
docker logs goclaw-goclaw-1 --tail 50 | grep facebook_personal   # gateway
```

---

## 4. Gỡ cài đặt

```bash
sudo bash install-fbm-bundle.sh ...  # hoặc:
sudo bash uninstall-fbm-bundle.sh /opt/goclaw
```

- Downtime: **< 30 giây**
- GoClaw + các channel khác (Telegram, Zalo, v.v.) **KHÔNG bị ảnh hưởng**
- Thêm `--purge` để xóa luôn fork images (tiết kiệm disk)

---

## 5. Update khi upstream GoClaw lên version mới

Xem [UPGRADE-GUIDE.md](UPGRADE-GUIDE.md).

**Tóm tắt**: Fork images dùng `pull_policy: never` nên `docker compose pull` **không** ghi đè chúng. FBM tiếp tục hoạt động. Chỉ khi upstream thay đổi nhiều (schema, interface) mới cần rebuild bundle.

Kiểm tra:
```bash
sudo bash install/fbm-check-upgrade.sh /opt/goclaw
```

- Exit 0: OK, không cần làm gì
- Exit 1: warning, nên xem
- Exit 2: cần action (rebuild bundle)

---

## 6. Troubleshooting

Xem [TROUBLESHOOTING.md](TROUBLESHOOTING.md) cho 15+ tình huống lỗi thường gặp.

Một số lỗi phổ biến nhất:

| Triệu chứng | Fix nhanh |
|---|---|
| "Facebook Messenger (Personal)" không hiện trong dropdown | Hard refresh browser (Cmd/Ctrl+Shift+R) |
| Sidecar không healthy | `docker logs fbm-sidecar --tail 30`; thường do Synapse boot chậm → đợi 60s |
| "signature invalid" trong logs | HMAC secret không khớp → `sudo bash setup-secrets.sh /opt/goclaw --force` rồi restart |
| Cookie hết hạn | Vào Channels → ⋯ → Re-authenticate → paste cookies mới |
| FB checkpoint ngay lần login đầu | VPS datacenter IP → stop ngay; không retry 72h; dùng residential IP |

---

## 7. FAQ

**Q: Cookie sống bao lâu?**
A: Thường 3–14 ngày tuỳ hoạt động account. FB làm mới tự động khi bot active. Hết hạn → re-auth qua wizard.

**Q: Có thể chạy nhiều account FB cùng lúc?**
A: Có, mỗi account cần 1 sidecar riêng + 1 residential IP riêng. Không share IP cross-account.

**Q: Bị ban tài khoản thì sao?**
A: Tài khoản mất vĩnh viễn. **Không retry login từ cùng IP trong 72h** — fingerprint sẽ linked. Tạo account mới qua residential IP khác + browser profile khác.

**Q: Có logs chi tiết không?**
A:
```bash
docker logs fbm-sidecar -f --tail 100
docker logs goclaw-goclaw-1 --tail 200 | grep -i "facebook_personal\|fbm"
```

**Q: Làm sao update bundle?**
A: Tải bundle mới → `sudo bash install-fbm-bundle.sh --force`. Downtime ~30 giây.

**Q: Gửi bundle cho người khác được không?**
A: Bundle chứa mautrix/meta (AGPL-3.0). Nếu phân phối, bạn **PHẢI** kèm source (AGPL §13). Bundle đã có `LICENSE` + `NOTICE` trong đó.

---

## 8. Thông tin thêm

- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) — lỗi thường gặp
- [UPGRADE-GUIDE.md](UPGRADE-GUIDE.md) — update khi GoClaw upstream release
- [facebook-personal.md](facebook-personal.md) — documentation kỹ thuật
- [facebook-personal-security.md](facebook-personal-security.md) — threat model

---

**Version**: bundle version ghi trong `MANIFEST.json`
**License**: AGPL-3.0-or-later (do mautrix/meta trong sidecar image)
**Hỗ trợ**: liên hệ người cung cấp bundle cho bạn
