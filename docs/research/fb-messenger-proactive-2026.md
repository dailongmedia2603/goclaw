# Research Report: Facebook Messenger Proactive Messaging — Cách Pancake/Smax/Mio Thực Sự Hoạt Động

**Ngày nghiên cứu:** 2026-04-26
**Phạm vi:** Cơ chế kỹ thuật + chính sách FB cho phép gửi tin nhắn proactive từ Page tới khách. Bối cảnh: GoClaw FBM channel cần gửi tin chủ động tới PSID đã backfill.

---

## Executive Summary

**Phát hiện ngắn:** Pancake/Smax/Mio/Harasocial **KHÔNG** reverse-engineer hay bypass FB. Họ dùng đúng các message tag chính thống của Graph Send API — đặc biệt là **HUMAN_AGENT tag** (window 7 ngày), kết hợp Private Reply API và (cho đến gần đây) Recurring Notifications.

**Critical Timeline 2026 — phải biết ngay:**

| Mốc | Sự kiện | Ảnh hưởng |
|---|---|---|
| 12 Jan 2026 | Recurring Notifications API **deprecated** | Tool nào còn dùng RN sẽ break |
| 10 Feb 2026 | RN tắt globally trừ AU/EU/JP/KR/UK | VN bị tắt — Pancake/Smax đã phải migrate |
| **27 Apr 2026** (mai!) | 3 tags cũ trả error code 100: `CONFIRMED_EVENT_UPDATE`, `ACCOUNT_UPDATE`, `POST_PURCHASE_UPDATE` | Code dùng những tag này sẽ chết |

Bối cảnh dồn dập. FB đang dồn ép developer chuyển sang **Marketing Messages API** (mới, beta, region-limited) hoặc **Utility Messages**.

**Khuyến nghị cho GoClaw:** Build path Send với `HUMAN_AGENT` tag — không vướng deadline, ổn định, là tag duy nhất KHÔNG nằm trong danh sách deprecated. Đây cũng đúng là cách Pancake gắn nhãn "Tin nhắn CSKH" trong UI.

---

## 1. Pancake/Smax thực sự dùng gì?

### 1.1 Phân loại message của Pancake (xác nhận từ docs)

Pancake chia broadcast làm 2 loại:

| Tên trong UI Pancake | Cơ chế FB | Window | Hạn chế nội dung |
|---|---|---|---|
| **Tin nhắn tiêu chuẩn** | Standard messaging (`messaging_type=RESPONSE`) | 24h | Free-form (kể cả promotional) |
| **Tin nhắn CSKH** | `messaging_type=MESSAGE_TAG`, `tag=HUMAN_AGENT` | 7 ngày | **KHÔNG** chứa từ promotional (Pancake auto-filter từ khoá: Khuyến mãi, Sale, Giảm giá, Deal, Coupon, Voucher) |

Source xác nhận từ docs Pancake + docs.botcake.io:
- *"Tin nhắn tiêu chuẩn: gửi cho những khách hàng nhắn tin với page trong vòng 24h, được phép gửi nội dung chứa quảng cáo"*
- *"Tin nhắn CSKH: gửi cho cả những khách hàng tương tác trong và ngoài 24h, KHÔNG được gửi nội dung chứa quảng cáo"*

→ **"Tin nhắn CSKH" chính là HUMAN_AGENT tag.** Pancake bắt buộc filter promotional keywords vì đó là điều kiện FB enforce.

### 1.2 Yêu cầu kỹ thuật để dùng HUMAN_AGENT

Theo doc Meta:
- App phải pass **App Review** với permission `human_agent_messaging`
- Page Admin phải confirm "có agent thật trả lời"
- Use case hợp lệ: "weekend closures", "issue cần >24h xử lý", customer support

→ Pancake/Smax đã có Page Access Token đã được approve permission này → tool của họ hoạt động được.

### 1.3 Các kỹ thuật phụ trợ Pancake/Smax dùng

1. **Comment-to-Inbox + Private Reply API**
   - `POST /comments/{comment-id}/private_replies` — chuyển khách comment thành DM
   - Mở 24h window mới hợp pháp
   - Phổ biến trong "auto inbox seeding"

2. **Token rotation**
   - Pancake doc đề cập "đảm bảo không có token nào bị lỗi, bật token Admin đang hoạt động"
   - Họ store nhiều admin token + rotate khi 1 token bị FB throttle

