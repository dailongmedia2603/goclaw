# CẨM NANG SỬ DỤNG PHONE-AGENT

> Hướng dẫn dành cho người dùng thực tế — viết bằng ngôn ngữ đời thường, không kỹ thuật. Mỗi menu đều có giải thích từng nút bấm, ý nghĩa, và việc bạn nên làm.
>
> **Phiên bản:** 2.0 — Cập nhật 2026-04-20  
> **Đối tượng:** Affiliate marketer, người vận hành farm iPhone, đội nuôi kênh TikTok / Facebook.  
> **Bản chi tiết hơn (có hình ảnh):** [Cẩm nang HTML](huong-dan-html/index.html)  
> **Tích hợp API cho phần mềm khác:** [API.md](API.md)

---

## MỤC LỤC

- [Chương 0 — Phone-Agent là gì? Bạn làm được gì với nó?](#chương-0--phone-agent-là-gì-bạn-làm-được-gì-với-nó)
- [Chương 1 — Đăng ký, đăng nhập & bố cục dashboard](#chương-1--đăng-ký-đăng-nhập--bố-cục-dashboard)
- [Chương 2 — Menu Dashboard (Tổng quan)](#chương-2--menu-dashboard-tổng-quan)
- [Chương 3 — Menu Thiết bị (Quản lý iPhone)](#chương-3--menu-thiết-bị-quản-lý-iphone)
- [Chương 4 — Menu Proxy (Đổi IP & chống phát hiện)](#chương-4--menu-proxy-đổi-ip--chống-phát-hiện)
- [Chương 5 — Menu Tài khoản (Lưu nick MXH & lấy OTP)](#chương-5--menu-tài-khoản-lưu-nick-mxh--lấy-otp)
- [Chương 6 — Menu Kịch bản (Kho cá nhân)](#chương-6--menu-kịch-bản-kho-cá-nhân)
- [Chương 7 — Trình biên tập kịch bản (kéo-thả)](#chương-7--trình-biên-tập-kịch-bản-kéo-thả)
- [Chương 8 — Menu Kho kịch bản (mẫu dùng chung)](#chương-8--menu-kho-kịch-bản-mẫu-dùng-chung)
- [Chương 9 — Menu Lịch trình (chạy tự động theo giờ)](#chương-9--menu-lịch-trình-chạy-tự-động-theo-giờ)
- [Chương 10 — Menu Nhật ký (xem lịch sử)](#chương-10--menu-nhật-ký-xem-lịch-sử)
- [Chương 11 — Menu Thư viện Video](#chương-11--menu-thư-viện-video)
- [Chương 12 — Menu Cài đặt cá nhân](#chương-12--menu-cài-đặt-cá-nhân)
- [Chương 13 — 10 luồng làm việc thực tế](#chương-13--10-luồng-làm-việc-thực-tế)
- [Chương 14 — Mẹo & thủ thuật](#chương-14--mẹo--thủ-thuật)
- [Chương 15 — Câu hỏi thường gặp (FAQ)](#chương-15--câu-hỏi-thường-gặp-faq)
- [Chương 16 — Bảng tra nhanh khi gặp sự cố](#chương-16--bảng-tra-nhanh-khi-gặp-sự-cố)

---

## Chương 0 — Phone-Agent là gì? Bạn làm được gì với nó?

### Nói ngắn gọn

**Phone-Agent là cái dashboard web** giúp bạn ngồi một chỗ, mở trình duyệt, rồi điều khiển hàng chục — hàng trăm chiếc iPhone thật ở nhà/văn phòng cùng lúc. Thay vì cầm từng máy bấm tay, bạn vẽ "công thức" thao tác bằng cách kéo-thả các khối, rồi bấm chạy — máy tự làm.

Ví dụ: vẽ kịch bản "Mở TikTok → lướt 20 lần → like 5 video", bấm chạy → 10 iPhone tự làm cùng lúc, mỗi máy dùng một nick khác nhau.

**Quan trọng:** Phone-Agent **không bẻ khoá** iPhone. Máy vẫn còn bảo hành, vẫn cập nhật iOS bình thường.

### Bạn làm được những gì?

| Tính năng | Có nghĩa là |
|-----------|-------------|
| Quản lý tập trung | Một màn hình thấy hết: máy nào đang bật, pin còn bao nhiêu, đang chạy gì |
| Điều khiển từ xa | Xem trực tiếp màn hình iPhone, bấm Home, vuốt, gõ chữ, chụp ảnh từ xa |
| Tự động hoá | Vẽ kịch bản kéo-thả: tap, swipe, gõ chữ, mở app, chờ, lặp lại |
| Chạy song song | Một kịch bản chạy đồng thời trên 1, 4, 9, 16 máy (hoặc nhiều hơn) |
| Hẹn giờ 24/7 | Đặt lịch chạy 8h sáng, mỗi giờ, mỗi ngày — máy tự làm khi bạn ngủ |
| Đổi IP / nguỵ trang | Gán proxy US/VN cho máy, giả lập múi giờ, ngôn ngữ, GPS để tránh bị phát hiện |
| Quản lý nick MXH | Lưu username/password TikTok, Facebook, Instagram cùng email OTP |
| Thư viện video | Upload video lên đám mây, truyền sang iPhone để đăng bài |
| Cảnh báo Telegram | Nhận tin nhắn khi máy rớt mạng hoặc kịch bản chạy lỗi |
| API mở | Phần mềm khác có thể gọi vào Phone-Agent để chạy kịch bản tự động |

### Hệ thống chạy ra sao?

```
[Trình duyệt của bạn] ──► [Phone-Agent (trên cloud)] ──► [iPhone của bạn (qua mạng riêng Tailscale)]
```

- Bạn ngồi máy tính, mở trang `phoneagent.io.vn`.
- Phone-Agent kết nối tới iPhone qua **mạng riêng Tailscale** — bảo mật, không lộ IP thật.
- Ảnh chụp, video lưu trên **bộ nhớ đám mây** — không làm đầy iPhone.

### Thuật ngữ cần biết trước

| Từ | Nghĩa đơn giản |
|----|----------------|
| **Thiết bị** | Một chiếc iPhone đã kết nối vào hệ thống |
| **Nhóm thiết bị** | Tập hợp iPhone gom lại theo mục đích (VD "TikTok Farm") |
| **Kịch bản** | "Công thức" iPhone sẽ làm theo từng bước |
| **Node** | Một khối hành động trong kịch bản (tap, gõ, chờ…) |
| **Kênh** | Loại MXH kịch bản nhắm tới: TikTok VN / TikTok US / Facebook |
| **Lịch trình** | Quy tắc chạy kịch bản tự động theo giờ |
| **Execution** | Một lần chạy cụ thể của kịch bản |
| **Proxy** | Máy chủ trung gian dùng để đổi IP cho iPhone |
| **Spoof** | Giả lập múi giờ, ngôn ngữ, GPS để iPhone "trông giống" ở nước khác |
| **Tín hiệu kết nối** | iPhone tự báo "còn sống" về hệ thống — mất quá 90 giây sẽ bị tính là offline |
| **Không gian riêng** | Mỗi tài khoản có dữ liệu độc lập, chỉ thấy đồ của mình |
| **Gói sử dụng** | Gói quy định bạn được bao nhiêu máy, bao nhiêu kịch bản, có proxy không… |

---

## Chương 1 — Đăng ký, đăng nhập & bố cục dashboard

### 1.1 Trang chủ & form đăng ký

Truy cập `phoneagent.io.vn` — bạn thấy trang giới thiệu có 4 nút chính:

- **Đăng nhập** — vào dashboard nếu đã có tài khoản
- **Đăng ký** — gửi yêu cầu để đội ngũ liên hệ tư vấn
- **Xem Demo** — xem video minh hoạ
- **Bắt đầu miễn phí** — cuộn xuống form đăng ký

**Form đăng ký** có 4 trường:

| Trường | Bắt buộc? | Ghi chú |
|--------|-----------|---------|
| Họ tên | ✅ | VD "Nguyễn Văn A" |
| Số điện thoại | ✅ | VD "0912 345 678" — đội ngũ sẽ gọi tư vấn |
| Gói đăng ký | ❌ | Starter / Basic / Pro / Business — có thể bỏ qua |
| Mục đích sử dụng | ❌ | Mô tả ngắn — VD "Nuôi nick TikTok VN, cần 10 máy" |

Bấm **"Đăng ký ngay"** → đội ngũ phản hồi qua Telegram trong 1–2 giờ làm việc, gửi email + mật khẩu cho bạn.

### 1.2 Đăng nhập lần đầu

Vào `phoneagent.io.vn/login`. Nhập:

- **Email** — đội ngũ đã cấp
- **Mật khẩu** — bấm 👁 để hiện/ẩn

> 💡 **Đổi mật khẩu ngay** sau khi đăng nhập lần đầu! Vào *Cài đặt → Thông tin cá nhân → Đổi mật khẩu*.

### 1.3 Bố cục dashboard

Sau khi đăng nhập, màn hình chia 2 phần:

**Cột trái — Thanh menu:**

| # | Menu | Dùng để |
|---|------|---------|
| 1 | 📊 Dashboard | Xem tổng quan |
| 2 | 📱 Thiết bị | Quản lý iPhone |
| 3 | 🛡 Proxy | Đổi IP (nếu gói có) |
| 4 | 👤 Tài khoản | Lưu nick MXH |
| 5 | 🪄 Kịch bản | Kho kịch bản cá nhân |
| 6 | 📖 Kho kịch bản | Mẫu dùng chung |
| 7 | 📅 Lịch trình | Hẹn giờ chạy tự động |
| 8 | 📜 Nhật ký | Xem lịch sử |
| 9 | 🎞 Thư viện Video | Upload video |
| 10 | ⚙ Cài đặt | Đổi mật khẩu, cấu hình thông báo |

Có thể thu gọn (chỉ icon) hoặc mở rộng (có chữ). Dưới cùng là tên tài khoản + nút **Đăng xuất** (chữ đỏ).

**Cột phải — Nội dung:**

Mỗi trang đều có:
- Tiêu đề trang (chữ lớn ở trên)
- Thanh công cụ (tìm kiếm, lọc, nút Thêm)
- Nội dung chính (bảng hoặc lưới thẻ)
- Cửa sổ nổi (khi bấm thêm/sửa/xoá)
- Thông báo nhỏ ở góc dưới-phải, tự biến mất sau 4 giây

**Bảng màu thông báo:**

| Màu | Nghĩa |
|-----|-------|
| 🟢 Xanh lá | Thành công |
| 🔴 Đỏ | Lỗi |
| 🟡 Vàng | Cảnh báo |
| 🔵 Xanh dương | Thông tin |

> 💡 Nếu bạn không thấy menu **Người dùng / Gói / Máy chủ** — đó là menu chỉ admin mới có. Bạn vẫn có đủ menu cần dùng hàng ngày.

---

## Chương 2 — Menu Dashboard (Tổng quan)

Trang đầu tiên khi đăng nhập. Cho cái nhìn tổng thể trong vài giây.

### 2.1 Bốn thẻ số liệu (trên cùng)

Tự cập nhật theo thời gian thực — không cần tải lại trang.

| Thẻ | Màu | Nghĩa |
|-----|-----|-------|
| Tổng thiết bị | 🔵 Xanh dương | Tổng iPhone đã đăng ký |
| Đang Online | 🟢 Xanh lá | Số máy sẵn sàng nhận lệnh |
| Ngoại tuyến | 🔴 Đỏ | Số máy mất kết nối > 90 giây |
| Đang chạy kịch bản | 🟣 Tím | Số kịch bản đang thực thi |

### 2.2 Hai biểu đồ song song

- **Thiết bị Online (24h)** — biểu đồ đường, cho biết "giờ vàng" và "giờ rớt mạng" trong ngày.
- **Kịch bản đã chạy (7 ngày)** — biểu đồ cột tím, mỗi cột là 1 ngày trong tuần. Rê chuột để xem chi tiết.

### 2.3 Thông báo gần đây

Phần dưới cùng. Hiển thị tối đa 20 thông báo mới nhất với:
- Icon màu (đỏ = lỗi, vàng = offline, xanh lá = thành công, xanh dương = thông tin)
- Nội dung ngắn
- Thời gian tương đối ("Vừa xong", "5 phút trước", "2 giờ trước")

Bấm **"Xem tất cả"** ở góc phải để xem lịch sử đầy đủ.

### 2.4 Khi nào hệ thống tạo thông báo?

1. Thiết bị chuyển từ online → offline (nếu bạn đã bật cảnh báo).
2. Kịch bản chạy lỗi.
3. Kịch bản chạy thành công (nếu đã bật).

> 📲 Nếu đã cấu hình Telegram (xem [Chương 12.2](#122-tab-thông-báo)), thông báo cũng được gửi qua Telegram kèm emoji.

---

## Chương 3 — Menu Thiết bị (Quản lý iPhone)

Trung tâm quản lý toàn bộ iPhone — nơi bạn thêm máy mới, gom nhóm, theo dõi trạng thái và xem màn hình trực tiếp.

### 3.1 Đầu trang & banner gói

- Tiêu đề **"Quản lý Thiết bị"** + phụ đề "X thiết bị | Y online"
- Nút **"+ Thêm thiết bị"** (xanh dương, góc phải)
- Banner gói hiển thị: tên gói + số máy tối đa + thanh tiến trình "Đã kết nối: X/Y"
- Khi đầy 100% → banner chuyển đỏ + nút **"Yêu cầu nâng cấp"**

### 3.2 Thanh công cụ & tab nhóm

- Ô tìm kiếm (theo tên hoặc IP)
- Tab "Tất cả" + các tab nhóm bạn đã tạo (VD "TikTok Farm", "Facebook Team", "Chưa phân nhóm")
- Nút **"+ Thêm nhóm"**
- Rê chuột vào tab nhóm sẽ hiện X nhỏ để xoá nhóm

### 3.3 Bảng danh sách iPhone (7 cột)

| Cột | Nghĩa |
|-----|-------|
| ☑ Checkbox | Chọn hàng loạt (có "chọn tất cả" ở header) |
| Tên thiết bị | Tên + tên nhóm. Icon xanh = online, xám = offline |
| IP Tailscale | Địa chỉ nội bộ `100.x.y.z` |
| iOS Version | VD "iOS 17.4" |
| Account | Nick MXH đang login (hoặc "-") |
| Status | Xem [bảng trạng thái](#34-bảng-trạng-thái-thiết-bị) bên dưới |
| Thao tác | 2 icon Sửa / Xoá. Bấm vào hàng (không phải icon) sẽ mở chi tiết |

### 3.4 Bảng trạng thái thiết bị

| Trạng thái | Nghĩa |
|------------|-------|
| 🔵 Đang kết nối | Vừa thêm, đang thiết lập kết nối |
| 🟢 Online | Sẵn sàng nhận lệnh |
| 🟣 Running | Đang chạy kịch bản |
| ⚫ Offline | Mất kết nối > 90 giây |

Nếu máy đang bật proxy, dòng phụ dưới badge sẽ hiện "Proxy US" hoặc "Proxy VN". Đang chạy kịch bản thì có tên kịch bản hiện ra.

### 3.5 Cửa sổ "Thêm thiết bị mới"

Bấm **"+ Thêm thiết bị"** → modal mở. Điền:

| Trường | Bắt buộc? | Điền gì |
|--------|-----------|---------|
| Tên thiết bị | ✅ | Đặt tên gợi nhớ: `TT-VN-01`, `iPhone-13-Pro-01` |
| IP Tailscale | ✅ | Dạng `100.96.65.5` — đội ngũ đã cấp khi bàn giao máy |
| Mật khẩu hiển thị màn hình | ✅ | Đội ngũ cài sẵn — phải nhập trùng chính xác |
| Tài khoản | ❌ | Gán nick MXH cho máy này (có thể để trống) |
| Nhóm | ❌ | Chọn nhóm hoặc "Chưa phân nhóm" |

Bấm **"Thêm thiết bị"** → hệ thống kiểm tra giới hạn gói và kết nối lần đầu.

### 3.6 Cửa sổ chi tiết thiết bị (2 cột)

Bấm vào hàng (không phải icon) → cửa sổ mở:

**Cột trái — Xem màn hình trực tiếp:**
- Online: thấy màn hình iPhone live + nhãn "Đang trực tiếp" + link "Mở toàn màn hình"
- Offline: hiện icon mất sóng + chữ "Thiết bị ngoại tuyến"

**Cột phải — Bảng điều khiển:**
- Thông tin máy (IP, iOS, Model)
- 6 nút điều khiển nhanh: Home · Trên xuống · Dưới lên · Lướt trái · Lướt phải · Chụp ảnh

Dưới cùng có link **"Sửa thông tin"** (xanh) và **"Xoá thiết bị"** (đỏ).

### 3.7 Xoá thiết bị

Bấm icon thùng rác → cửa sổ xác nhận. Khi xoá:
- Bản ghi mất khỏi hệ thống
- KHÔNG xoá dữ liệu trên iPhone — máy có thể đăng ký lại sau
- Lịch trình đang trỏ đến máy đó sẽ báo lỗi "Device offline" khi tới giờ

### 3.8 Tạo nhóm thiết bị

Bấm **"+ Thêm nhóm"** → cửa sổ nhỏ:
- Nhập tên nhóm (gợi ý: "TikTok Farm", "Facebook Team")
- Bấm Enter hoặc **"Tạo nhóm"** → nhóm mới xuất hiện ngay trong tab lọc

### 3.9 Điều khiển nhiều máy cùng lúc

Phone-Agent hỗ trợ xem & điều khiển song song:

- 1 máy → toàn màn hình
- 4 máy → lưới 2×2
- 9 máy → lưới 3×3
- 16 máy → lưới 4×4

Mỗi ô tự cập nhật ảnh **mỗi 5 giây**. Máy đang chọn có viền xanh + bảng điều khiển bên phải:
- 4 nút cơ bản: Home · Back · Chụp ảnh · Gõ chữ
- Mở app nhanh: TikTok · Facebook · Zalo · YouTube · Instagram · Shopee
- Đồng bộ clipboard: "Gửi đến ĐT" (đẩy text từ máy tính sang iPhone) · "Lấy từ ĐT" (đọc clipboard iPhone)

---

## Chương 4 — Menu Proxy (Đổi IP & chống phát hiện)

Đổi IP cho từng máy hoặc cả nhóm. Quan trọng khi nuôi nick TikTok US, Facebook US mà farm thật đặt ở Việt Nam — giúp tài khoản "trông giống" người dùng bản địa.

### 4.1 Đầu trang & thống kê

- Tiêu đề **"Proxy Manager"**
- Số liệu: Tổng thiết bị · Số online · Số proxy đang chạy
- Nút **"Làm mới"** — tải lại trạng thái

### 4.2 Ba chế độ đổi IP

Chọn 1 trong 3:

| Chế độ | Dùng khi | Hệ thống làm gì |
|--------|----------|------------------|
| 🇺🇸 **Proxy US** | Nuôi nick TikTok US, Facebook US | Đổi IP sang Mỹ + giả lập múi giờ + ngôn ngữ + GPS sang Mỹ |
| 🇻🇳 **Proxy VN** | Nuôi nick TikTok VN | Đổi IP nội địa + GPS khớp vị trí proxy + ẩn WiFi, giữ tiếng Việt |
| ❌ **Không proxy** | Trở về mặc định | Tắt proxy + xoá toàn bộ giả lập |

### 4.3 Bảng cấu hình 2 cột (chỉ hiện khi chọn US/VN)

**Cột trái — Thông tin proxy:**

| Trường | Điền gì |
|--------|---------|
| Ô dán nhanh | Dán chuỗi `ip:port:user:pass` → tự tách vào 4 ô |
| Địa chỉ IP | VD `1.2.3.4` |
| Cổng | VD `8080` |
| Tên đăng nhập | Nhà cung cấp proxy cấp |
| Mật khẩu | Nhà cung cấp proxy cấp |

**Cột phải — Giả lập chống phát hiện:**

*Nếu chọn Proxy US:*
- Nút **"Tự động phát hiện"** — hệ thống tự tra IP, điền giúp múi giờ + GPS
- Múi giờ: Eastern / Central / Mountain / Pacific
- GPS: chọn 1 trong 5 thành phố mẫu (New York, LA, Chicago, Houston, Phoenix)
- Tự động: ngôn ngữ `en_US`, ẩn WiFi, giả sóng di động Mỹ, DNS qua proxy

*Nếu chọn Proxy VN:*
- Nút **"Tự động phát hiện"** — lấy thành phố + GPS của IP Việt Nam
- Không có chọn múi giờ/ngôn ngữ — giữ tiếng Việt tự nhiên
- Tự động: GPS khớp vị trí proxy, ẩn WiFi

### 4.4 Hành động hàng loạt

| Hành động | Tác dụng |
|-----------|----------|
| Chọn tất cả / Bỏ chọn | Chọn toàn bộ máy online (offline không thể chọn) |
| **Áp dụng & Chạy** | Tên đổi theo chế độ: "Áp dụng US (X)" / "Áp dụng VN (X)" / "Tắt tất cả (X)" |
| **Dừng (X)** | Tắt proxy hàng loạt |
| Đếm số lượng | "Đã chọn X thiết bị" |

Nút "Áp dụng" bị mờ nếu: chưa chọn máy, chưa nhập IP/Port, hoặc máy đang chạy proxy.

### 4.5 Bảng thiết bị chi tiết

| Cột | Nghĩa |
|-----|-------|
| ☑ Chọn | Không thể tick máy offline |
| Thiết bị | Tên + IP nội bộ |
| Trạng thái | Online / Đang chạy / Đang kết nối / Ngoại tuyến |
| Proxy | IP:Cổng đã cấu hình hoặc "Chưa cấu hình" |
| Trạng thái Proxy | "Đang chạy" hoặc "Đã tắt" |
| Hành động | "Kích hoạt" (xanh) hoặc "Dừng" (đỏ) |

### 4.6 Khi bấm "Áp dụng & Chạy" — hệ thống làm gì?

1. **Lưu cấu hình** — ghi IP/cổng/user/pass vào từng máy.
2. **Bật proxy** — khởi động phần mềm đổi IP trên iPhone, chờ 2 giây xác minh.
3. **Áp dụng giả lập** — múi giờ, ngôn ngữ, GPS theo chế độ. Nếu chưa tự phát hiện sẽ tự tra IP.

Mỗi lỗi từng máy có thông báo riêng, máy còn lại vẫn tiếp tục. Cuối cùng: "Đã áp dụng Proxy US/VN cho X thiết bị".

### 4.7 Hành động đơn lẻ

| Nút | Tác dụng |
|-----|----------|
| **Kích hoạt** (xanh) | Chỉ bật proxy đã cấu hình sẵn |
| **Dừng** (đỏ) | Tắt proxy trên máy đó |

> ⚠️ Tính năng Proxy + GPS nằm trong gói. Nếu gói không bật, hệ thống báo "Gói không hỗ trợ".

> 💡 **Kinh nghiệm:** Dùng **proxy residential trả phí** (giả IP gia đình), bật từng đợt **10–20 máy** một lần. Proxy miễn phí thường đã bị TikTok/Facebook chặn sẵn.

---

## Chương 5 — Menu Tài khoản (Lưu nick MXH & lấy OTP)

Lưu username/password Facebook, TikTok, Instagram, YouTube để kịch bản dùng khi login tự động. Hỗ trợ kết nối Gmail để lấy OTP — không cần gõ tay.

### 5.1 Đầu trang & thanh công cụ

- Tiêu đề **"Quản lý Tài khoản"** + phụ đề "X tài khoản | Y hoạt động"
- Nút **"+ Thêm tài khoản"** (xanh dương)
- Ô tìm kiếm (theo username hoặc email)
- Tab kênh: Tất cả | Facebook | TikTok VN | TikTok US | Instagram | YouTube | Khác
- Bộ lọc trạng thái: Tất cả / Hoạt động / Khoá / Cấm

### 5.2 Bảng tài khoản (9 cột)

| Cột | Nghĩa |
|-----|-------|
| Tài khoản | Username (đậm) + tên kênh (xám) |
| Mật khẩu | Bị che `••••••••`. Icon 👁 hiện, 📋 copy |
| Kênh | Badge màu theo kênh |
| Email | Email liên kết + nút copy |
| Pass mail | Mật khẩu email (che/copy) |
| Thiết bị | Tên máy được gán (hoặc "-") |
| Trạng thái | Hoạt động / Khoá / Cấm |
| Gmail | "Kết nối" (chưa) hoặc "✓ Đã kết nối" |
| Thao tác | Sửa & xoá (hiện khi rê chuột) |

### 5.3 Cửa sổ Thêm/Sửa tài khoản

| Trường | Bắt buộc? | Điền gì |
|--------|-----------|---------|
| Tài khoản | ✅ | Username — VD `tiktok_user_01` |
| Mật khẩu | ❌ | Có thể để trống nếu dùng OAuth |
| Kênh | ✅ | Facebook / TikTok VN / TikTok US / IG / YT / Khác |
| Email | ❌ | Email liên kết (để lấy OTP) |
| Mật khẩu mail | ❌ | Password hộp mail |
| Thiết bị | ❌ | Máy chủ lực dùng nick này |
| Trạng thái | ❌ | Hoạt động / Khoá / Cấm — ghi chú nội bộ |
| Ghi chú | ❌ | Tự do — VD "Ngày tạo: 01/03/2026" |

### 5.4 Kết nối Gmail để lấy OTP tự động

1. **Admin** đã cấu hình Gmail OAuth sẵn cho hệ thống (làm 1 lần — bạn không cần làm).
2. **Bấm "Kết nối"** ở cột Gmail của tài khoản cần.
3. **Tab mới mở Google** → đăng nhập bằng chính email cần kết nối.
4. **Cấp quyền** cho Phone-Agent (chỉ đọc email, không gửi/xoá).
5. **Tự động quay về dashboard** — hệ thống lưu xong.
6. **Nhãn đổi thành "✓ Đã kết nối"** → trong kịch bản, node *"Lấy OTP Mail"* sẽ tự đọc email và điền mã.

### 5.5 Tự động chia tài khoản cho nhiều máy

Khi kịch bản chạy trên nhiều máy và node "Login" có nhiều tài khoản được chọn, hệ thống **chia mỗi máy một nick khác nhau theo thứ tự vòng**:

- Máy 1 → tài khoản 1
- Máy 2 → tài khoản 2
- Máy 3 → tài khoản 3
- … (hết → quay lại tài khoản 1)

> 💚 **Lợi ích:** 10 máy login 10 nick khác nhau **chỉ bằng 1 kịch bản** — không cần duplicate 10 bản.

### 5.6 Xoá tài khoản

Bấm icon thùng rác → xác nhận. **Tất cả kịch bản đang dùng tài khoản này sẽ báo lỗi ở node login** — hãy kiểm tra trước khi xoá.

---

## Chương 6 — Menu Kịch bản (Kho cá nhân)

Kho kịch bản riêng của bạn — xem danh sách, chạy, xuất/nhập, mở vào trình biên tập.

### 6.1 Đầu trang & thanh công cụ

- Tiêu đề **"Kịch bản của tôi"** + phụ đề "Quản lý các tập lệnh tự động cho thiết bị di động"
- 3 tab kênh: TikTok VN | TikTok US | Facebook (mỗi tab có badge đếm số kịch bản)
- Ô tìm kiếm
- **+ Tạo mới** → mở trình biên tập tại `/kich-ban/tao-moi`
- **Import** → chọn file `.json` đã export
- **Export** → modal chọn kịch bản → tải file `phoneclaw-scripts-YYYY-MM-DD.json`

### 6.2 Bảng kịch bản

| Cột | Nghĩa |
|-----|-------|
| Tên kịch bản | Tên + tooltip khi rê chuột |
| Mô tả | Ghi chú ngắn |
| Số bước | Đếm node trong graph |
| Ngày sửa | "5 phút trước", "1 giờ trước", "2 ngày trước" |
| Trạng thái | Nháp / Hoạt động / Tạm dừng |
| Thao tác | ▶ Chạy · ✎ Chỉnh sửa · ⇩ Tải · 🗑 Xoá |

### 6.3 Luồng chạy một kịch bản

1. Bấm **"▶ Chạy"** → cửa sổ chọn máy mở.
2. **Chọn nhiều thiết bị** — chỉ hiện máy online. Tick máy muốn chạy.
3. Bấm **"Chạy ngay"** → hệ thống bắt đầu chạy trên từng máy. Mỗi máy có tiến độ riêng.
4. **Cửa sổ theo dõi tự cập nhật** — bạn thấy bước nào đang chạy, bước nào đã xong.
5. Trạng thái: Đang chạy (xoay + nút Dừng) · Thành công (tick xanh + thời gian) · Thất bại (X đỏ + lý do).
6. Bấm **"Dừng"** → kịch bản kết thúc ngay sau bước hiện tại.
7. **Lưu vào Nhật ký** (giữ tối đa 500 lần chạy gần nhất).

### 6.4 Xuất/Nhập kịch bản (JSON)

**Tải xuống 1 kịch bản:** bấm **⇩ Tải** → file JSON cấu trúc:

```json
{
  "version": 1,
  "exportedAt": "2026-04-11T12:34:56Z",
  "scripts": [{
    "name": "Login Flow",
    "description": "Tự động login TikTok",
    "channel": "tiktok_vn",
    "graphData": { "nodes": [...], "edges": [...] },
    "steps": [...]
  }]
}
```

**Import:** chọn file JSON → tạo bản sao mới (không ghi đè).

**Export hàng loạt:** modal hiện tất cả kịch bản với checkbox → tick → "Tải xuống" → file `phoneclaw-scripts-YYYY-MM-DD.json`.

### 6.5 Xoá kịch bản

Bấm 🗑 → xác nhận. **Lịch trình đang dùng kịch bản này sẽ báo lỗi khi tới giờ chạy** — nên xoá/cập nhật lịch trình trước.

### 6.6 Giới hạn gói

Mỗi gói có số kịch bản tối đa. Khi đạt giới hạn, nút **+ Tạo mới** vẫn bấm được nhưng API trả lỗi *"Đã đạt giới hạn kịch bản, vui lòng nâng cấp gói"*.

---

## Chương 7 — Trình biên tập kịch bản (kéo-thả)

Giao diện kéo-thả để bạn "vẽ" kịch bản bằng các **khối (node)** và **đường nối (edge)**. Hiển thị toàn màn hình (ẩn sidebar). Bố cục 3 cột: **Palette** (trái) – **Canvas** (giữa) – **Property Panel** (phải).

### 7.1 Thanh header

Từ trái sang phải:
- **◀** — quay về danh sách. Nếu có thay đổi chưa lưu, hiện modal cảnh báo.
- **Ô "Tên kịch bản..."** — bắt buộc nhập để lưu.
- **Dropdown chọn kênh**: TikTok VN / TikTok US / Facebook.
- **Ô "Mô tả..."** — tuỳ chọn.
- **Dropdown chọn máy để pick toạ độ** — chỉ hiện máy online. Khi bấm "Chọn vị trí tap", màn hình VNC máy này mở ra.
- **💾 Lưu** — lưu tất cả thay đổi.

### 7.2 Palette — Danh sách node theo 4 nhóm

#### 🟢 Nhóm Cơ bản

| Node | Thông số | Ý nghĩa |
|------|----------|---------|
| ▶ Bắt đầu | — | Điểm xuất phát. Luôn có sẵn, không thể xoá |
| ⏱ Chờ | `seconds` (mặc định 5) | Dừng N giây |
| 🔀 Chờ ngẫu nhiên | `min` / `max` (3-8) | Dừng ngẫu nhiên — tránh bị phát hiện bot |
| 🔁 Lặp lại | `count` (mặc định 5) | Vòng lặp có **2 output**: *body* (lặp) & *next* (sau khi lặp xong) |
| 📷 Chụp ảnh | — | Capture màn hình, lưu vào nhật ký |
| ⬆ Truyền File | Chọn video từ Thư viện | Truyền video từ đám mây sang iPhone (chờ tối đa 20 phút) |
| 🗑 Xoá Photos | `deleteX, deleteY` (0-1) | Xoá ảnh/video trong thư viện iPhone |

#### 🔵 Nhóm Điều khiển

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

#### 🟣 Nhóm Ứng dụng

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

#### 🟠 Nhóm Tuỳ chỉnh

Các node bạn **tự tạo** bằng nút **"+ Thêm node"** — dùng cơ chế tap toạ độ, chỉ khác tên (VD "Nút chia sẻ", "Nút Like", "Nút Follow").

### 7.3 Tạo node tuỳ chỉnh

1. Bấm **"+ Thêm"** màu cam ở đầu palette.
2. Nhập tên (VD "Nút chia sẻ").
3. Bấm **"Tạo node"** → xuất hiện trong nhóm Tuỳ chỉnh.
4. Kéo vào canvas → chọn vị trí tap (xem [7.5](#75-chọn-toạ-độ-bằng-màn-hình-iphone-trực-tiếp)).

Có thể đổi tên/xoá bằng icon edit/trash khi rê chuột vào node trong palette.

### 7.4 Canvas — Thao tác

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

### 7.5 Chọn toạ độ bằng màn hình iPhone trực tiếp

1. **Điều kiện:** phải có máy online được chọn ở header.
2. Hệ thống kết nối và **hiển thị màn hình iPhone** trong cửa sổ lớn (mất 2 giây).
3. Cửa sổ toàn màn hình hiện màn hình thật. Dưới có "Con trỏ: x=123 y=456" (vị trí chuột) và "Đã chọn: x=123 y=456" (toạ độ đã lưu, màu đỏ).
4. **Click chuột** vào vị trí mong muốn → toạ độ được ghi.
5. **🖱 Test** — gửi cú chạm thử lên iPhone để kiểm tra.
6. **Xác nhận** — đóng cửa sổ, toạ độ tự điền vào property panel.

### 7.6 Tự sắp xếp & Lưu kịch bản

Giữa canvas có 2 nút lơ lửng:
- **🔧 Sắp xếp gọn** — dàn tất cả khối theo hàng dọc, cách đều.
- **✨ Sáng tạo** — áp dụng ngẫu nhiên 1 trong 5 kiểu trang trí (Zigzag, Wave, Spiral, Staircase, Circle) — chỉ để nhìn đẹp.

**Lưu:** bấm **💾 Lưu** → kiểm tra tên không trống → "Đã lưu".

> ⚠️ **Cảnh báo thoát khi chưa lưu:** nếu có thay đổi mà bấm ◀ hoặc đóng tab, modal hiện cảnh báo "Chưa lưu kịch bản — thoát sẽ mất các thay đổi này." Chọn "Ở lại chỉnh sửa" hoặc "Thoát không lưu".

### 7.7 ⏱ Auto pre-delay 5 giây (mới từ 04/2026)

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

---

## Chương 8 — Menu Kho kịch bản (mẫu dùng chung)

Thư viện kịch bản mẫu do **Admin** tạo và chia sẻ cho tất cả user. User thường có thể **copy về dùng cá nhân**, Admin có quyền tạo/sửa/xoá.

### 8.1 Đầu trang & thanh công cụ

- Tiêu đề **"Kho kịch bản"** + phụ đề "Kịch bản mẫu có sẵn, chia sẻ cho tất cả người dùng"
- (Chỉ Admin) 3 nút: Import / Export / **+ Tạo kịch bản**
- Tab kênh + ô tìm kiếm (giống menu Kịch bản)

### 8.2 Bảng danh sách & phân quyền

| Cột | Nghĩa |
|-----|-------|
| Tên kịch bản | Tên mẫu |
| Mô tả | Ghi chú (2 dòng) |
| Số bước | Đếm node |
| Ngày tạo | Ngày tạo |
| Trạng thái | Luôn là "Sẵn sàng" |
| Thao tác (User) | Chỉ có **"Thêm"** (copy) |
| Thao tác (Admin) | Thêm: ▶ Chạy · ✎ Sửa · 🗑 Xoá |

### 8.3 Copy kịch bản về kho cá nhân

User bấm **"Thêm"** → hệ thống tạo bản sao kịch bản mẫu vào `/kich-ban` cá nhân, giữ nguyên name/description/steps. Toast: *"Đã thêm '{name}' vào kịch bản của bạn"*.

Sau đó bạn mở ở menu Kịch bản để **tuỳ biến riêng** mà không ảnh hưởng bản gốc.

### 8.4 Quyền Admin

| Hành động | Chi tiết |
|-----------|----------|
| **+ Tạo kịch bản** | Mở editor tại `/kho-kich-ban/tao-moi` — lưu vào database shared |
| **✎ Sửa** | Mở editor tại `/kho-kich-ban/:id` |
| **▶ Chạy** | Chạy trực tiếp lên thiết bị. **Lưu ý:** kịch bản shared do admin chạy lên máy của user X sẽ lưu execution vào tenant của X (để X vẫn thấy log) |
| **🗑 Xoá** | "Xoá kịch bản mẫu? Sẽ bị xoá khỏi kho và không thể hoàn tác." |
| **Import / Export** | Giống menu Kịch bản nhưng tác động vào kho shared |

### 8.5 Khi nào nên dùng Kho kịch bản?

- **Template khởi đầu** — Admin cung cấp SOP chuẩn (đăng bài, nuôi tương tác, onboarding).
- **User mới onboarding** — không biết bắt đầu từ đâu → copy mẫu → chỉnh sửa nhẹ → chạy ngay.
- **Đồng bộ team** — đảm bảo cả team dùng cùng quy trình chuẩn.

---

## Chương 9 — Menu Lịch trình (chạy tự động theo giờ)

Hẹn giờ chạy kịch bản — cốt lõi của tự động hoá 24/7. Hệ thống có 1 scheduler chạy ngầm, **mỗi 60 giây kiểm tra một lần** và kích hoạt lịch nào khớp giờ hiện tại.

### 9.1 Bốn tab điều hướng

| Tab | Hiển thị |
|-----|----------|
| **Danh sách** | Bảng tất cả lịch trình đã tạo (mặc định khi vào) |
| **Lịch biểu** | Calendar dạng tháng — Tháng trước/Tiếp, Hôm nay |
| **Hàng đợi** | Execution đang chạy ngay bây giờ. Có badge số lượng |
| **Lịch sử chạy** | Execution đã kết thúc (success / error) |

### 9.2 Bảng "Danh sách" lịch trình

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

### 9.3 Bộ lọc & tìm kiếm

- Ô tìm kiếm: theo tên lịch hoặc tên kịch bản
- Nút **"Bộ lọc"** mở panel:
  - Kênh: Tất cả / TikTok VN / TikTok US / Facebook
  - Tần suất: Tất cả / Hàng giờ / Hàng ngày / Hàng tuần / Ngày tuỳ chỉnh
  - Thiết bị: Tất cả / [danh sách máy]
  - Nút Reset · Huỷ · Áp dụng

### 9.4 Cửa sổ "Tạo lịch trình mới"

| Trường | Điền gì |
|--------|---------|
| Tên lịch trình | VD "Đăng bài TikTok sáng" |
| Kịch bản | Dropdown chọn từ kho cá nhân |
| Thiết bị | Dropdown chọn máy (hoặc nhóm) |
| Tần suất | Radio 5 lựa chọn (xem dưới) |
| Giờ chạy | HH:MM, mặc định `08:00` |
| Ngày chạy | Chỉ hiện nếu chọn "Ngày tuỳ chỉnh" |

**5 tần suất:**

| Tần suất | Ý nghĩa |
|----------|---------|
| Một lần | Chạy đúng 1 lần (tự tắt sau đó) |
| Hàng giờ | Mỗi giờ vào phút MM |
| Hàng ngày | Mỗi ngày lúc HH:MM |
| Hàng tuần | Mỗi tuần vào thứ Hai lúc HH:MM |
| Ngày tuỳ chỉnh | Ngày cụ thể trong tháng/năm |

### 9.5 Hệ thống hẹn giờ làm việc thế nào?

1. Bộ hẹn giờ chạy ngầm, kiểm tra **mỗi 60 giây**.
2. Với mỗi lịch đang bật: so sánh thời gian hiện tại với giờ đã đặt.
3. Nếu khớp: tìm thiết bị tương ứng, **loại trừ máy offline**.
4. Chạy lần lượt trên từng máy (máy 1 xong mới sang máy 2).
5. Ghi lại thời điểm chạy và tính lần chạy kế tiếp.

> 💡 **Máy offline khi tới giờ?** Lịch bỏ qua máy đó và ghi vào nhật ký lỗi. Máy nào online thì vẫn chạy bình thường.

### 9.6 Tạm dừng (toggle) thay vì xoá

Nếu chỉ muốn **tạm dừng vài ngày**, bấm switch ở cột Bật/Tắt thay vì xoá — đặt `enabled = false` nhưng giữ cấu hình. Bật lại rất nhanh.

### 9.7 Xoá lịch trình

Icon thùng rác → xác nhận. Lịch trình biến mất nhưng **các execution đã chạy vẫn còn trong Nhật ký**.

---

## Chương 10 — Menu Nhật ký (xem lịch sử)

Trung tâm xem **toàn bộ lịch sử** chạy kịch bản — từng bước được làm ra sao, có ảnh chụp, có lỗi gì, mất bao lâu.

### 10.1 Đầu trang & Export

- Tiêu đề **"Nhật ký"** + phụ đề "Theo dõi lịch sử chạy kịch bản"
- Nút **"Tự động dọn"** (icon Settings) — mở modal cấu hình dọn dẹp
- Nút **"Export"** (xanh dương, ChevronDown) — dropdown:
  - Xuất Log (CSV)
  - Xuất Log (JSON)
  - Tải Ảnh (ZIP)

### 10.2 Danh sách log dạng Accordion

Mỗi log là một hàng có thể mở rộng. Khi chưa mở thấy:

- Mũi tên ► (chưa mở) hoặc ▼ (đã mở)
- **Tên kịch bản** (đậm)
- Badge: ✓ Thành công / ⟳ Đang chạy / ✗ Lỗi
- Thông tin dòng dưới: tên máy · thời gian bắt đầu · thời lượng

### 10.3 Chi tiết bước (khi mở rộng)

Click → thấy **timeline dọc từng bước**:

- Icon tròn màu theo trạng thái: Success / Error / Running / Pending
- Tiêu đề: "BƯỚC {index}" + thời gian
- Mô tả hành động (font mono): `tap(0.5, 0.7)`, `wait(5s)`, `launch(com.zhiliaoapp.musically)`…
- Lỗi nếu có: hộp đỏ "Lỗi: [thông báo]"
- Ảnh chụp nếu có: thumbnail 48×80 — click mở lightbox

### 10.4 Lightbox xem ảnh

Click thumbnail → modal full-screen nền đen:
- Ảnh phóng to tối đa
- Metadata bên dưới: tên thiết bị, thời gian, tên kịch bản
- Nút **"Tải xuống"** — lưu ảnh về máy
- Nút **"Xoá ảnh"** — xoá ảnh khỏi bộ nhớ + nhật ký
- Nút X góc phải để đóng

### 10.5 Cửa sổ "Cấu hình dọn dẹp tự động"

Bấm **"Tự động dọn"**:
- Info box xanh: "Hệ thống sẽ tự xoá dữ liệu cũ để giải phóng dung lượng. Hành động này không thể hoàn tác."
- "Nhật ký kịch bản": ô nhập số ngày + đơn vị "ngày" (mặc định 30). Ghi chú: "Xoá nhật ký cũ hơn số ngày chỉ định"
- Nút **"Dọn dẹp ngay"** (đỏ) — chạy cleanup ngay
- Nút **"Huỷ"** và **"Lưu cấu hình"**

### 10.6 Giới hạn lưu trữ (hệ thống tự dọn)

| Loại | Giới hạn | Cơ chế |
|------|----------|--------|
| Nhật ký hoạt động | 1000 bản ghi/tài khoản | Bản cũ nhất bị xoá trước |
| Ảnh chụp màn hình | 500 bản ghi/tài khoản | Bản cũ nhất xoá + xoá ảnh trên đám mây |
| Lịch sử chạy | 500 bản ghi/tài khoản | Bản cũ nhất xoá trước |
| Thông báo | 200 bản ghi | Toàn hệ thống, bản cũ nhất xoá |

Ảnh thực tế lưu trên **bộ nhớ đám mây** — khi bản ghi bị xoá, ảnh cũng bị xoá theo.

---

## Chương 11 — Menu Thư viện Video

Nơi **upload video** để dùng trong kịch bản. Node *"Truyền File"* sẽ truyền video từ đây sang iPhone để đăng TikTok / Facebook / Reels.

### 11.1 Đầu trang

- Tiêu đề **"Thư viện Video"** + phụ đề "Upload và quản lý video, gửi tới thiết bị để đăng"
- Nút **"Upload video"** (xanh dương) — mở hộp thoại chọn file. Khi đang upload → "Đang upload {filename}..." + spinner

### 11.2 Lưới video (responsive)

Hiển thị: 1 cột (mobile) / 2 (md) / 3 (lg) / 4 (xl) cột. Mỗi thẻ:

- **Thumbnail** — frame đầu của video
- **Tên video** (truncate)
- Thông tin: dung lượng (KB/MB) · ngày upload (`DD/MM/YYYY HH:MM`)
- Nút **"Gửi tới thiết bị"** (xanh dương)
- Nút 🗑 — xoá video

### 11.3 Luồng upload video

1. Bấm **"Upload video"** → chọn file từ máy tính.
2. Phải là file video, **tối đa 500MB**.
3. Hệ thống kiểm tra dung lượng còn lại trong gói — nếu vượt sẽ từ chối.
4. Tải lên bộ nhớ đám mây (bạn không cần lo chỗ lưu).
5. Thông báo **"Upload thành công!"** → video xuất hiện trong lưới.

### 11.4 Cửa sổ "Gửi video tới thiết bị"

Bấm **"Gửi tới thiết bị"** → cửa sổ:
- Dòng "Video: **[tên video]**"
- Danh sách "Chọn thiết bị" — chỉ hiện máy online
- Nút **Huỷ** và **Gửi** (mờ nếu chưa chọn máy)

Bấm Gửi → iPhone tự tải video về thư viện ảnh (camera roll). Tối đa **120 giây**.

### 11.5 Xoá video

Bấm 🗑 → "Xoá video? Video sẽ bị xoá và không thể hoàn tác." → Xác nhận → Thông báo "Đã xoá video".

> ℹ️ **Tính năng Video cần admin bật bộ nhớ đám mây.** Nếu thông báo "Chưa cấu hình bộ nhớ" → liên hệ admin.

---

## Chương 12 — Menu Cài đặt cá nhân

Trang cài đặt chia tab dọc bên trái. Tài khoản thường có **2 tab chính**: Thông tin cá nhân & Thông báo.

### 12.1 Tab "Thông tin cá nhân"

#### Thẻ thông tin tài khoản

- Ảnh đại diện (màu nền tự sinh theo tên)
- Tên + email đăng nhập
- Nhãn vai trò: "Người dùng"

#### Khoá API cá nhân (nếu gói có)

Khoá API dùng để kết nối Phone-Agent với phần mềm bên ngoài (bot, tool tự động hoá). Xem chi tiết tích hợp ở [API.md](API.md).

> 🔐 **Khoá chỉ hiện đầy đủ 1 lần khi mới tạo!** Sau khi đóng, chỉ còn hiện 20 ký tự đầu. **Sao chép và lưu vào trình quản lý mật khẩu ngay.**

Thông tin kèm theo: ngày tạo, lần dùng gần nhất, IP sử dụng gần nhất.

**Nút "Đổi khoá mới"** (cam) → cảnh báo:
- "Khoá cũ sẽ ngừng hoạt động ngay lập tức."
- "Mọi phần mềm đang dùng khoá hiện tại cần cập nhật sang khoá mới."

#### Đổi mật khẩu

- Ô **"Mật khẩu hiện tại"**
- Ô **"Mật khẩu mới"** (tối thiểu 6 ký tự)
- Nút **"Cập nhật mật khẩu"** (xanh dương)

### 12.2 Tab "Thông báo"

#### Bật/tắt loại cảnh báo

- 🔕 Công tắc **Thiết bị ngoại tuyến**
- ⚠️ Công tắc **Kịch bản chạy lỗi**

Bật cái nào → khi có sự kiện đó, bạn sẽ nhận cảnh báo.

#### Kết nối Telegram Bot

| Trường | Điền gì |
|--------|---------|
| Bot Token | Mã từ `@BotFather`. Có nút 👁 hiện/ẩn |
| Chat ID | ID group Telegram nơi bạn muốn nhận cảnh báo |

**Nút "Kiểm tra kết nối"** → gửi tin nhắn thử. Nếu nhận được → OK. Thông báo: *"Kết nối Telegram thành công! Kiểm tra tin nhắn trong chat."*

#### Cách lấy Bot Token & Chat ID

1. Mở Telegram → tìm `@BotFather` → gõ `/newbot` → đặt tên → nhận **Bot Token**.
2. Thêm bot vào group + gọi tên bot 1 lần để bot "thấy" group.
3. Lấy Chat ID: truy cập `https://api.telegram.org/bot{TOKEN}/getUpdates`, tìm số `chat.id`.

---

## Chương 13 — 10 luồng làm việc thực tế

Các luồng chuẩn từ onboarding mới đến cảnh báo Telegram. Làm theo đúng trình tự để không kẹt giữa đường.

### 13.1 Nhận tài khoản & đăng nhập lần đầu

1. Đội ngũ Phone-Agent gửi cho bạn **email + mật khẩu tạm**.
2. Vào `phoneagent.io.vn` → bấm "Đăng nhập" → nhập email + mật khẩu.
3. Vào ngay *Cài đặt → Thông tin cá nhân → Đổi mật khẩu*.

### 13.2 Thêm iPhone mới vào hệ thống

1. iPhone phải được đội ngũ cài sẵn và kết nối Tailscale (làm khi bàn giao máy).
2. *Thiết bị → + Thêm thiết bị*: nhập tên, IP, mật khẩu hiển thị màn hình, chọn nhóm.
3. Đợi ~15 giây để hệ thống kiểm tra kết nối.
4. Máy chuyển sang **Online**.
5. Click vào hàng → xem màn hình iPhone live → thử Home, Lướt lên, Chụp ảnh.

### 13.3 Tạo kịch bản TikTok đơn giản

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

### 13.4 Chạy đồng thời 10 máy

1. *Kịch bản → ▶ Chạy*.
2. Tick 10 máy online.
3. Bấm **"Chạy ngay"**.
4. Xem tiến độ trong tab **"Hàng đợi"** của Lịch trình.
5. Xong → xem log từng máy ở *Nhật ký*.

### 13.5 Lên lịch chạy tự động 8h sáng

1. *Lịch trình → + Tạo lịch trình*.
2. Tên: "Sáng nuôi TikTok" · Kịch bản: "Nuôi kênh TikTok sáng".
3. Thiết bị: máy cụ thể (hoặc tạo 10 lịch cho 10 máy).
4. Tần suất **Hàng ngày**, giờ **08:00**.
5. Bấm **"Tạo lịch trình"**. Scheduler tự chạy mỗi 8h sáng.

### 13.6 Chuyển sang IP US trước khi chạy

1. *Proxy → chế độ "Proxy US"*.
2. Paste `ip:port:user:pass`.
3. Bấm **"Tự động phát hiện"** → timezone + GPS tự điền.
4. Tick các máy → bấm **"Áp dụng US & Chạy"**.
5. Đợi Proxy Status chuyển **Running**.
6. Chạy kịch bản TikTok US bình thường.

### 13.7 Cảnh báo Telegram khi máy rớt

1. Lấy Bot Token từ `@BotFather` + Chat ID từ group.
2. *Cài đặt → Thông báo*: bật toggle **"Thiết bị ngoại tuyến"**.
3. Nhập Bot Token + Chat ID → **"Kiểm tra kết nối"**.
4. Nhận tin test → bấm **"Lưu cấu hình"**.
5. Máy mất kết nối > 90s → tin nhắn Telegram cảnh báo.

### 13.8 Upload video và truyền sang iPhone

1. Tính năng Video cần admin bật bộ nhớ đám mây trước (làm 1 lần).
2. *Thư viện Video → Upload* → chọn file .mp4.
3. Trong trình biên tập, kéo khối **Truyền File** → chọn video.
4. Nối tiếp: mở TikTok → tap nút tạo bài → chọn video → đăng.
5. Lưu và chạy.

> 🤖 **Cho AI/Bot bên ngoài đăng video tự động:** Xem [API.md mục 16 — Luồng đăng video tự động](API.md#16--luồng-đăng-video-tự-động). AI có thể đăng ký video bằng URL CDN của họ, không cần upload lên dashboard.

### 13.9 Lấy OTP email tự động khi login MXH

1. Admin đã kết nối Gmail OAuth sẵn (bạn không cần làm).
2. *Tài khoản → cột Gmail → Kết nối* → cấp quyền Google (chỉ đọc email).
3. Nhãn chuyển **✓ Đã kết nối**.
4. Trong kịch bản, sắp theo thứ tự: **Nhập tài khoản → Nhập mật khẩu → Chờ 5s → Lấy OTP Mail**.
5. Khối "Lấy OTP Mail" tự đọc email mới nhất trong 60 giây và điền OTP.

### 13.10 Phục hồi khi kịch bản có lỗi

- Vào *Nhật ký* → tìm dòng "Lỗi" → xem bước nào fail.
- Test điều khiển thủ công trong cửa sổ chi tiết thiết bị → bấm Home → không phản hồi nghĩa là iPhone có vấn đề.
- Nếu tất cả máy offline cùng lúc: kiểm tra WiFi / nguồn điện nơi đặt iPhone.
- Nếu không xử lý được: liên hệ đội ngũ Phone-Agent qua Telegram.

---

## Chương 14 — Mẹo & thủ thuật

### 14.1 Tối ưu kịch bản

- **Dùng "Chờ ngẫu nhiên"** thay vì Chờ cố định ở vị trí nhạy cảm — tránh bị phát hiện bot.
- **Không hardcode toạ độ tuyệt đối** — dùng giá trị 0–1, Phone-Agent tự scale theo màn hình.
- **Chụp ảnh tại checkpoint** (sau login, trước khi đăng bài) — rất giá trị khi debug.
- **Lặp lại ở ngoài cùng** thay vì tạo kịch bản dài vô tận.
- **Nhóm thao tác thành custom node** (VD "Đăng bài", "Comment") để tái sử dụng.

### 14.2 Quản lý farm

- Gom máy theo mục đích: TikTok VN · TikTok US · Facebook.
- Đặt tên máy có quy tắc: `TT-VN-01` thay vì "iPhone của Nam".
- Kiểm tra Tailscale status hàng ngày — máy rớt Tailscale sẽ offline dù WiFi OK.
- Không chạy quá nhiều kịch bản song song trên cùng 1 máy — chậm/giật.

### 14.3 Bảo mật

- **Đổi mật khẩu ngay** sau lần đăng nhập đầu tiên.
- **Khoá API chỉ hiện 1 lần** — lưu vào trình quản lý mật khẩu ngay.
- **Bật cả 2 cảnh báo Telegram** (máy offline + kịch bản lỗi) để biết sớm khi có sự cố.
- Không chia sẻ tài khoản dashboard với người ngoài team.

### 14.4 Hiệu năng

- **Bật proxy từng đợt 10–20 máy** thay vì 100 máy cùng lúc.
- **Dọn nhật ký cũ hàng tuần** — nhật ký lớn làm màn hình tải chậm.
- **Video < 100MB** — file lớn truyền sang iPhone sẽ chậm.
- Không chạy quá nhiều kịch bản song song trên cùng 1 máy.

### 14.5 Xử lý nhanh khi có lỗi

| Triệu chứng | Cách xử lý |
|-------------|------------|
| Máy "Đang kết nối" mãi | Kiểm tra IP và mật khẩu đã đúng chưa, iPhone đã mở app điều khiển chưa |
| Kịch bản chạy 1 bước rồi dừng | Mở Nhật ký → xem bước lỗi → thường do tap sai hoặc app crash |
| Bật proxy nhưng IP vẫn nguyên | Bấm **Áp dụng & Chạy** thay vì chỉ "Kích hoạt" — vì "Kích hoạt" không tự áp dụng giả lập |
| "Đã đạt giới hạn" | Gói đầy máy/kịch bản — liên hệ admin nâng cấp hoặc xoá bớt |

---

## Chương 15 — Câu hỏi thường gặp (FAQ)

**Q: Phone-Agent có bẻ khoá iPhone không?**  
**Không.** Hệ thống cài đặt an toàn, iPhone vẫn giữ bảo hành gốc, vẫn cập nhật iOS bình thường.

**Q: Cần bao nhiêu iPhone để bắt đầu?**  
Tối thiểu **1 iPhone**. Phần lớn user bắt đầu với gói Starter (5 máy) rồi mở rộng.

**Q: iPhone cần WiFi riêng không?**  
Không. Chỉ cần iPhone vào được Internet — phần mạng riêng đã được đội ngũ cài sẵn.

**Q: 50 máy chạy cùng lúc có ảnh hưởng hiệu suất không?**  
Hệ thống được thiết kế để chịu tải. Nếu muốn chạy hơn 100 máy, báo đội ngũ để nâng cấp cấu hình.

**Q: Có giới hạn kịch bản / lịch trình không?**  
Phụ thuộc gói của bạn. Nếu cần thêm, liên hệ admin để nâng cấp.

**Q: Dashboard có theme tối không?**  
Phiên bản hiện tại chỉ có theme sáng. Theme tối đang trong kế hoạch.

**Q: Có thể dùng 2 tài khoản trên cùng dashboard không?**  
Có. Mỗi tài khoản có **không gian riêng**, dữ liệu độc lập — chỉ thấy tài nguyên của mình.

**Q: Có app mobile không?**  
Hiện chỉ có web. Có thể lưu trang vào màn hình chính trên Safari/Chrome mobile — giao diện đã tối ưu cho điện thoại.

**Q: Sao tôi không thấy menu Người dùng / Gói / Máy chủ?**  
Các menu này chỉ hiện với tài khoản admin. Bạn vẫn có đủ menu cần dùng hàng ngày + Cài đặt cá nhân.

**Q: Proxy miễn phí có chạy được không?**  
Về kỹ thuật là có, nhưng proxy miễn phí thường đã bị TikTok/Facebook chặn sẵn. Nên dùng **proxy residential trả phí**.

**Q: Nếu quên mật khẩu thì sao?**  
Liên hệ admin qua Telegram — admin sẽ reset giúp bạn. Sau đó nhớ đổi mật khẩu mới ngay trong *Cài đặt → Thông tin cá nhân*.

**Q: Kịch bản chạy sai vị trí tap thì sao?**  
Màn hình iPhone các dòng khác nhau có độ phân giải khác nhau. Dùng **"Chọn vị trí tap"** trong trình biên tập để pick lại toạ độ cho đúng.

**Q: Có thể cho phần mềm khác (AI, bot) tự động đăng video qua Phone-Agent không?**  
Có. Xem hướng dẫn đầy đủ ở [API.md](API.md) — phần mềm bên ngoài có thể đăng ký video bằng URL CDN, chạy kịch bản với biến động, theo dõi kết quả từ xa.

---

## Chương 16 — Bảng tra nhanh khi gặp sự cố

### 16.1 Không đăng nhập được

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Sai email/mật khẩu" | Nhập sai | Kiểm tra phím CapsLock, sao chép chính xác từ email được cấp |
| Nút Đăng nhập quay mãi | Server không phản hồi | Tải lại trang, thử sau vài phút, liên hệ admin nếu vẫn lỗi |
| Vừa đăng nhập đã bị đá ra | Phiên hết hạn (sau 7 ngày) | Đăng nhập lại bình thường |

### 16.2 Thiết bị offline liên tục

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Offline ngay sau khi thêm | Sai IP | Kiểm tra IP đã được cấp, nhập lại cho đúng |
| Rớt sau vài phút | Pin yếu / iPhone tự ngủ màn hình | Bật "Luôn sáng màn hình" + cắm sạc |
| Nhiều máy offline cùng lúc | Mất WiFi / mất điện | Kiểm tra WiFi + nguồn điện |
| Online nhưng không nhận lệnh | App điều khiển trên iPhone bị treo | Tắt và mở lại app điều khiển |

### 16.3 Kịch bản chạy lỗi

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Lỗi ở bước "tap" | Vị trí tap sai do khác độ phân giải | Dùng "Chọn vị trí tap" pick lại trên màn hình chính xác |
| Lỗi "Device offline" | Máy mất kết nối giữa chừng | Kiểm tra WiFi iPhone, chạy lại |
| Lỗi "Không tìm thấy kịch bản" | Kịch bản đã bị xoá nhưng lịch trình còn trỏ đến | Xoá lịch trình hoặc gán kịch bản khác |
| Lỗi ở bước login | Sai thông tin tài khoản hoặc app đổi giao diện | Kiểm tra lại nick, cập nhật kịch bản theo UI mới |

### 16.4 Proxy không hoạt động

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| Trạng thái proxy "Lỗi" | Thông tin proxy sai | Kiểm tra IP, cổng, user, mật khẩu proxy |
| Giả lập không áp dụng | Chưa tự phát hiện IP | Bấm "Tự động phát hiện" — đảm bảo IP đúng quốc gia |
| Múi giờ vẫn là VN khi dùng proxy US | Chỉ bấm "Kích hoạt", chưa áp dụng giả lập | Bấm **Áp dụng & Chạy** thay vì "Kích hoạt" |

### 16.5 Video không gửi được

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Chưa cấu hình bộ nhớ" | Admin chưa bật bộ nhớ đám mây | Liên hệ admin để bật |
| "Quá thời gian chờ" | Video quá lớn hoặc iPhone chậm | Giảm video xuống < 100MB |
| "Thiết bị ngoại tuyến" | Máy rớt trong lúc gửi | Kiểm tra lại máy trước khi gửi |

### 16.6 Không xem được màn hình iPhone

| Triệu chứng | Nguyên nhân | Cách xử lý |
|-------------|-------------|------------|
| "Đang kết nối..." mãi | Mạng chậm hoặc bị chặn | Tải lại trang, nếu vẫn lỗi liên hệ admin |
| Màn hình đen | App điều khiển trên iPhone chưa bật | Mở app điều khiển trên iPhone, bật dịch vụ |
| Hình méo / giật | Băng thông mạng thấp | Đổi mạng ổn định hơn hoặc báo admin giảm FPS |
| Sai mật khẩu | Mật khẩu hiển thị màn hình không đúng | "Sửa thiết bị" → nhập lại mật khẩu |

### 16.7 Cần hỗ trợ sâu hơn?

Liên hệ đội ngũ Phone-Agent qua **Telegram** (link ở trang chủ → Footer → "Tư vấn qua Telegram"). Khi báo lỗi, vui lòng gửi kèm:

- 📸 Ảnh chụp màn hình lỗi
- 📄 Log chi tiết từ menu *Nhật ký* (bấm Export → JSON)
- 📦 Thông tin gói đang dùng + số máy hiện có
- 📱 Model iPhone và phiên bản iOS

---

## ✅ Checklist sử dụng lần đầu

- [ ] Đổi mật khẩu ngay sau lần đăng nhập đầu tiên
- [ ] Cấu hình Telegram Bot để nhận cảnh báo
- [ ] Thêm iPhone vào hệ thống
- [ ] Tạo nhóm thiết bị theo mục đích
- [ ] Copy kịch bản mẫu từ *Kho kịch bản* hoặc tự tạo
- [ ] Test chạy thủ công trên 1 máy trước
- [ ] Thiết lập lịch trình tự động
- [ ] Theo dõi *Nhật ký* hàng ngày trong tuần đầu

---

**📞 Hỗ trợ:** Telegram của đội ngũ Phone-Agent (link ở Footer trang chủ)  
**🔗 Tài liệu liên quan:**
- [API.md](API.md) — Tích hợp với phần mềm khác (AI, bot tự động đăng video)
- [huong-dan-html/index.html](huong-dan-html/index.html) — Phiên bản HTML có hình ảnh minh hoạ
- [huong-dan-setup-gmail-oauth.md](huong-dan-setup-gmail-oauth.md) — Cấu hình Gmail OAuth (cho admin)

© 2026 Phone Agent · Cẩm nang sử dụng v2.0 · phoneagent.io.vn
