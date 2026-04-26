# FBCloak — ToS Disclaimer (Source of Truth)

**Version:** v1.0
**Effective:** 2026-04-26
**Bumping policy:** Any wording change → bump `CurrentDisclaimerVersion` in `internal/channels/fbcloak/disclaimer.go`. The DB constraint forces every tenant to re-acknowledge the new version before re-enabling jobs.

---

## Tiếng Việt (nguồn gốc — bản chuẩn)

Bằng việc kích hoạt tính năng FBCloak Re-engagement, người vận hành xác nhận hiểu và chấp nhận các điều sau:

1. **Tôi hiểu tính năng này có thể vi phạm Điều khoản dịch vụ của Meta.**
   FBCloak vận hành thông qua trình duyệt tự động chứ không phải Graph API chính thức của Meta. Theo Điều khoản dịch vụ Meta cập nhật ngày 01/01/2025, việc truy cập tự động không có sự cho phép trước có thể bị xem là hành vi vi phạm.

2. **Tôi hiểu page có thể bị hạn chế và tài khoản admin có thể bị khóa.**
   Khi Meta phát hiện hành vi tự động hoá, page có thể bị giảm reach, bị tạm khoá messaging, hoặc bị xoá. Tài khoản cá nhân quản trị page có thể bị giới hạn hoặc bị khoá vĩnh viễn. Đây là rủi ro đã biết, không phải lỗi kỹ thuật.

3. **Tôi sử dụng dưới trách nhiệm cá nhân và đã có sự đồng ý của người nhận.**
   Người vận hành chịu trách nhiệm hoàn toàn về nội dung gửi đi, đảm bảo chỉ liên hệ những người đã từng tương tác với page (theo cửa sổ 7 ngày–6 tháng), và đảm bảo nội dung phù hợp với pháp luật bảo vệ người tiêu dùng tại Việt Nam.

GoClaw cung cấp tính năng này để hỗ trợ vận hành; mọi hệ quả pháp lý, tài khoản hoặc danh tiếng phát sinh đều do người vận hành tự gánh chịu.

---

## English (translation)

By enabling the FBCloak Re-engagement feature, the operator confirms and accepts:

1. **I understand this feature may violate Meta's Terms of Service.**
   FBCloak operates via browser automation, not Meta's official Graph API. Per Meta's ToS effective 2025-01-01, automated access without prior permission may be deemed a violation.

2. **I understand my page may be restricted and admin accounts may be locked.**
   When Meta detects automation, the page may suffer reach reduction, temporary messaging block, or deletion. Personal admin accounts may be restricted or permanently banned. This is a known risk, not a software defect.

3. **I use this feature under my own responsibility and have the recipient's consent.**
   The operator is fully responsible for outbound content, ensures recipients are limited to those who previously interacted with the page (7d–6m window), and complies with applicable consumer-protection laws.

GoClaw provides this feature as a tool. All legal, account, or reputation consequences are the operator's responsibility.

---

## 中文(译文)

启用 FBCloak 重新互动功能即表示运营者确认并接受以下内容:

1. **我理解此功能可能违反 Meta 服务条款。**
   FBCloak 通过浏览器自动化运行,而非 Meta 的官方 Graph API。根据 2025-01-01 生效的 Meta 服务条款,未经授权的自动化访问可能被视为违规。

2. **我理解主页可能被限制,管理员账号可能被锁定。**
   当 Meta 检测到自动化行为时,主页可能被降权、暂时禁止发消息或被删除。个人管理员账号可能被限制或永久封禁。这是已知风险,而非软件缺陷。

3. **我自愿承担使用责任,并已获得收件人同意。**
   运营者对发送内容负全责,确保仅联系曾与主页互动过的用户(7 天–6 个月窗口),并遵守当地消费者保护法律。

GoClaw 提供此功能作为工具。所有法律、账户或声誉后果均由运营者承担。

---

## Translation policy

If translations diverge from the Vietnamese source, the **Vietnamese version controls**. Translators must keep three statements 1:1 with the source. Any operator querying the disclaimer via `fbcloak.disclaimer.status` receives the version string only — UI looks up the corresponding locale text by `version` + current `i18n` namespace.

## Changelog

- **v1.0** (2026-04-26): Initial release. Phase 4 launch.
