# Menu Lịch trình (chạy tự động theo giờ)

Hẹn giờ chạy kịch bản — cốt lõi của tự động hoá 24/7. Hệ thống có 1 scheduler chạy ngầm, **mỗi 60 giây kiểm tra một lần** và kích hoạt lịch nào khớp giờ hiện tại.

## Bốn tab điều hướng

| Tab | Hiển thị |
|-----|----------|
| **Danh sách** | Bảng tất cả lịch trình đã tạo (mặc định khi vào) |
| **Lịch biểu** | Calendar dạng tháng — Tháng trước/Tiếp, Hôm nay |
| **Hàng đợi** | Execution đang chạy ngay bây giờ. Có badge số lượng |
| **Lịch sử chạy** | Execution đã kết thúc (success / error) |

## Bảng "Danh sách" lịch trình

| Cột | Nghĩa |
|-----|-------|
| Kênh | Badge TikTok VN / TikTok US / Facebook |
| Tên lịch trình | Tên + icon calendar |
| Kịch bản | Tên kịch bản sẽ chạy |
| Thiết bị / Nhóm | Tên máy hoặc tên nhóm |
| Tần suất | VD "Hàng ngày lúc 08:00" |
| Lần chạy tiếp | "Hôm nay, 08:00" / "Ngày mai, 14:30"… |
| Bật/Tắt | Switch xanh/xám để **tạm dừng** mà không xoá |
| Thao tác | Sửa + xoá |

## Bộ lọc & tìm kiếm

- Ô tìm kiếm: theo tên lịch hoặc tên kịch bản
- Nút **"Bộ lọc"** mở panel:
  - Kênh: Tất cả / TikTok VN / TikTok US / Facebook
  - Tần suất: Tất cả / Hàng giờ / Hàng ngày / Hàng tuần / Ngày tuỳ chỉnh
  - Thiết bị: Tất cả / [danh sách máy]
  - Nút Reset · Huỷ · Áp dụng

## Cửa sổ "Tạo lịch trình mới"

| Trường | Điền gì |
|--------|---------|
| Tên lịch trình | VD "Đăng bài TikTok sáng" |
| Kịch bản | Dropdown chọn từ kho cá nhân |
| Thiết bị | Dropdown chọn máy (hoặc nhóm) |
| Tần suất | Radio 5 lựa chọn (xem dưới) |
| Giờ chạy | HH:MM, mặc định `08:00` |
| Ngày chạy | Chỉ hiện nếu chọn "Ngày tuỳ chỉnh" |

### 5 tần suất

| Tần suất | Ý nghĩa |
|----------|---------|
| Một lần | Chạy đúng 1 lần (tự tắt sau đó) |
| Hàng giờ | Mỗi giờ vào phút MM |
| Hàng ngày | Mỗi ngày lúc HH:MM |
| Hàng tuần | Mỗi tuần vào thứ Hai lúc HH:MM |
| Ngày tuỳ chỉnh | Ngày cụ thể trong tháng/năm |

## Hệ thống hẹn giờ làm việc thế nào

1. Bộ hẹn giờ chạy ngầm, kiểm tra **mỗi 60 giây**.
2. Với mỗi lịch đang bật: so sánh thời gian hiện tại với giờ đã đặt.
3. Nếu khớp: tìm thiết bị tương ứng, **loại trừ máy offline**.
4. Chạy lần lượt trên từng máy (máy 1 xong mới sang máy 2).
5. Ghi lại thời điểm chạy và tính lần chạy kế tiếp.

> 💡 **Máy offline khi tới giờ?** Lịch bỏ qua máy đó và ghi vào nhật ký lỗi. Máy nào online thì vẫn chạy bình thường.

## Tạm dừng (toggle) thay vì xoá

Nếu chỉ muốn **tạm dừng vài ngày**, bấm switch ở cột Bật/Tắt thay vì xoá — đặt `enabled = false` nhưng giữ cấu hình. Bật lại rất nhanh.

## Xoá lịch trình

Icon thùng rác → xác nhận. Lịch trình biến mất nhưng **các execution đã chạy vẫn còn trong Nhật ký**.
