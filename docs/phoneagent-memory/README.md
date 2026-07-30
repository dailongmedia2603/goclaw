# Phone-Agent Support — Bộ nhớ đã phân tách

Bộ file markdown dành cho agent **PhoneAgent Support** (trong GoClaw Memory), được phân tách từ `docs/HDSD-phoneagent.md` (v2.0, 2026-04-20) theo từng chương để tối ưu truy hồi ngữ nghĩa (RAG).

## Cấu trúc thư mục

```
huong-dan-su-dung/
├── gioi-thieu-phone-agent.md              (Ch 0) Phone-Agent là gì + thuật ngữ
├── dang-ky-dang-nhap.md                    (Ch 1) Đăng ký, đăng nhập, bố cục dashboard
├── dashboard-tong-quan.md                  (Ch 2) Menu Dashboard (thẻ số liệu, biểu đồ)
├── quan-ly-thiet-bi.md                     (Ch 3) Menu Thiết bị (thêm/xoá/nhóm iPhone)
├── proxy-doi-ip.md                         (Ch 4) Menu Proxy (US/VN, giả lập GPS)
├── tai-khoan-mxh.md                        (Ch 5) Menu Tài khoản (nick MXH + Gmail OTP)
├── kich-ban-ca-nhan.md                     (Ch 6) Menu Kịch bản (chạy, import/export)
├── trinh-bien-tap-kich-ban-truc-quan.md    (Ch 7) Trình biên tập kéo-thả (node, canvas)
├── kho-kich-ban-thu-vien-mau.md            (Ch 8) Kho kịch bản mẫu (admin chia sẻ)
├── lich-trinh-chay-tu-dong-theo-gio.md     (Ch 9) Lịch trình (scheduler 60s)
├── nhat-ky-theo-doi-lich-su.md             (Ch 10) Nhật ký (log, ảnh, auto-cleanup)
├── thu-vien-video.md                       (Ch 11) Thư viện Video (upload, truyền)
├── cai-dat.md                              (Ch 12) Cài đặt cá nhân (đổi pass, Telegram)
├── cac-luong-hoat-dong-dien-hinh.md        (Ch 13) 10 luồng làm việc thực tế
├── meo-thu-thuat.md                        (Ch 14) Mẹo tối ưu, quản lý farm
├── cau-hoi-thuong-gap.md                   (Ch 15) FAQ
└── xu-ly-su-co.md                          (Ch 16) Bảng tra sự cố + checklist
```

## Nguyên tắc phân tách

- **1 chương = 1 file** — mỗi file tự đủ ngữ cảnh, phù hợp vector retrieval.
- **Giữ nguyên thuật ngữ UI** (tên menu, tên nút, badge màu) — agent trích dẫn lại đúng cho user.
- **Không bịa thêm** nội dung ngoài bản gốc `docs/HDSD-phoneagent.md`.
- **Heading H2 ngắn + cụ thể** — khớp với câu hỏi user thường hỏi ("Làm sao để…", "Proxy US là gì").
- **Bảng giữ nguyên** — giá trị tra cứu cao, agent có thể trích dẫn trực tiếp.

## Cách đưa vào bộ nhớ agent PhoneAgent Support

Trên dashboard GoClaw:

1. Vào **Bộ nhớ** (sidebar trái).
2. Dropdown **Agent** → chọn `PhoneAgent Support` (⚠️ xác nhận đúng agent, không nhầm sang agent khác).
3. Phạm vi: **Toàn cục** (để tất cả user của agent đều nhìn thấy).
4. Xoá 9 file `huong-dan-su-dung/*.md` hiện có (nếu muốn thay thế toàn bộ) hoặc chỉ refresh các file đã có.
5. Upload/copy toàn bộ 17 file trong `huong-dan-su-dung/` vào workspace agent (`/app/workspace/phoneagent-support/huong-dan-su-dung/`).
6. Bấm **Lập chỉ mục tất cả** để re-embed bằng `gemini-embedding-001`.

### Đồng bộ qua Docker (nếu workspace là volume)

```bash
# Copy vào volume goclaw-workspace
docker cp docs/phoneagent-memory/huong-dan-su-dung/. \
  <container-name>:/app/workspace/phoneagent-support/huong-dan-su-dung/
```

Sau đó chạy lại **Lập chỉ mục tất cả** trong UI.

## Chương đã cập nhật

| File hiện có trong UI | Chương | Trạng thái |
|----------------------|--------|------------|
| trinh-bien-tap-kich-ban-truc-quan.md | Ch 7 | **Thay thế** (đã cập nhật đủ node + auto pre-delay 5s) |
| kho-kich-ban-thu-vien-mau.md | Ch 8 | **Thay thế** |
| lich-trinh-chay-tu-dong-theo-gio.md | Ch 9 | **Thay thế** |
| nhat-ky-theo-doi-lich-su.md | Ch 10 | **Thay thế** |
| thu-vien-video.md | Ch 11 | **Thay thế** |
| cai-dat.md | Ch 12 | **Thay thế** |
| cac-luong-hoat-dong-dien-hinh.md | Ch 13 | **Thay thế** (10 luồng đầy đủ) |
| meo-thu-thuat.md | Ch 14 | **Thay thế** |
| xu-ly-su-co.md | Ch 16 | **Thay thế** (bảng tra đầy đủ 6 nhóm) |

## Chương mới bổ sung (chưa có trong UI)

| File mới | Chương | Nội dung |
|---------|--------|----------|
| gioi-thieu-phone-agent.md | Ch 0 | Giới thiệu + thuật ngữ |
| dang-ky-dang-nhap.md | Ch 1 | Đăng ký + dashboard layout |
| dashboard-tong-quan.md | Ch 2 | Menu Dashboard |
| quan-ly-thiet-bi.md | Ch 3 | Menu Thiết bị |
| proxy-doi-ip.md | Ch 4 | Menu Proxy |
| tai-khoan-mxh.md | Ch 5 | Menu Tài khoản MXH |
| kich-ban-ca-nhan.md | Ch 6 | Menu Kịch bản (kho cá nhân) |
| cau-hoi-thuong-gap.md | Ch 15 | FAQ |

---

**Nguồn:** `docs/HDSD-phoneagent.md` (Cẩm nang v2.0 — phoneagent.io.vn)
