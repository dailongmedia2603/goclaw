# Giới thiệu Phone-Agent — Phone-Agent là gì, bạn làm được gì

## Phone-Agent là gì

Phone-Agent là **dashboard web** giúp bạn ngồi một chỗ, mở trình duyệt và điều khiển hàng chục — hàng trăm iPhone thật cùng lúc. Thay vì cầm từng máy bấm tay, bạn vẽ "công thức" thao tác bằng cách kéo-thả các khối, rồi bấm chạy — máy tự làm.

Ví dụ: vẽ kịch bản "Mở TikTok → lướt 20 lần → like 5 video", bấm chạy → 10 iPhone cùng lúc tự làm, mỗi máy dùng một nick khác nhau.

**Quan trọng:** Phone-Agent **không bẻ khoá** iPhone. Máy vẫn còn bảo hành, vẫn cập nhật iOS bình thường.

## Đối tượng sử dụng

- Affiliate marketer
- Người vận hành farm iPhone
- Đội nuôi kênh TikTok / Facebook / Instagram

## Bạn làm được những gì

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

## Hệ thống chạy ra sao

```
[Trình duyệt của bạn] ──► [Phone-Agent (cloud)] ──► [iPhone của bạn (mạng riêng Tailscale)]
```

- Bạn ngồi máy tính, mở trang `phoneagent.io.vn`.
- Phone-Agent kết nối tới iPhone qua **mạng riêng Tailscale** — bảo mật, không lộ IP thật.
- Ảnh chụp, video lưu trên **bộ nhớ đám mây** — không làm đầy iPhone.

## Thuật ngữ cần biết trước

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
| **Tín hiệu kết nối** | iPhone tự báo "còn sống" về hệ thống — mất > 90 giây = offline |
| **Không gian riêng** | Mỗi tài khoản có dữ liệu độc lập, chỉ thấy đồ của mình |
| **Gói sử dụng** | Gói quy định số máy tối đa, số kịch bản, có proxy hay không… |

## Phiên bản & liên kết

- **Phiên bản cẩm nang:** 2.0 (cập nhật 2026-04-20)
- **Tên miền:** `phoneagent.io.vn`
- **Bản HTML có hình ảnh minh hoạ:** `huong-dan-html/index.html`
- **Tích hợp API cho phần mềm khác:** xem `API.md`
