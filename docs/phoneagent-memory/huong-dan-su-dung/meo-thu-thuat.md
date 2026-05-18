# Mẹo & thủ thuật

## Tối ưu kịch bản

- **Dùng "Chờ ngẫu nhiên"** thay vì Chờ cố định ở vị trí nhạy cảm — tránh bị phát hiện bot.
- **Không hardcode toạ độ tuyệt đối** — dùng giá trị 0–1, Phone-Agent tự scale theo màn hình.
- **Chụp ảnh tại checkpoint** (sau login, trước khi đăng bài) — rất giá trị khi debug.
- **Lặp lại ở ngoài cùng** thay vì tạo kịch bản dài vô tận.
- **Nhóm thao tác thành custom node** (VD "Đăng bài", "Comment") để tái sử dụng.

## Quản lý farm

- Gom máy theo mục đích: TikTok VN · TikTok US · Facebook.
- Đặt tên máy có quy tắc: `TT-VN-01` thay vì "iPhone của Nam".
- Kiểm tra Tailscale status hàng ngày — máy rớt Tailscale sẽ offline dù WiFi OK.
- Không chạy quá nhiều kịch bản song song trên cùng 1 máy — chậm/giật.

## Bảo mật

- **Đổi mật khẩu ngay** sau lần đăng nhập đầu tiên.
- **Khoá API chỉ hiện 1 lần** — lưu vào trình quản lý mật khẩu ngay.
- **Bật cả 2 cảnh báo Telegram** (máy offline + kịch bản lỗi) để biết sớm khi có sự cố.
- Không chia sẻ tài khoản dashboard với người ngoài team.

## Hiệu năng

- **Bật proxy từng đợt 10–20 máy** thay vì 100 máy cùng lúc.
- **Dọn nhật ký cũ hàng tuần** — nhật ký lớn làm màn hình tải chậm.
- **Video < 100MB** — file lớn truyền sang iPhone sẽ chậm.
- Không chạy quá nhiều kịch bản song song trên cùng 1 máy.

## Xử lý nhanh khi có lỗi

| Triệu chứng | Cách xử lý |
|-------------|------------|
| Máy "Đang kết nối" mãi | Kiểm tra IP và mật khẩu đã đúng chưa, iPhone đã mở app điều khiển chưa |
| Kịch bản chạy 1 bước rồi dừng | Mở Nhật ký → xem bước lỗi → thường do tap sai hoặc app crash |
| Bật proxy nhưng IP vẫn nguyên | Bấm **Áp dụng & Chạy** thay vì chỉ "Kích hoạt" — vì "Kích hoạt" không tự áp dụng giả lập |
| "Đã đạt giới hạn" | Gói đầy máy/kịch bản — liên hệ admin nâng cấp hoặc xoá bớt |
