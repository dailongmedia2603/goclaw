# Menu Kho kịch bản (mẫu dùng chung)

Thư viện kịch bản mẫu do **Admin** tạo và chia sẻ cho tất cả user. User thường có thể **copy về dùng cá nhân**, Admin có quyền tạo/sửa/xoá.

## Đầu trang & thanh công cụ

- Tiêu đề **"Kho kịch bản"** + phụ đề "Kịch bản mẫu có sẵn, chia sẻ cho tất cả người dùng"
- (Chỉ Admin) 3 nút: Import / Export / **+ Tạo kịch bản**
- Tab kênh + ô tìm kiếm (giống menu Kịch bản)

## Bảng danh sách & phân quyền

| Cột | Nghĩa |
|-----|-------|
| Tên kịch bản | Tên mẫu |
| Mô tả | Ghi chú (2 dòng) |
| Số bước | Đếm node |
| Ngày tạo | Ngày tạo |
| Trạng thái | Luôn là "Sẵn sàng" |
| Thao tác (User) | Chỉ có **"Thêm"** (copy) |
| Thao tác (Admin) | Thêm: ▶ Chạy · ✎ Sửa · 🗑 Xoá |

## Copy kịch bản về kho cá nhân

User bấm **"Thêm"** → hệ thống tạo bản sao kịch bản mẫu vào `/kich-ban` cá nhân, giữ nguyên name/description/steps. Toast: *"Đã thêm '{name}' vào kịch bản của bạn"*.

Sau đó bạn mở ở menu Kịch bản để **tuỳ biến riêng** mà không ảnh hưởng bản gốc.

## Quyền Admin

| Hành động | Chi tiết |
|-----------|----------|
| **+ Tạo kịch bản** | Mở editor tại `/kho-kich-ban/tao-moi` — lưu vào database shared |
| **✎ Sửa** | Mở editor tại `/kho-kich-ban/:id` |
| **▶ Chạy** | Chạy trực tiếp lên thiết bị. **Lưu ý:** kịch bản shared do admin chạy lên máy của user X sẽ lưu execution vào tenant của X (để X vẫn thấy log) |
| **🗑 Xoá** | "Xoá kịch bản mẫu? Sẽ bị xoá khỏi kho và không thể hoàn tác." |
| **Import / Export** | Giống menu Kịch bản nhưng tác động vào kho shared |

## Khi nào nên dùng Kho kịch bản

- **Template khởi đầu** — Admin cung cấp SOP chuẩn (đăng bài, nuôi tương tác, onboarding).
- **User mới onboarding** — không biết bắt đầu từ đâu → copy mẫu → chỉnh sửa nhẹ → chạy ngay.
- **Đồng bộ team** — đảm bảo cả team dùng cùng quy trình chuẩn.