3. **SPIN content + variable substitution**
   - `{full_name}`, `{gender}` trong template
   - Tránh content giống hệt → giảm risk bị FB flag spam

4. **Click-to-Messenger Ads**
   - Chạy ads CTA Messenger → user click → reset 24h window
   - Tốn tiền ads nhưng không vi phạm

---

## 2. State hiện tại của FB Messenger Platform (April 2026)

### 2.1 Còn dùng được

| Cơ chế | API | Window | Yêu cầu |
|---|---|---|---|
| Standard messaging | `messaging_type=RESPONSE` | 24h | `pages_messaging` (App Review) |
| **HUMAN_AGENT tag** | `messaging_type=MESSAGE_TAG`, `tag=HUMAN_AGENT` | **7 ngày** | `human_agent_messaging` (App Review riêng) |
| Private Reply (comments) | `/comments/{id}/private_replies` | Mở conversation mới | Page Access Token |
| Sponsored Message | Ads Manager API | Bất kỳ | Chi phí ads |
| Marketing Messages API (new) | `/me/messages` với template | Bất kỳ (template-based) | App Review + region eligibility (KHÔNG khả dụng EU; VN status không rõ, đang beta rollout) |
| Utility Messages (new) | `/me/messages` với utility template | Bất kỳ | App Review + non-promotional content |

### 2.2 Đã / sắp chết

| Cơ chế | Trạng thái | Migration path |
|---|---|---|
| `CONFIRMED_EVENT_UPDATE` tag | Chết 27 Apr 2026 (mai) | → Utility Messages |
| `ACCOUNT_UPDATE` tag | Chết 27 Apr 2026 | → Utility Messages |
| `POST_PURCHASE_UPDATE` tag | Chết 27 Apr 2026 | → Utility Messages |
| Recurring Notifications (RN) | Chết 10 Feb 2026 globally (trừ AU/EU/JP/KR/UK) | → Marketing Messages API |
| One-Time Notification (OTN) | Đã chết từ 2024 (subsumed bởi RN, sau đó RN cũng chết) | → Marketing Messages API |

### 2.3 Cảnh báo deadline cận kề

**27 Apr 2026** chỉ còn 1 ngày:
- Code nào còn gửi với `tag=CONFIRMED_EVENT_UPDATE/ACCOUNT_UPDATE/POST_PURCHASE_UPDATE` → API trả error code 100
- Hệ thống automation kiểu Manychat/Chatfuel/Pancake nếu vẫn còn flow cũ sẽ bị break ngay
- → GoClaw nếu build không nên dùng các tag này, đi thẳng vào HUMAN_AGENT là an toàn dài hạn nhất

---

## 3. Implementation cho GoClaw — Đề xuất kỹ thuật

### 3.1 Path A — HUMAN_AGENT (đề xuất ưu tiên)

**Tại sao chọn:** ổn định nhất, không trong danh sách deprecation, đúng pattern Pancake dùng.

**Cần làm:**

1. **App Review** — xin permission `human_agent_messaging` cho App `DrX x Dailong` (App ID 1186390573343179):
   - Vào *Xét duyệt ứng dụng → Quyền và tính năng* → tìm `Human Agent Messaging` → Request
   - Use case mô tả: "AI agent supports customer inquiries on behalf of human agents during off-hours"
   - Screencast demo flow: agent reply một câu hỏi đặt 2-3 ngày trước

2. **Code change trong GoClaw** — thêm Send path direct qua Graph API (song song sidecar):

```go
// internal/channels/facebookmessenger/graph_send.go (mới)

type GraphSendOpts struct {
    PSID         string
    Message      string
    MessagingType string // "RESPONSE" | "MESSAGE_TAG"
    Tag          string // "HUMAN_AGENT" khi out-of-window
}

func (c *Channel) SendViaGraph(ctx context.Context, opts GraphSendOpts) error {
    body := map[string]any{
        "recipient":      map[string]string{"id": opts.PSID},
        "messaging_type": opts.MessagingType,
        "message":        map[string]string{"text": opts.Message},
    }
    if opts.MessagingType == "MESSAGE_TAG" {
        body["tag"] = opts.Tag
    }
    // POST graph.facebook.com/v25.0/me/messages?access_token=<page_token>
    // ...
}
```

