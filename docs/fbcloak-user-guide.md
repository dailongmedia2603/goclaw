# FBCloak — Hướng dẫn sử dụng

> Tính năng tự động re-engage hộp thư fanpage Facebook cho khách hàng đã > 7 ngày không tương tác. Chỉ khả dụng ở **Standard edition**.

## 1. Khi nào dùng FBCloak — và khi nào KHÔNG

| Khoảng cách `last_inbound_at` | Đề xuất | Tính năng dùng |
|---|---|---|
| ≤ 24 giờ | Trả lời thường | Channel `facebook_personal` (Send qua sidecar) |
| ≤ 7 ngày | Reply HUMAN_AGENT | Graph API qua `fbcloak.send-proactive` (khi đã cấu hình) |
| 7 ngày → 6 tháng | **FBCloak browser automation** | Phần lớn use case của tài liệu này |
| > 6 tháng | Không hỗ trợ | Marketing Messages (defer) |

**FBCloak khác `facebook_personal` ở chỗ:** `facebook_personal` xử lý realtime inbound + outbound trong cùng phiên hội thoại; FBCloak chỉ chạy theo cron để chăm sóc khách lâu năm.

## 2. Trước khi bắt đầu — checklist

- [ ] Tenant đang dùng **Standard edition** (`edition.FBCloakEnabled = true`)
- [ ] Đã đọc và đồng ý với [docs/fbcloak-tos-disclaimer.md](./fbcloak-tos-disclaimer.md)
- [ ] Có tài khoản admin của fanpage và 1 trình duyệt cá nhân để export cookie
- [ ] (Khuyến nghị) Có proxy SOCKS5 cố định cho fanpage này — IP residential VN
- [ ] Đã sync `fbbackfill` cho fanpage để có dữ liệu `last_inbound_at`

## 3. Lấy cookie fanpage

1. Đăng nhập Facebook bằng tài khoản admin trên trình duyệt cá nhân.
2. Cài extension **Cookie-Editor** (Chrome / Firefox).
3. Vào `business.facebook.com/latest/inbox` — đảm bảo inbox load đầy đủ.
4. Bấm icon Cookie-Editor → **Export → JSON**.
5. Copy JSON đó vào ô "Cookies (JSON)" trong dialog "Thêm credential".

Bắt buộc có ít nhất 4 cookie: `c_user`, `xs`, `fr`, `datr`. Server kiểm tra qua `pkg/browser.ValidateFBCookies`.

## 4. Tạo credential

Vào **/fbcloak → Credentials → Thêm credential**:

- **Fanpage ID**: số ID fanpage (ví dụ `100012345678`)
- **Fanpage name**: hiển thị nội bộ
- **Cookies (JSON)**: dán từ bước 3
- **Proxy URL** (tuỳ chọn): `socks5://user:pass@host:port` — nên là proxy cố định cho từng fanpage để Meta không phát hiện thay IP đột ngột
- **User agent**: để trống → server chọn UA Chrome 124 mặc định

Sau khi lưu, bấm **Test** để chạy health probe (cookie inject + redirect `/me`). Status `active` = OK.

## 5. Tạo job re-engagement

**/fbcloak → Jobs → Tạo job**:

| Trường | Khuyến nghị |
|---|---|
| Cron | `0 9 * * *` — chạy 9h sáng mỗi ngày |
| Idle tối thiểu | **7 ngày** (mặc định) — không nhỏ hơn vì sẽ chồng vùng HUMAN_AGENT |
| Idle tối đa | 30–90 ngày |
| Daily cap | **5–10** ngày đầu, tăng dần lên tối đa 50 |
| Working hours | 08:00–21:00 `Asia/Ho_Chi_Minh` |
| Dry-run | **BẬT** lần đầu — quan sát log 1 tuần trước khi chuyển live |

> 💡 Phía dưới ô cron, UI hiển thị `min idle > 7 days — Cloak browser path` cho biết job sẽ chạy qua FBCloak (chứ không qua Graph API). Đây là chỉ dẫn minh bạch.

