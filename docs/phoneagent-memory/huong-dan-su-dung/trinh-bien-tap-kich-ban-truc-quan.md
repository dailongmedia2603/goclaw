# Trình biên tập kịch bản (kéo-thả)

Giao diện kéo-thả để bạn "vẽ" kịch bản bằng các **khối (node)** và **đường nối (edge)**. Hiển thị toàn màn hình (ẩn sidebar). Bố cục 3 cột: **Palette** (trái) – **Canvas** (giữa) – **Property Panel** (phải).

## Thanh header

Từ trái sang phải:
- **◀** — quay về danh sách. Nếu có thay đổi chưa lưu, hiện modal cảnh báo.
- **Ô "Tên kịch bản..."** — bắt buộc nhập để lưu.
- **Dropdown chọn kênh**: TikTok VN / TikTok US / Facebook.
- **Ô "Mô tả..."** — tuỳ chọn.
- **Dropdown chọn máy để pick toạ độ** — chỉ hiện máy online. Khi bấm "Chọn vị trí tap", màn hình VNC máy này mở ra.
- **💾 Lưu** — lưu tất cả thay đổi.

## Palette — Danh sách node theo 4 nhóm

### 🟢 Nhóm Cơ bản

| Node | Thông số | Ý nghĩa |
|------|----------|---------|
| ▶ Bắt đầu | — | Điểm xuất phát. Luôn có sẵn, không thể xoá |
| ⏱ Chờ | `seconds` (mặc định 5) | Dừng N giây |
| 🔀 Chờ ngẫu nhiên | `min` / `max` (3-8) | Dừng ngẫu nhiên — tránh bị phát hiện bot |
| 🔁 Lặp lại | `count` (mặc định 5) | Vòng lặp có **2 output**: *body* (lặp) & *next* (sau khi lặp xong) |
| 📷 Chụp ảnh | — | Capture màn hình, lưu vào nhật ký |
| ⬆ Truyền File | Chọn video từ Thư viện | Truyền video từ đám mây sang iPhone (chờ tối đa 20 phút) |
| 🗑 Xoá Photos | `deleteX, deleteY` (0-1) | Xoá ảnh/video trong thư viện iPhone |

### 🔵 Nhóm Điều khiển

| Node | Thông số | Ý nghĩa |
|------|----------|---------|
| 🏠 Nút Home | — | Bấm Home |
| ↑ Lướt lên | Mặc định | Vuốt từ dưới lên |
| ↓ Lướt xuống | Mặc định | Vuốt từ trên xuống |
| ← Lướt trái | Mặc định | Vuốt trái (bài kế) |
| → Lướt phải | Mặc định | Vuốt phải (bài trước) |
| Vuốt toạ độ | `fromX/Y, toX/Y, duration` | Swipe tuỳ chỉnh |
| ⌨ Gõ text | `text` | Gõ chuỗi vào ô đang focus |
| ✚ Tap toạ độ | `x, y` | Bấm chính xác (custom) |

### 🟣 Nhóm Ứng dụng

| Khối | Tác dụng |
|------|----------|
| 🎵 Mở TikTok VN | Khởi động TikTok bản Việt |
| 🌐 Mở TikTok US | Khởi động TikTok quốc tế |
| f Mở Facebook | Mở Facebook |
| 📧 Mở Mail | Mở Mail mặc định iOS |
| Login Facebook | Tự nhập username + password cùng lúc |
| Nhập tài khoản | Tự điền username vào ô bạn chỉ định |
| Nhập mật khẩu | Tự điền password |
| Nhập email | Tự điền email |
| Nhập pass mail | Tự điền pass mail |
| 📨 Lấy OTP Mail | Tự đọc email và điền OTP (yêu cầu Gmail đã kết nối, mặc định chờ 60s) |

### 🟠 Nhóm Tuỳ chỉnh

Các node bạn **tự tạo** bằng nút **"+ Thêm node"** — dùng cơ chế tap toạ độ, chỉ khác tên (VD "Nút chia sẻ", "Nút Like", "Nút Follow").

## Tạo node tuỳ chỉnh

