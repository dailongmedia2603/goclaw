# Menu Cài đặt cá nhân

Trang cài đặt chia tab dọc bên trái. Tài khoản thường có **2 tab chính**: Thông tin cá nhân & Thông báo.

## Tab "Thông tin cá nhân"

### Thẻ thông tin tài khoản

- Ảnh đại diện (màu nền tự sinh theo tên)
- Tên + email đăng nhập
- Nhãn vai trò: "Người dùng"

### Khoá API cá nhân (nếu gói có)

Khoá API dùng để kết nối Phone-Agent với phần mềm bên ngoài (bot, tool tự động hoá). Xem chi tiết tích hợp ở `API.md`.

> 🔐 **Khoá chỉ hiện đầy đủ 1 lần khi mới tạo!** Sau khi đóng, chỉ còn hiện 20 ký tự đầu. **Sao chép và lưu vào trình quản lý mật khẩu ngay.**

Thông tin kèm theo: ngày tạo, lần dùng gần nhất, IP sử dụng gần nhất.

**Nút "Đổi khoá mới"** (cam) → cảnh báo:
- "Khoá cũ sẽ ngừng hoạt động ngay lập tức."
- "Mọi phần mềm đang dùng khoá hiện tại cần cập nhật sang khoá mới."

### Đổi mật khẩu

- Ô **"Mật khẩu hiện tại"**
- Ô **"Mật khẩu mới"** (tối thiểu 6 ký tự)
- Nút **"Cập nhật mật khẩu"** (xanh dương)

## Tab "Thông báo"

### Bật/tắt loại cảnh báo

- 🔕 Công tắc **Thiết bị ngoại tuyến**
- ⚠️ Công tắc **Kịch bản chạy lỗi**

Bật cái nào → khi có sự kiện đó, bạn sẽ nhận cảnh báo.

### Kết nối Telegram Bot

| Trường | Điền gì |
|--------|---------|
| Bot Token | Mã từ `@BotFather`. Có nút 👁 hiện/ẩn |
| Chat ID | ID group Telegram nơi bạn muốn nhận cảnh báo |

**Nút "Kiểm tra kết nối"** → gửi tin nhắn thử. Nếu nhận được → OK. Thông báo: *"Kết nối Telegram thành công! Kiểm tra tin nhắn trong chat."*

### Cách lấy Bot Token & Chat ID

1. Mở Telegram → tìm `@BotFather` → gõ `/newbot` → đặt tên → nhận **Bot Token**.
2. Thêm bot vào group + gọi tên bot 1 lần để bot "thấy" group.
3. Lấy Chat ID: truy cập `https://api.telegram.org/bot{TOKEN}/getUpdates`, tìm số `chat.id`.