Job mặc định ở trạng thái **Disabled + Dry-run**. Bạn phải:
1. Xác nhận **Disclaimer** (popup tự bật khi vào /fbcloak/jobs lần đầu).
2. Theo dõi **Send Log** trong 1–7 ngày dry-run.
3. Khi tự tin, bật Switch "Enabled".
4. Tắt dry-run **chỉ khi** đã review nội dung message + verify pass logic.

## 6. Theo dõi Send Log

**/fbcloak → Send Log**:

- Filter theo job, status, ngày
- Click "View" để xem chi tiết: message, screenshot trước/sau, error
- Status:
  - `sent` — gửi thành công
  - `dry_run` — đã giả lập (chưa gửi thật)
  - `skipped` — bị policy chặn (cooldown 30d, daily cap, opt-out keyword, verify mismatch)
  - `failed` — lỗi navigate / type / click / xác nhận

## 7. Khi gặp checkpoint

Khi job runner phát hiện checkpoint giữa chừng (`/checkpoint`, captcha, redirect `/login`, account suspended):

1. **Job tự động abort** — status `killed` ở Job table.
2. **Credential.status** chuyển sang `checkpoint` hoặc `disabled`.
3. **Eventbus** publish `fbcloak.checkpoint` (có thể subscribe vào admin Telegram).
4. **Screenshot** evidence được lưu vào `~/.goclaw/data/fbcloak/screenshots/{tenant}/{job}/`.

**Recovery:**
1. Đăng nhập tài khoản admin vào trình duyệt thật → giải checkpoint thủ công (nhập OTP, captcha, …).
2. Export cookie mới qua Cookie-Editor.
3. Vào /fbcloak → Credentials → xoá credential cũ, **Thêm credential** mới với cookie vừa export.
4. Tạm dừng job 24h trước khi enable lại để Meta "nguội" tín hiệu nghi ngờ.

## 8. Khi page bị restrict

- **Tắt killswitch ngay**: `export GOCLAW_FBCLOAK_KILLSWITCH=1` (xem operator runbook).
- Đợi Meta bỏ restrict (thường 24–72 giờ) — không cố gửi thêm.
- Sau khi gỡ, **giảm daily cap xuống 50%** trong 1 tháng để theo dõi.
- Cân nhắc đổi proxy IP và UA fingerprint.

## 9. Nội dung message — best practices

Server enforce:
- Tối đa 500 ký tự
- Blocklist keyword (cấu hình ở Policy)
- Per-recipient cooldown 30 ngày

Bạn nên:
- Dùng template SKILL.md có placeholder `{{recipient_name}}`
- Tránh CTA "click link" / "đăng ký ngay" — Meta NLP rất nhạy với tín hiệu spam
- Mở đầu bằng câu hỏi mở, ngắn (vd. "Anh/chị còn quan tâm đến X không ạ?")
- KHÔNG gửi link ngắn (`bit.ly`, `cutt.ly`) — gắn flag spam ngay

## 10. FAQ

**Q: Tôi có thể chạy nhiều fanpage cùng lúc không?**
A: Có. Mỗi fanpage 1 credential, hệ thống serialize per-credential và giới hạn `MaxConcurrent` (mặc định 5) toàn cluster.

**Q: Job có gửi tin trong giờ nghỉ không?**
A: Không. `WorkingHours` được kiểm tra mỗi tick — ngoài giờ thì RunOnce return `OK` mà không gửi.

**Q: Khách trả lời gần đây thì sao?**
A: `Verify-last-message` đọc tin cuối hiển thị trên thread, nếu khoảng cách < `MinIdle` → skip với reason `customer_replied_recent`.

**Q: Có thể dùng FBCloak qua MCP / agent tool không?**
A: KHÔNG. Theo thiết kế, FBCloak chỉ expose qua RPC admin-gated. LLM không có quyền gọi trực tiếp để tránh lạm dụng.

**Q: Tôi muốn xoá toàn bộ dữ liệu FBCloak khỏi tenant?**
A: Xoá credentials → cascade tự xoá jobs + send_log. Screenshots được purge sau retention (mặc định 30d) hoặc xoá thủ công ở `~/.goclaw/data/fbcloak/screenshots/{tenant_id}`.
