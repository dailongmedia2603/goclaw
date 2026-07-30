# Bảng tra nhanh khi gặp sự cố

## Không đăng nhập được

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Sai email/mật khẩu" | Nhập sai | Kiểm tra phím CapsLock, sao chép chính xác từ email được cấp |
| Nút Đăng nhập quay mãi | Server không phản hồi | Tải lại trang, thử sau vài phút, liên hệ admin nếu vẫn lỗi |
| Vừa đăng nhập đã bị đá ra | Phiên hết hạn (sau 7 ngày) | Đăng nhập lại bình thường |

## Thiết bị offline liên tục

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Offline ngay sau khi thêm | Sai IP | Kiểm tra IP đã được cấp, nhập lại cho đúng |
| Rớt sau vài phút | Pin yếu / iPhone tự ngủ màn hình | Bật "Luôn sáng màn hình" + cắm sạc |
| Nhiều máy offline cùng lúc | Mất WiFi / mất điện | Kiểm tra WiFi + nguồn điện |
| Online nhưng không nhận lệnh | App điều khiển trên iPhone bị treo | Tắt và mở lại app điều khiển |

## Kịch bản chạy lỗi

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Lỗi ở bước "tap" | Vị trí tap sai do khác độ phân giải | Dùng "Chọn vị trí tap" pick lại trên màn hình chính xác |
| Lỗi "Device offline" | Máy mất kết nối giữa chừng | Kiểm tra WiFi iPhone, chạy lại |
| Lỗi "Không tìm thấy kịch bản" | Kịch bản đã bị xoá nhưng lịch trình còn trỏ đến | Xoá lịch trình hoặc gán kịch bản khác |
| Lỗi ở bước login | Sai thông tin tài khoản hoặc app đổi giao diện | Kiểm tra lại nick, cập nhật kịch bản theo UI mới |

## Proxy không hoạt động

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Trạng thái proxy "Lỗi" | Thông tin proxy sai | Kiểm tra IP, cổng, user, mật khẩu proxy |
| Giả lập không áp dụng | Chưa tự phát hiện IP | Bấm "Tự động phát hiện" — đảm bảo IP đúng quốc gia |
| Múi giờ vẫn là VN khi dùng proxy US | Chỉ bấm "Kích hoạt", chưa áp dụng giả lập | Bấm **Áp dụng & Chạy** thay vì "Kích hoạt" |

## Video không gửi được

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Chưa cấu hình bộ nhớ" | Admin chưa bật bộ nhớ đám mây | Liên hệ admin để bật |
| "Quá thời gian chờ" | Video quá lớn hoặc iPhone chậm | Giảm video xuống < 100MB |
| "Thiết bị ngoại tuyến" | Máy rớt trong lúc gửi | Kiểm tra lại máy trước khi gửi |

## Không xem được màn hình iPhone

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Đang kết nối..." mãi | Mạng chậm hoặc bị chặn | Tải lại trang, nếu vẫn lỗi liên hệ admin |
| Màn hình đen | App điều khiển trên iPhone chưa bật | Mở app điều khiển trên iPhone, bật dịch vụ |
| Hình méo / giật | Băng thông mạng thấp | Đổi mạng ổn định hơn hoặc báo admin giảm FPS |
| Sai mật khẩu | Mật khẩu hiển thị màn hình không đúng | "Sửa thiết bị" → nhập lại mật khẩu |

## Cần hỗ trợ sâu hơn?

Liên hệ đội ngũ Phone-Agent qua **Telegram** (link ở trang chủ → Footer → "Tư vấn qua Telegram"). Khi báo lỗi, vui lòng gửi kèm:

- 📸 Ảnh chụp màn hình lỗi
- 📄 Log chi tiết từ menu *Nhật ký* (bấm Export → JSON)
- 📦 Thông tin gói đang dùng + số máy hiện có
- 📱 Model iPhone và phiên bản iOS

## Checklist sử dụng lần đầu

- [ ] Đổi mật khẩu ngay sau lần đăng nhập đầu tiên
- [ ] Cấu hình Telegram Bot để nhận cảnh báo
- [ ] Thêm iPhone vào hệ thống
- [ ] Tạo nhóm thiết bị theo mục đích
- [ ] Copy kịch bản mẫu từ *Kho kịch bản* hoặc tự tạo
- [ ] Test chạy thủ công trên 1 máy trước
- [ ] Thiết lập lịch trình tự động
- [ ] Theo dõi *Nhật ký* hàng ngày trong tuần đầu