1. Bấm **"+ Thêm"** màu cam ở đầu palette.
2. Nhập tên (VD "Nút chia sẻ").
3. Bấm **"Tạo node"** → xuất hiện trong nhóm Tuỳ chỉnh.
4. Kéo vào canvas → chọn vị trí tap.

Có thể đổi tên/xoá bằng icon edit/trash khi rê chuột vào node trong palette.

## Canvas — Thao tác

- **Thả node:** kéo từ palette → canvas → node mới hiện với tham số mặc định.
- **Chọn node:** click → viền xanh dương + Property Panel cập nhật.
- **Di chuyển:** kéo thả trong canvas.
- **Xoá node:** chọn + Delete/Backspace, hoặc bấm "🗑 Xoá node". **Node "Bắt đầu" KHÔNG thể xoá.**
- **Kết nối 2 node:** giữ handle dưới (output) của node A, kéo tới handle trên (input) của node B.
- **Xoá edge:** rê chuột vào giữa đường nối → nút X nhỏ hiện.
- **Pan:** kéo chuột phải/giữa.
- **Zoom:** lăn chuột hoặc nút +/-.
- **Fit view:** nút khung hình ở góc trái dưới.
- **Minimap:** góc phải dưới — click để nhảy đến.

> 💡 **Node "Lặp lại" có 2 output:**
> - **Handle phải — "body":** các node ở đây sẽ lặp N lần.
> - **Handle dưới — "next":** đường đi sau khi kết thúc vòng lặp.

## Chọn toạ độ bằng màn hình iPhone trực tiếp

1. **Điều kiện:** phải có máy online được chọn ở header.
2. Hệ thống kết nối và **hiển thị màn hình iPhone** trong cửa sổ lớn (mất 2 giây).
3. Cửa sổ toàn màn hình hiện màn hình thật. Dưới có "Con trỏ: x=123 y=456" (vị trí chuột) và "Đã chọn: x=123 y=456" (toạ độ đã lưu, màu đỏ).
4. **Click chuột** vào vị trí mong muốn → toạ độ được ghi.
5. **🖱 Test** — gửi cú chạm thử lên iPhone để kiểm tra.
6. **Xác nhận** — đóng cửa sổ, toạ độ tự điền vào property panel.

## Tự sắp xếp & Lưu kịch bản

Giữa canvas có 2 nút lơ lửng:
- **🔧 Sắp xếp gọn** — dàn tất cả khối theo hàng dọc, cách đều.
- **✨ Sáng tạo** — áp dụng ngẫu nhiên 1 trong 5 kiểu trang trí (Zigzag, Wave, Spiral, Staircase, Circle) — chỉ để nhìn đẹp.

**Lưu:** bấm **💾 Lưu** → kiểm tra tên không trống → "Đã lưu".

> ⚠️ **Cảnh báo thoát khi chưa lưu:** nếu có thay đổi mà bấm ◀ hoặc đóng tab, modal hiện cảnh báo "Chưa lưu kịch bản — thoát sẽ mất các thay đổi này." Chọn "Ở lại chỉnh sửa" hoặc "Thoát không lưu".

## ⏱ Auto pre-delay 5 giây (mới từ 04/2026)

**Mọi node thực thi sẽ tự động chờ 5 giây trước khi chạy** — để iPhone kịp load giao diện, tránh tap/swipe sai vị trí.

✅ **Áp dụng cho:**
- Tất cả node đang có sẵn trong palette
- Tất cả node trong các kịch bản đã lưu từ trước
- Node tuỳ chỉnh (`tap_x_custom`)

❌ **KHÔNG áp dụng cho:**
- Chờ / Chờ ngẫu nhiên — bạn tự quyết định
- Lặp lại — nhưng node BÊN TRONG loop vẫn nhận 5s pre-delay
- Bắt đầu — không thực thi gì

**Hiển thị trong UI:** mỗi node có badge `⏱ Chờ 5s trước` ngay dưới tên. Property Panel hiện *"Chờ tự động: 5 giây"* (chỉ thông tin, không tắt được).
