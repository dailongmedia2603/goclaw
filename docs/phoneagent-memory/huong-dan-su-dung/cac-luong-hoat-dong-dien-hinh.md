# 10 luồng làm việc thực tế

Các luồng chuẩn từ onboarding mới đến cảnh báo Telegram. Làm theo đúng trình tự để không kẹt giữa đường.

## 1. Nhận tài khoản & đăng nhập lần đầu

1. Đội ngũ Phone-Agent gửi cho bạn **email + mật khẩu tạm**.
2. Vào `phoneagent.io.vn` → bấm "Đăng nhập" → nhập email + mật khẩu.
3. Vào ngay *Cài đặt → Thông tin cá nhân → Đổi mật khẩu*.

## 2. Thêm iPhone mới vào hệ thống

1. iPhone phải được đội ngũ cài sẵn và kết nối Tailscale (làm khi bàn giao máy).
2. *Thiết bị → + Thêm thiết bị*: nhập tên, IP, mật khẩu hiển thị màn hình, chọn nhóm.
3. Đợi ~15 giây để hệ thống kiểm tra kết nối.
4. Máy chuyển sang **Online**.
5. Click vào hàng → xem màn hình iPhone live → thử Home, Lướt lên, Chụp ảnh.

## 3. Tạo kịch bản TikTok đơn giản

1. *Kịch bản → + Tạo mới*.
2. Tên "Nuôi kênh TikTok sáng", kênh **TikTok VN**.
3. Chọn máy pick toạ độ (dropdown header).
4. Kéo node **Mở TikTok VN** → nối từ "Bắt đầu".
5. Kéo **Chờ** (10s) để app load.
6. Kéo **Lướt lên** → nối tiếp.
7. Kéo **Chờ ngẫu nhiên** (5-15s).
8. Kéo **Lặp lại** count=20, nối vào handle "next".
9. Kéo 2 node vào body (handle phải): **Lướt lên** → **Chờ ngẫu nhiên**.
10. Bấm **🔧 Sắp xếp gọn** → **💾 Lưu**.

## 4. Chạy đồng thời 10 máy

1. *Kịch bản → ▶ Chạy*.
2. Tick 10 máy online.
3. Bấm **"Chạy ngay"**.
4. Xem tiến độ trong tab **"Hàng đợi"** của Lịch trình.
5. Xong → xem log từng máy ở *Nhật ký*.

## 5. Lên lịch chạy tự động 8h sáng

1. *Lịch trình → + Tạo lịch trình*.
2. Tên: "Sáng nuôi TikTok" · Kịch bản: "Nuôi kênh TikTok sáng".
3. Thiết bị: máy cụ thể (hoặc tạo 10 lịch cho 10 máy).
4. Tần suất **Hàng ngày**, giờ **08:00**.
5. Bấm **"Tạo lịch trình"**. Scheduler tự chạy mỗi 8h sáng.

## 6. Chuyển sang IP US trước khi chạy

1. *Proxy → chế độ "Proxy US"*.
2. Paste `ip:port:user:pass`.
3. Bấm **"Tự động phát hiện"** → timezone + GPS tự điền.
4. Tick các máy → bấm **"Áp dụng US & Chạy"**.
5. Đợi Proxy Status chuyển **Running**.
6. Chạy kịch bản TikTok US bình thường.

## 7. Cảnh báo Telegram khi máy rớt

1. Lấy Bot Token từ `@BotFather` + Chat ID từ group.
2. *Cài đặt → Thông báo*: bật toggle **"Thiết bị ngoại tuyến"**.
3. Nhập Bot Token + Chat ID → **"Kiểm tra kết nối"**.
4. Nhận tin test → bấm **"Lưu cấu hình"**.
5. Máy mất kết nối > 90s → tin nhắn Telegram cảnh báo.

## 8. Upload video và truyền sang iPhone

1. Tính năng Video cần admin bật bộ nhớ đám mây trước (làm 1 lần).
2. *Thư viện Video → Upload* → chọn file .mp4.
3. Trong trình biên tập, kéo khối **Truyền File** → chọn video.
4. Nối tiếp: mở TikTok → tap nút tạo bài → chọn video → đăng.
5. Lưu và chạy.

> 🤖 **Cho AI/Bot bên ngoài đăng video tự động:** Xem `API.md` mục 16 — Luồng đăng video tự động. AI có thể đăng ký video bằng URL CDN của họ, không cần upload lên dashboard.

## 9. Lấy OTP email tự động khi login MXH

1. Admin đã kết nối Gmail OAuth sẵn (bạn không cần làm).
2. *Tài khoản → cột Gmail → Kết nối* → cấp quyền Google (chỉ đọc email).
3. Nhãn chuyển **✓ Đã kết nối**.
4. Trong kịch bản, sắp theo thứ tự: **Nhập tài khoản → Nhập mật khẩu → Chờ 5s → Lấy OTP Mail**.
5. Khối "Lấy OTP Mail" tự đọc email mới nhất trong 60 giây và điền OTP.

## 10. Phục hồi khi kịch bản có lỗi

- Vào *Nhật ký* → tìm dòng "Lỗi" → xem bước nào fail.
- Test điều khiển thủ công trong cửa sổ chi tiết thiết bị → bấm Home → không phản hồi nghĩa là iPhone có vấn đề.
- Nếu tất cả máy offline cùng lúc: kiểm tra WiFi / nguồn điện nơi đặt iPhone.
- Nếu không xử lý được: liên hệ đội ngũ Phone-Agent qua Telegram.