3. **RPC method mới:** `fb.send_proactive` trong `internal/gateway/methods/`
4. **Logic chọn tag:** lookup `last_inbound_at` của PSID từ episodic_summaries
   - `<24h` → `messaging_type=RESPONSE` (free)
   - `24h–7d` → `MESSAGE_TAG` + `HUMAN_AGENT` + content filter
   - `>7d` → reject với error rõ ràng "ngoài cửa sổ HUMAN_AGENT"
5. **Content filter:** bắt chước Pancake — block keywords: "Khuyến mãi", "Sale", "Giảm giá", "Deal", "Coupon", "Voucher", "Off", "Free" trước khi gửi với HUMAN_AGENT tag.

### 3.2 Path B — Comment-to-Inbox (kéo lead mới)

**Use case:** mở Conversation mới với người chưa từng nhắn Page.

**Cần:** webhook subscribe field `feed` (post comments) → khi có comment mới + match keyword config → tự gọi `POST /comments/{comment_id}/private_replies` → mở DM → bot tự reply.

**Code complexity:** medium. Thêm field vào webhook handler + RPC method `fb.private_reply_to_comment`.

### 3.3 Path C — Marketing Messages API (làm xa hơn nếu cần)

Mới, beta, không khả dụng tại nhiều region (chưa rõ VN). **Skip ngay nếu chỉ làm cho thị trường VN** — chờ FB rollout chính thức.

### 3.4 KHÔNG nên làm

- ❌ Browser automation business.facebook.com Inbox — tốn RAM, dễ bị FB anti-bot, user đã reject
- ❌ Reverse-engineered Messenger MQTT (fbchat-style) — tài khoản admin bị ban, maintenance khổng lồ khi FB break protocol
- ❌ Dùng các tag deprecated trước 27 Apr — sẽ chết trong vài ngày

---

## 4. So sánh các tool VN với GoClaw sau khi thêm HUMAN_AGENT path

| Capability | Pancake | Smax | GoClaw (sau khi build) |
|---|---|---|---|
| Auto-reply 24h | ✅ | ✅ | ✅ (đã có) |
| HUMAN_AGENT 7 ngày | ✅ "CSKH" | ✅ | ✅ (build mới) |
| Comment-to-Inbox | ✅ | ✅ | ❌ → Phase 2 |
| Click-to-Messenger ads | Manual | Manual | Manual (FB Ads Manager) |
| Backfill lịch sử + AI context | ❌ | ❌ | ✅ (lợi thế GoClaw) |
| Marketing Messages API | Đang migrate | Đang migrate | Defer |
| AI agent thật sự (LLM-powered) | Hạn chế | Hạn chế | ✅ (lợi thế GoClaw) |

→ GoClaw + HUMAN_AGENT = parity functional với Pancake/Smax cho phần proactive, **vượt trội** ở backfill + AI agent.

---

## 5. Risk Assessment

| Risk | Pancake/Smax cách | Browser automation | Reverse-engineered |
|---|---|---|---|
| Page bị restrict | Thấp (tuân thủ tag rules) | Trung bình–Cao | Cao |
| Account admin bị lock | Không | Trung bình (anti-bot detect) | Cao |
| API break giữa chừng | Thấp (official API) | Cao (UI change) | Rất cao (protocol change) |
| Maintenance burden | Thấp | Cao | Rất cao |
| App Review reject | Có thể (nếu use case yếu) | N/A | N/A |

---

## 6. Action Items cho GoClaw

### Ngay lập tức (1–2 ngày)
1. ☐ Submit App Review xin `human_agent_messaging` cho App 1186390573343179
   - Chuẩn bị screencast 2–3 phút demo flow agent reply customer support out-of-24h
   - Use case template: customer inquiry handled by AI agent for after-hours support
2. ☐ Audit code GoClaw chắc chắn không có chỗ nào dùng tag `CONFIRMED_EVENT_UPDATE`, `ACCOUNT_UPDATE`, `POST_PURCHASE_UPDATE` (deadline 27 Apr 2026)

### Trong khi chờ App Review (3–7 ngày)
3. ☐ Build `Channel.SendViaGraph` direct Graph API path (song song sidecar mautrix-meta)
4. ☐ Thêm RPC `fb.send_proactive` với param: `psid`, `message`, `mode` (auto/standard/human_agent)
5. ☐ Build content filter cho promotional keywords (block list theo Pancake pattern)
6. ☐ Track `last_inbound_at` per PSID — extend backfill state hoặc dùng webhook event ghi vào table mới

