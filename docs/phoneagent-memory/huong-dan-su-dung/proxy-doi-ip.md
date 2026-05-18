# Menu Proxy (Đổi IP & chống phát hiện)

Đổi IP cho từng máy hoặc cả nhóm. Quan trọng khi nuôi nick TikTok US, Facebook US mà farm thật đặt ở Việt Nam — giúp tài khoản "trông giống" người dùng bản địa.

## Đầu trang & thống kê

- Tiêu đề **"Proxy Manager"**
- Số liệu: Tổng thiết bị · Số online · Số proxy đang chạy
- Nút **"Làm mới"** — tải lại trạng thái

## Ba chế độ đổi IP

Chọn 1 trong 3:

| Chế độ | Dùng khi | Hệ thống làm gì |
|--------|----------|------------------|
| 🇺🇸 **Proxy US** | Nuôi nick TikTok US, Facebook US | Đổi IP sang Mỹ + giả lập múi giờ + ngôn ngữ + GPS sang Mỹ |
| 🇻🇳 **Proxy VN** | Nuôi nick TikTok VN | Đổi IP nội địa + GPS khớp vị trí proxy + ẩn WiFi, giữ tiếng Việt |
| ❌ **Không proxy** | Trở về mặc định | Tắt proxy + xoá toàn bộ giả lập |

## Bảng cấu hình 2 cột (chỉ hiện khi chọn US/VN)

### Cột trái — Thông tin proxy

| Trường | Điền gì |
|--------|---------|
| Ô dán nhanh | Dán chuỗi `ip:port:user:pass` → tự tách vào 4 ô |
| Địa chỉ IP | VD `1.2.3.4` |
| Cổng | VD `8080` |
| Tên đăng nhập | Nhà cung cấp proxy cấp |
| Mật khẩu | Nhà cung cấp proxy cấp |

### Cột phải — Giả lập chống phát hiện

**Nếu chọn Proxy US:**
- Nút **"Tự động phát hiện"** — hệ thống tự tra IP, điền giúp múi giờ + GPS
- Múi giờ: Eastern / Central / Mountain / Pacific
- GPS: chọn 1 trong 5 thành phố mẫu (New York, LA, Chicago, Houston, Phoenix)
- Tự động: ngôn ngữ `en_US`, ẩn WiFi, giả sóng di động Mỹ, DNS qua proxy

**Nếu chọn Proxy VN:**
- Nút **"Tự động phát hiện"** — lấy thành phố + GPS của IP Việt Nam
- Không có chọn múi giờ/ngôn ngữ — giữ tiếng Việt tự nhiên
- Tự động: GPS khớp vị trí proxy, ẩn WiFi

## Hành động hàng loạt

| Hành động | Tác dụng |
|-----------|----------|
| Chọn tất cả / Bỏ chọn | Chọn toàn bộ máy online (offline không thể chọn) |
| **Áp dụng & Chạy** | Tên đổi theo chế độ: "Áp dụng US (X)" / "Áp dụng VN (X)" / "Tắt tất cả (X)" |
| **Dừng (X)** | Tắt proxy hàng loạt |
| Đếm số lượng | "Đã chọn X thiết bị" |

Nút "Áp dụng" bị mờ nếu: chưa chọn máy, chưa nhập IP/Port, hoặc máy đang chạy proxy.

## Bảng thiết bị chi tiết

| Cột | Nghĩa |
|-----|-------|
| ☑ Chọn | Không thể tick máy offline |
| Thiết bị | Tên + IP nội bộ |
| Trạng thái | Online / Đang chạy / Đang kết nối / Ngoại tuyến |
| Proxy | IP:Cổng đã cấu hình hoặc "Chưa cấu hình" |
| Trạng thái Proxy | "Đang chạy" hoặc "Đã tắt" |
| Hành động | "Kích hoạt" (xanh) hoặc "Dừng" (đỏ) |

## Khi bấm "Áp dụng & Chạy" — hệ thống làm gì

1. **Lưu cấu hình** — ghi IP/cổng/user/pass vào từng máy.
2. **Bật proxy** — khởi động phần mềm đổi IP trên iPhone, chờ 2 giây xác minh.
3. **Áp dụng giả lập** — múi giờ, ngôn ngữ, GPS theo chế độ. Nếu chưa tự phát hiện sẽ tự tra IP.

Mỗi lỗi từng máy có thông báo riêng, máy còn lại vẫn tiếp tục. Cuối cùng: "Đã áp dụng Proxy US/VN cho X thiết bị".

## Hành động đơn lẻ

| Nút | Tác dụng |
|-----|----------|
| **Kích hoạt** (xanh) | Chỉ bật proxy đã cấu hình sẵn |
| **Dừng** (đỏ) | Tắt proxy trên máy đó |

## Lưu ý quan trọng

> ⚠️ Tính năng Proxy + GPS nằm trong gói. Nếu gói không bật, hệ thống báo "Gói không hỗ trợ".

> 💡 **Kinh nghiệm:** Dùng **proxy residential trả phí** (giả IP gia đình), bật từng đợt **10–20 máy** một lần. Proxy miễn phí thường đã bị TikTok/Facebook chặn sẵn.

> ⚠️ **Múi giờ vẫn là VN khi dùng proxy US:** Bạn chỉ bấm "Kích hoạt" nên chưa áp dụng giả lập. Hãy bấm **"Áp dụng & Chạy"** thay vì chỉ "Kích hoạt".
