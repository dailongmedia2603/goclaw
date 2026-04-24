# Test nhanh Facebook History Backfill — Hướng dẫn tiếng Việt

Dành cho chủ fork sau khi deploy thành công lên VPS (`agent.thekp3.com`).

## 1. Xác nhận feature đã có trên production

Mở https://agent.thekp3.com/overview và đăng nhập như bình thường.

Trong console trình duyệt (F12 → Console), gõ:
```js
location.reload()
```
để hard refresh, tránh cache UI cũ.

## 2. Kiểm tra qua log VPS

SSH vào VPS và kiểm tra dòng khởi động fbbackfill:

```bash
ssh -p 2018 root@61.14.233.24
docker logs goclaw-goclaw-1 2>&1 | grep fb_backfill | head -5
```

**Kỳ vọng**: dòng `INFO fb_backfill.register.ok rpc=true events=true llm=true`. Nếu không có → feature chưa active (check container đã restart chưa).

## 3. Tạo channel Facebook để test

### Bước 3a: Vào Channels → Add Channel

Trong UI, chọn loại **Facebook** (không phải "Facebook Personal" / Zalo / Telegram...).

### Bước 3b: Điền thông tin

| Field | Giá trị |
|-------|---------|
| Key | `fb-test-backfill` (hoặc tên bất kỳ) |
| Display Name | `FB Test` |
| Agent | Chọn 1 agent bất kỳ |
| Page Access Token | Token từ Facebook Developer Console |
| App Secret | App Secret từ Facebook App → Settings |
| Webhook Verify Token | Chuỗi bất kỳ do bạn đặt |
| Page ID | ID số của Page (lấy từ Meta Business Suite) |

### Bước 3c: Trong phần Config (cuối form)

Scroll xuống dưới cùng của phần Configuration, bạn sẽ thấy checkbox mới:

> ☐ **Scan conversation history after creating**
>
> *When enabled, the system automatically scans all past Messenger conversations from this Page after connecting...*

**Tích checkbox này** → bấm **Create**.

## 4. Xem backfill chạy

Sau khi tạo xong:

1. Vào **Channels** → click vào channel `fb-test-backfill` vừa tạo.
2. Scroll xuống phía dưới các tab (General / Credentials / Managers). Bạn sẽ thấy panel mới:

> **Conversation History Backfill** (Running)
>
> ━━━━━━━━━━━━━━━━ 15%
>
> 3/20 conversations · 47 messages · 2 summaries
>
> [⏸ Pause] [✕ Cancel]

3. Progress cập nhật real-time. Khi xong:

> **Conversation History Backfill** (Completed)
>
> ━━━━━━━━━━━━━━━━ 100%
>
> 20/20 conversations · 312 messages · 20 summaries
>
> [↻ Re-sync]

## 5. Chứng minh agent đã có ngữ cảnh

Sau khi backfill xong, nhắn từ tài khoản Facebook cá nhân đến Page. Agent sẽ trả lời **có ngữ cảnh** cuộc trò chuyện cũ — thay vì hỏi "Bạn cần hỗ trợ gì?" như kênh mới tích hợp.

Kiểm tra qua DB:
```sql
SELECT source_id, l0_abstract, turn_count
FROM episodic_summaries
WHERE source_type = 'fb_backfill'
LIMIT 5;
```
Mỗi hàng = 1 conversation cũ đã tóm tắt. `source_id` có dạng `fb_backfill:{page_id}:{psid}`.

## 6. Khi nào backfill fail?

| Trường hợp | Thấy gì | Làm sao |
|------------|--------|---------|
| Token hết hạn | Status = `Failed`, error = "Page Access Token expired..." | Vào tab Credentials → update Page Access Token mới → quay lại panel click **Retry** |
| Vượt rate limit | Status = `Paused`, error = "Graph API rate limit reached..." | Để yên — tự tiếp tục trong 15-60 phút |
| Page không có tin nhắn | Status = `Completed`, 0/0 conversations | Đúng rồi, không có gì để quét |
| App chưa qua App Review | Status = `Failed`, "permission denied" | Cần submit app vào Meta App Review + Business Verification |

## 7. Pause / Resume thủ công

Khi backfill đang chạy mà muốn tạm dừng (ví dụ để tiết kiệm quota):

1. Click **Pause** → trạng thái chuyển sang `Paused`, cursor được lưu.
2. Khi muốn tiếp tục → click **Resume** → backfill tiếp tục từ cursor đã lưu, không duplicate.

## 8. Re-sync toàn bộ (force recreate)

Sau khi `Completed`, nếu bạn muốn tóm tắt lại tất cả (ví dụ sau khi cấu hình LLM tốt hơn):

1. Click **Re-sync** → confirm trên dialog.
2. Backfill chạy lại, lần này ghi đè (delete + create) tất cả tóm tắt cũ.

## 9. Những gì KHÔNG xảy ra khi backfill

Để bạn yên tâm:
- ❌ **Không** gửi tin nhắn cho khách (không spam)
- ❌ **Không** ghi tin nhắn cũ vào session đang diễn ra (không phá conversation hiện tại)
- ❌ **Không** expose hot data (không lấy hình/file về — chỉ note có bao nhiêu attachment)
- ❌ **Không** ảnh hưởng tenant khác (tenant-scoped triệt để)
- ❌ **Không** mất feature khi cập nhật GoClaw upstream (fork-safety check trong CI)

## 10. Clean-up test channel

Sau khi test xong muốn xoá:

1. Channels → channel fb-test → Header có nút **Delete**.
2. Trong SQL, nếu muốn xoá luôn episodic entries đã tạo (tuỳ chọn):
```sql
DELETE FROM episodic_summaries
WHERE source_type = 'fb_backfill'
  AND agent_id = '<agent_uuid>';
```

---

Bất kỳ lỗi lạ nào → gõ log `docker logs goclaw-goclaw-1 2>&1 | grep fb_backfill.*error | tail -20` và gửi cho dev.