### Sau khi App Review approved
7. ☐ E2E test: gửi tin tới PSID đã nhắn 3 ngày trước với tag=HUMAN_AGENT
8. ☐ UI dashboard: list PSID đã backfill + filter "có thể gửi tin (≤7 ngày)" + nút "Gửi thông báo"
9. ☐ Documentation cho user về window rules (24h vs 7d) + content restrictions

### Phase 2 (sau khi proactive cơ bản chạy ổn)
10. ☐ Implement Comment-to-Inbox flow (Private Reply API)
11. ☐ Theo dõi rollout Marketing Messages API tại VN; nếu khả dụng, add as Path C

---

## Resources & References

### Official Documentation
- [Messenger Platform Changelog (Meta)](https://developers.facebook.com/docs/messenger-platform/changelog/)
- [Human Agent Tag — Graph API Reference (Meta)](https://developers.facebook.com/docs/features-reference/human-agent)
- [Send API Reference (Meta)](https://developers.facebook.com/docs/messenger-platform/reference/send-api/)
- [Marketing Messages on Messenger API (Meta)](https://developers.facebook.com/docs/marketing-messages-on-messenger/)
- [Marketing Messages API FAQ (Meta)](https://developers.facebook.com/docs/marketing-messages-on-messenger/faq/)
- [RN Deprecation Notice (Meta Business Help)](https://www.facebook.com/business/help/1321849029608125)

### Community/Tool Documentation
- [Pancake Botcake — Gửi tin nhắn](https://docs.pancake.biz/botcake/st-f4/st-p2?lang=vi)
- [Botcake Send Message Docs](https://docs.botcake.io/overview/gui-tin-nhan)
- [Manychat — Send messages outside 24h/7d windows](https://help.manychat.com/hc/en-us/articles/14281199732892)
- [Manychat — Marketing Messages on Messenger guide](https://help.manychat.com/hc/en-us/articles/24351480518684)
- [Chatimize — Comply with Messenger Rules in 2026](https://chatimize.com/facebook-messenger-policy/)
- [Genesys — HUMAN_AGENT tag deprecation announcement](https://help.genesys.cloud/announcements/facebook-messenger-human-agent-message-tag-deprecation/) (note: title misleading — only old tags removed, HUMAN_AGENT stays)
- [Sprinklr — FB Platform Policy for DM replies](https://www.sprinklr.com/help/articles/reply-from-engagement-dashboards/facebook-platform-policy-for-replying-to-direct-messages/63eb9947ef1b447d6c61e4b8)
- [Social Media Today — Meta Recurring Marketing Messages API End](https://www.socialmediatoday.com/news/metas-recurring-marketing-messages-api-will-end-this-week/811668/)
- [Releasebot — Graph API Jan 2026 Release Notes](https://releasebot.io/updates/meta/graph-api)

### Vietnamese Tool Vendors
- [Harasocial — Facebook AI Sales Tool](https://www.haravan.com/pages/phan-mem-ban-hang-facebook-harasocial)
- [Pages.fm (Pancake)](https://pages.fm/contact)
- [Fchat — Broadcast Campaigns](https://fchat.vn/help/broadcast)

---

## Unresolved Questions

1. **Marketing Messages API có khả dụng tại VN chưa?** Doc nói "limited regions, beta rollout". Cần kiểm tra trên dashboard Meta Business của user (App ID 1186390573343179) → có thấy mục Marketing Messages permission request không.
2. **HUMAN_AGENT tag bị siết chính sách không trong tương lai?** Genesys có announcement title gây hiểu lầm là "deprecation" nhưng nội dung thực ra chỉ liên quan các tag khác. Cần monitor changelog Meta định kỳ.
3. **Sidecar mautrix-meta có thể gửi với tag không?** Cần kiểm tra source mautrix-meta — nếu không thì bắt buộc đi direct Graph API path (đề xuất 3.1).
4. **Content filter promotional keywords có sufficient không?** Pancake dùng blacklist; FB có thể dùng ML classifier mạnh hơn — không có guarantee tin "sạch keyword" sẽ không bị reject.
