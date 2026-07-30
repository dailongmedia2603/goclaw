# Menu Thư viện Video

Nơi **upload video** để dùng trong kịch bản. Node *"Truyền File"* sẽ truyền video từ đây sang iPhone để đăng TikTok / Facebook / Reels.

## Đầu trang

- Tiêu đề **"Thư viện Video"** + phụ đề "Upload và quản lý video, gửi tới thiết bị để đăng"
- Nút **"Upload video"** (xanh dương) — mở hộp thoại chọn file. Khi đang upload → "Đang upload {filename}..." + spinner

## Lưới video (responsive)

Hiển thị: 1 cột (mobile) / 2 (md) / 3 (lg) / 4 (xl) cột. Mỗi thẻ:

- **Thumbnail** — frame đầu của video
- **Tên video** (truncate)
- Thông tin: dung lượng (KB/MB) · ngày upload (`DD/MM/YYYY HH:MM`)
- Nút **"Gửi tới thiết bị"** (xanh dương)
- Nút 🗑 — xoá video

## Luồng upload video

1. Bấm **"Upload video"** → chọn file từ máy tính.
2. Phải là file video, **tối đa 500MB**.
3. Hệ thống kiểm tra dung lượng còn lại trong gói — nếu vượt sẽ từ chối.
4. Tải lên bộ nhớ đám mây (bạn không cần lo chỗ lưu).
5. Thông báo **"Upload thành công!"** → video xuất hiện trong lưới.

## Cửa sổ "Gửi video tới thiết bị"

Bấm **"Gửi tới thiết bị"** → cửa sổ:

- Dòng "Video: **[tên video]**"
- Danh sách "Chọn thiết bị" — chỉ hiện máy online
- Nút **Huỷ** và **Gửi** (mờ nếu chưa chọn máy)

Bấm Gửi → iPhone tự tải video về thư viện ảnh (camera roll). Tối đa **120 giây**.

## Xoá video

Bấm 🗑 → "Xoá video? Video sẽ bị xoá và không thể hoàn tác." → Xác nhận → Thông báo "Đã xoá video".

## Lưu ý quan trọng

> ℹ️ **Tính năng Video cần admin bật bộ nhớ đám mây.** Nếu thông báo "Chưa cấu hình bộ nhớ" → liên hệ admin.

> 💡 **Hiệu năng:** Video < 100MB truyền sang iPhone nhanh hơn. File quá lớn dễ bị "Quá thời gian chờ".

> 🤖 **Cho AI/Bot bên ngoài đăng video tự động:** Xem `API.md` mục 16 — Luồng đăng video tự động. AI có thể đăng ký video bằng URL CDN của họ, không cần upload lên dashboard.
