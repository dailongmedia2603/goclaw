# Menu Nhật ký (xem lịch sử)

Trung tâm xem **toàn bộ lịch sử** chạy kịch bản — từng bước được làm ra sao, có ảnh chụp, có lỗi gì, mất bao lâu.

## Đầu trang & Export

- Tiêu đề **"Nhật ký"** + phụ đề "Theo dõi lịch sử chạy kịch bản"
- Nút **"Tự động dọn"** (icon Settings) — mở modal cấu hình dọn dẹp
- Nút **"Export"** (xanh dương, ChevronDown) — dropdown:
  - Xuất Log (CSV)
  - Xuất Log (JSON)
  - Tải Ảnh (ZIP)

## Danh sách log dạng Accordion

Mỗi log là một hàng có thể mở rộng. Khi chưa mở thấy:

- Mũi tên ► (chưa mở) hoặc ▼ (đã mở)
- **Tên kịch bản** (đậm)
- Badge: ✓ Thành công / ⟳ Đang chạy / ✗ Lỗi
- Thông tin dòng dưới: tên máy · thời gian bắt đầu · thời lượng

## Chi tiết bước (khi mở rộng)

Click → thấy **timeline dọc từng bước**:

- Icon tròn màu theo trạng thái: Success / Error / Running / Pending
- Tiêu đề: "BƯỚC {index}" + thời gian
- Mô tả hành động (font mono): `tap(0.5, 0.7)`, `wait(5s)`, `launch(com.zhiliaoapp.musically)`…
- Lỗi nếu có: hộp đỏ "Lỗi: [thông báo]"
- Ảnh chụp nếu có: thumbnail 48×80 — click mở lightbox

## Lightbox xem ảnh

Click thumbnail → modal full-screen nền đen:

- Ảnh phóng to tối đa
- Metadata bên dưới: tên thiết bị, thời gian, tên kịch bản
- Nút **"Tải xuống"** — lưu ảnh về máy
- Nút **"Xoá ảnh"** — xoá ảnh khỏi bộ nhớ + nhật ký
- Nút X góc phải để đóng

## Cửa sổ "Cấu hình dọn dẹp tự động"

Bấm **"Tự động dọn"**:

- Info box xanh: "Hệ thống sẽ tự xoá dữ liệu cũ để giải phóng dung lượng. Hành động này không thể hoàn tác."
- "Nhật ký kịch bản": ô nhập số ngày + đơn vị "ngày" (mặc định 30). Ghi chú: "Xoá nhật ký cũ hơn số ngày chỉ định"
- Nút **"Dọn dẹp ngay"** (đỏ) — chạy cleanup ngay
- Nút **"Huỷ"** và **"Lưu cấu hình"**

## Giới hạn lưu trữ (hệ thống tự dọn)

| Loại | Giới hạn | Cơ chế |
|------|----------|--------|
| Nhật ký hoạt động | 1000 bản ghi/tài khoản | Bản cũ nhất bị xoá trước |
| Ảnh chụp màn hình | 500 bản ghi/tài khoản | Bản cũ nhất xoá + xoá ảnh trên đám mây |
| Lịch sử chạy | 500 bản ghi/tài khoản | Bản cũ nhất xoá trước |
| Thông báo | 200 bản ghi | Toàn hệ thống, bản cũ nhất xoá |

Ảnh thực tế lưu trên **bộ nhớ đám mây** — khi bản ghi bị xoá, ảnh cũng bị xoá theo.
