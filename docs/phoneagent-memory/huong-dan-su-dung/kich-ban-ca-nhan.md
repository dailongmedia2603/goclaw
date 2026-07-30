# Menu Kịch bản (Kho cá nhân)

Kho kịch bản riêng của bạn — xem danh sách, chạy, xuất/nhập, mở vào trình biên tập.

## Đầu trang & thanh công cụ

- Tiêu đề **"Kịch bản của tôi"** + phụ đề "Quản lý các tập lệnh tự động cho thiết bị di động"
- 3 tab kênh: TikTok VN | TikTok US | Facebook (mỗi tab có badge đếm số kịch bản)
- Ô tìm kiếm
- **+ Tạo mới** → mở trình biên tập tại `/kich-ban/tao-moi`
- **Import** → chọn file `.json` đã export
- **Export** → modal chọn kịch bản → tải file `phoneclaw-scripts-YYYY-MM-DD.json`

## Bảng kịch bản

| Cột | Nghĩa |
|-----|-------|
| Tên kịch bản | Tên + tooltip khi rê chuột |
| Mô tả | Ghi chú ngắn |
| Số bước | Đếm node trong graph |
| Ngày sửa | "5 phút trước", "1 giờ trước", "2 ngày trước" |
| Trạng thái | Nháp / Hoạt động / Tạm dừng |
| Thao tác | ▶ Chạy · ✎ Chỉnh sửa · ⇩ Tải · 🗑 Xoá |

## Luồng chạy một kịch bản

1. Bấm **"▶ Chạy"** → cửa sổ chọn máy mở.
2. **Chọn nhiều thiết bị** — chỉ hiện máy online. Tick máy muốn chạy.
3. Bấm **"Chạy ngay"** → hệ thống bắt đầu chạy trên từng máy. Mỗi máy có tiến độ riêng.
4. **Cửa sổ theo dõi tự cập nhật** — bạn thấy bước nào đang chạy, bước nào đã xong.
5. Trạng thái: Đang chạy (xoay + nút Dừng) · Thành công (tick xanh + thời gian) · Thất bại (X đỏ + lý do).
6. Bấm **"Dừng"** → kịch bản kết thúc ngay sau bước hiện tại.
7. **Lưu vào Nhật ký** (giữ tối đa 500 lần chạy gần nhất).

## Xuất/Nhập kịch bản (JSON)

**Tải xuống 1 kịch bản:** bấm **⇩ Tải** → file JSON cấu trúc:

```json
{
  "version": 1,
  "exportedAt": "2026-04-11T12:34:56Z",
  "scripts": [{
    "name": "Login Flow",
    "description": "Tự động login TikTok",
    "channel": "tiktok_vn",
    "graphData": { "nodes": [], "edges": [] },
    "steps": []
  }]
}
```

**Import:** chọn file JSON → tạo bản sao mới (không ghi đè).

**Export hàng loạt:** modal hiện tất cả kịch bản với checkbox → tick → "Tải xuống" → file `phoneclaw-scripts-YYYY-MM-DD.json`.

## Xoá kịch bản

Bấm 🗑 → xác nhận.

> ⚠️ **Lịch trình đang dùng kịch bản này sẽ báo lỗi khi tới giờ chạy** — nên xoá/cập nhật lịch trình trước.

## Giới hạn gói

Mỗi gói có số kịch bản tối đa. Khi đạt giới hạn, nút **+ Tạo mới** vẫn bấm được nhưng API trả lỗi *"Đã đạt giới hạn kịch bản, vui lòng nâng cấp gói"*.
