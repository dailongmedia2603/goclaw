# Menu Thiết bị (Quản lý iPhone)

Trung tâm quản lý toàn bộ iPhone — nơi bạn thêm máy mới, gom nhóm, theo dõi trạng thái và xem màn hình trực tiếp.

## Đầu trang & banner gói

- Tiêu đề **"Quản lý Thiết bị"** + phụ đề "X thiết bị | Y online"
- Nút **"+ Thêm thiết bị"** (xanh dương, góc phải)
- Banner gói hiển thị: tên gói + số máy tối đa + thanh tiến trình "Đã kết nối: X/Y"
- Khi đầy 100% → banner chuyển đỏ + nút **"Yêu cầu nâng cấp"**

## Thanh công cụ & tab nhóm

- Ô tìm kiếm (theo tên hoặc IP)
- Tab "Tất cả" + các tab nhóm bạn đã tạo (VD "TikTok Farm", "Facebook Team", "Chưa phân nhóm")
- Nút **"+ Thêm nhóm"**
- Rê chuột vào tab nhóm sẽ hiện X nhỏ để xoá nhóm

## Bảng danh sách iPhone (7 cột)

| Cột | Nghĩa |
|-----|-------|
| ☑ Checkbox | Chọn hàng loạt (có "chọn tất cả" ở header) |
| Tên thiết bị | Tên + tên nhóm. Icon xanh = online, xám = offline |
| IP Tailscale | Địa chỉ nội bộ `100.x.y.z` |
| iOS Version | VD "iOS 17.4" |
| Account | Nick MXH đang login (hoặc "-") |
| Status | Xem bảng trạng thái bên dưới |
| Thao tác | 2 icon Sửa / Xoá. Bấm vào hàng (không phải icon) sẽ mở chi tiết |

## Bảng trạng thái thiết bị

| Trạng thái | Nghĩa |
|------------|-------|
| 🔵 Đang kết nối | Vừa thêm, đang thiết lập kết nối |
| 🟢 Online | Sẵn sàng nhận lệnh |
| 🟣 Running | Đang chạy kịch bản |
| ⚫ Offline | Mất kết nối > 90 giây |

Nếu máy đang bật proxy, dòng phụ dưới badge sẽ hiện "Proxy US" hoặc "Proxy VN". Đang chạy kịch bản thì có tên kịch bản hiện ra.

## Cửa sổ "Thêm thiết bị mới"

Bấm **"+ Thêm thiết bị"** → modal mở. Điền:

| Trường | Bắt buộc? | Điền gì |
|--------|-----------|---------|
| Tên thiết bị | ✅ | Đặt tên gợi nhớ: `TT-VN-01`, `iPhone-13-Pro-01` |
| IP Tailscale | ✅ | Dạng `100.96.65.5` — đội ngũ đã cấp khi bàn giao máy |
| Mật khẩu hiển thị màn hình | ✅ | Đội ngũ cài sẵn — phải nhập trùng chính xác |
| Tài khoản | ❌ | Gán nick MXH cho máy này (có thể để trống) |
| Nhóm | ❌ | Chọn nhóm hoặc "Chưa phân nhóm" |

Bấm **"Thêm thiết bị"** → hệ thống kiểm tra giới hạn gói và kết nối lần đầu.

## Cửa sổ chi tiết thiết bị (2 cột)

Bấm vào hàng (không phải icon) → cửa sổ mở.

**Cột trái — Xem màn hình trực tiếp:**
- Online: thấy màn hình iPhone live + nhãn "Đang trực tiếp" + link "Mở toàn màn hình"
- Offline: hiện icon mất sóng + chữ "Thiết bị ngoại tuyến"

**Cột phải — Bảng điều khiển:**
- Thông tin máy (IP, iOS, Model)
- 6 nút điều khiển nhanh: Home · Trên xuống · Dưới lên · Lướt trái · Lướt phải · Chụp ảnh

Dưới cùng có link **"Sửa thông tin"** (xanh) và **"Xoá thiết bị"** (đỏ).

## Xoá thiết bị

Bấm icon thùng rác → cửa sổ xác nhận. Khi xoá:

- Bản ghi mất khỏi hệ thống
- KHÔNG xoá dữ liệu trên iPhone — máy có thể đăng ký lại sau
- Lịch trình đang trỏ đến máy đó sẽ báo lỗi "Device offline" khi tới giờ

## Tạo nhóm thiết bị

Bấm **"+ Thêm nhóm"** → cửa sổ nhỏ:

- Nhập tên nhóm (gợi ý: "TikTok Farm", "Facebook Team")
- Bấm Enter hoặc **"Tạo nhóm"** → nhóm mới xuất hiện ngay trong tab lọc

## Điều khiển nhiều máy cùng lúc

Phone-Agent hỗ trợ xem & điều khiển song song:

- 1 máy → toàn màn hình
- 4 máy → lưới 2×2
- 9 máy → lưới 3×3
- 16 máy → lưới 4×4

Mỗi ô tự cập nhật ảnh **mỗi 5 giây**. Máy đang chọn có viền xanh + bảng điều khiển bên phải:

- 4 nút cơ bản: Home · Back · Chụp ảnh · Gõ chữ
- Mở app nhanh: TikTok · Facebook · Zalo · YouTube · Instagram · Shopee
- Đồng bộ clipboard: "Gửi đến ĐT" (đẩy text từ máy tính sang iPhone) · "Lấy từ ĐT" (đọc clipboard iPhone)
