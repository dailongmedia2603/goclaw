# Menu Tài khoản (Lưu nick MXH & lấy OTP)

Lưu username/password Facebook, TikTok, Instagram, YouTube để kịch bản dùng khi login tự động. Hỗ trợ kết nối Gmail để lấy OTP — không cần gõ tay.

## Đầu trang & thanh công cụ

- Tiêu đề **"Quản lý Tài khoản"** + phụ đề "X tài khoản | Y hoạt động"
- Nút **"+ Thêm tài khoản"** (xanh dương)
- Ô tìm kiếm (theo username hoặc email)
- Tab kênh: Tất cả | Facebook | TikTok VN | TikTok US | Instagram | YouTube | Khác
- Bộ lọc trạng thái: Tất cả / Hoạt động / Khoá / Cấm

## Bảng tài khoản (9 cột)

| Cột | Nghĩa |
|-----|-------|
| Tài khoản | Username (đậm) + tên kênh (xám) |
| Mật khẩu | Bị che `••••••••`. Icon 👁 hiện, 📋 copy |
| Kênh | Badge màu theo kênh |
| Email | Email liên kết + nút copy |
| Pass mail | Mật khẩu email (che/copy) |
| Thiết bị | Tên máy được gán (hoặc "-") |
| Trạng thái | Hoạt động / Khoá / Cấm |
| Gmail | "Kết nối" (chưa) hoặc "✓ Đã kết nối" |
| Thao tác | Sửa & xoá (hiện khi rê chuột) |

## Cửa sổ Thêm/Sửa tài khoản

| Trường | Bắt buộc? | Điền gì |
|--------|-----------|---------|
| Tài khoản | ✅ | Username — VD `tiktok_user_01` |
| Mật khẩu | ❌ | Có thể để trống nếu dùng OAuth |
| Kênh | ✅ | Facebook / TikTok VN / TikTok US / IG / YT / Khác |
| Email | ❌ | Email liên kết (để lấy OTP) |
| Mật khẩu mail | ❌ | Password hộp mail |
| Thiết bị | ❌ | Máy chủ lực dùng nick này |
| Trạng thái | ❌ | Hoạt động / Khoá / Cấm — ghi chú nội bộ |
| Ghi chú | ❌ | Tự do — VD "Ngày tạo: 01/03/2026" |

## Kết nối Gmail để lấy OTP tự động

1. **Admin** đã cấu hình Gmail OAuth sẵn cho hệ thống (làm 1 lần — bạn không cần làm).
2. **Bấm "Kết nối"** ở cột Gmail của tài khoản cần.
3. **Tab mới mở Google** → đăng nhập bằng chính email cần kết nối.
4. **Cấp quyền** cho Phone-Agent (chỉ đọc email, không gửi/xoá).
5. **Tự động quay về dashboard** — hệ thống lưu xong.
6. **Nhãn đổi thành "✓ Đã kết nối"** → trong kịch bản, node *"Lấy OTP Mail"* sẽ tự đọc email và điền mã.

## Tự động chia tài khoản cho nhiều máy

Khi kịch bản chạy trên nhiều máy và node "Login" có nhiều tài khoản được chọn, hệ thống **chia mỗi máy một nick khác nhau theo thứ tự vòng**:

- Máy 1 → tài khoản 1
- Máy 2 → tài khoản 2
- Máy 3 → tài khoản 3
- … (hết → quay lại tài khoản 1)

> 💚 **Lợi ích:** 10 máy login 10 nick khác nhau **chỉ bằng 1 kịch bản** — không cần duplicate 10 bản.

## Xoá tài khoản

Bấm icon thùng rác → xác nhận.

> ⚠️ **Tất cả kịch bản đang dùng tài khoản này sẽ báo lỗi ở node login** — hãy kiểm tra trước khi xoá.
