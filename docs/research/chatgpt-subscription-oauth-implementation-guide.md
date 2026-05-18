# Hướng dẫn triển khai tính năng "ChatGPT Subscription (OAuth)" — Đầy đủ & Chi tiết

> **Mục đích:** Tài liệu này mô tả toàn diện cách triển khai tính năng cho phép phần mềm sử dụng **tài khoản ChatGPT Plus/Pro/Business/Edu** của người dùng (đã trả tiền subscription) để gọi LLM **mà không cần API key**. Cơ chế dựa trên việc mô phỏng **Codex CLI** chính thức của OpenAI: đăng nhập user qua OAuth 2.0 PKCE, sau đó gọi endpoint nội bộ `chatgpt.com/backend-api/codex/responses`.
>
> Tài liệu được viết để bạn có thể **port nguyên vẹn** tính năng này sang phần mềm khác (Node.js, Python, Rust, v.v.). Code mẫu là Go vì đây là codebase gốc, nhưng pattern và protocol có thể áp dụng cho mọi ngôn ngữ.

---

## Mục lục

1. [Tổng quan & Khái niệm cốt lõi](#1-tổng-quan--khái-niệm-cốt-lõi)
2. [Kiến trúc tổng thể](#2-kiến-trúc-tổng-thể)
3. [OAuth 2.0 PKCE Flow — Chi tiết từng bước](#3-oauth-20-pkce-flow--chi-tiết-từng-bước)
4. [Token Management & Lifecycle](#4-token-management--lifecycle)
5. [Trích xuất Metadata từ JWT](#5-trích-xuất-metadata-từ-jwt)
6. [Gọi LLM qua Responses API](#6-gọi-llm-qua-responses-api)
7. [Quota API — Kiểm tra hạn mức](#7-quota-api--kiểm-tra-hạn-mức)
8. [Database Schema](#8-database-schema)
9. [HTTP API Design (REST cho UI)](#9-http-api-design-rest-cho-ui)
10. [UI/UX Flow](#10-uiux-flow)
11. [Multi-Account Pool (Nâng cao)](#11-multi-account-pool-nâng-cao)
12. [Production Hardening — Edge Cases & Errors](#12-production-hardening--edge-cases--errors)
13. [Reference Code — Standalone Go](#13-reference-code--standalone-go)
14. [Port sang Node.js / Python](#14-port-sang-nodejs--python)
15. [Testing & Debugging](#15-testing--debugging)
16. [Pháp lý & Chính sách](#16-pháp-lý--chính-sách)
17. [Roadmap mở rộng](#17-roadmap-mở-rộng)

---

## 1. Tổng quan & Khái niệm cốt lõi

### 1.1 Tính năng làm gì?

Cho phép người dùng đăng nhập tài khoản ChatGPT trả phí (Plus $20/tháng, Pro, Business, Edu) vào phần mềm của bạn, sau đó phần mềm có thể:

- Gọi GPT-5.x, GPT-4.x, o-series qua API
- Dùng tool calling (function calling)
- Stream response qua SSE
- Sinh ảnh native (gpt-image-2)
- Reasoning/thinking mode

**Billing trừ vào hạn mức subscription** (5-hour window + weekly limit), **không tốn credit API**.

### 1.2 Hoạt động dựa trên?

Phần mềm mã nguồn mở **Codex CLI** của OpenAI (`https://github.com/openai/codex`) là một CLI cho phép developer dùng tài khoản ChatGPT để code. OAuth client_id của nó là `app_EMoamEEZ73f0CkXaXp7hrann` — public và stable. Chúng ta **bắt chước y hệt protocol** của Codex CLI để OpenAI server xem mình là "Codex CLI".

### 1.3 Hai endpoint quan trọng nhất

| Endpoint | Mục đích |
|---|---|
| `https://auth.openai.com/oauth/authorize` | Authorize URL (OAuth start) |
| `https://auth.openai.com/oauth/token` | Token exchange + refresh |
| `https://chatgpt.com/backend-api/codex/responses` | LLM streaming call (Responses API) |
| `https://chatgpt.com/backend-api/wham/usage` | Quota check (rate limit info) |

Lưu ý: endpoint LLM **KHÔNG phải `api.openai.com`** mà là `chatgpt.com/backend-api`. Đây là API nội bộ của ChatGPT web app — Codex CLI re-use.

### 1.4 3 hằng số "ma thuật" phải copy chính xác

```
client_id     = "app_EMoamEEZ73f0CkXaXp7hrann"
redirect_uri  = "http://localhost:1455/auth/callback"   // Port 1455 CỐ ĐỊNH
scopes        = "openid profile email offline_access api.connectors.read api.connectors.invoke"
```

Bất kỳ thay đổi nào → OpenAI auth server từ chối.

---

## 2. Kiến trúc tổng thể

```
┌─────────────────────────────────────────────────────────────────────┐
│                            User's Browser                            │
└──────────┬────────────────────────────────────────┬─────────────────┘
           │ 1. Click "Sign in"                     │ 4. Redirect with ?code=
           ▼                                        ▼
┌──────────────────────────────┐         ┌───────────────────────────┐
│   Your App's Web UI (SPA)    │         │  auth.openai.com (OAuth)  │
└──────────┬───────────────────┘         └─────────▲─────────────────┘
           │ 2. POST /v1/auth/.../start             │ 3. /authorize?...
           ▼                                        │
┌──────────────────────────────────────────────────┴──────────────────┐
│                       Your App's Backend (Go)                        │
│  ┌────────────────────┐  ┌─────────────────┐  ┌──────────────────┐ │
│  │  HTTP REST handler │  │ Local callback  │  │ Token Refresh    │ │
│  │  /v1/auth/...      │  │ server :1455    │  │ background       │ │
│  └─────────┬──────────┘  └────────┬────────┘  └────────┬─────────┘ │
│            │                       │                    │           │
│            └───────────┬───────────┴────────────────────┘           │
│                        │                                            │
│         ┌──────────────▼───────────────┐                            │
│         │     DBTokenSource (cache)    │                            │
│         │  - Token() with auto-refresh │                            │
│         └──────────────┬───────────────┘                            │
│                        │                                            │
│         ┌──────────────▼───────────────┐                            │
│         │  CodexProvider               │                            │
│         │  - Chat / ChatStream         │                            │
│         │  - POST /codex/responses     │                            │
│         └──────────────┬───────────────┘                            │
└────────────────────────┼────────────────────────────────────────────┘
                         │
                         ▼
        ┌────────────────────────────────────┐
        │   chatgpt.com/backend-api          │
        │   /codex/responses (SSE)           │
        │   /wham/usage (quota)              │
        └────────────────────────────────────┘
```

### 2.1 Các thành phần phải có

| Component | Bắt buộc? | Mô tả |
|---|---|---|
| **PKCE generator** | ✅ | Sinh verifier/challenge S256 |
| **Local callback server** | ✅ | Bắt redirect tại `localhost:1455` |
| **Token exchanger** | ✅ | Đổi code → tokens |
| **Token refresher** | ✅ | Tự động refresh trước expiry |
| **Token store** | ✅ | DB hoặc file (mã hóa) |
| **JWT decoder** | ✅ | Lấy `account_id`, `plan_type` |
| **SSE parser** | ✅ | Parse streaming response |
| **Responses API builder** | ✅ | Build request body theo schema |
| **Browser opener** | ⚠️ | Optional (CLI có; web không cần) |
| **Paste-URL fallback** | ⚠️ | Optional (cho VPS không có browser) |
| **Quota fetcher** | ⚠️ | Optional (chỉ cần khi muốn hiển thị quota) |
| **Multi-account router** | ⚠️ | Optional (cho phép nhiều tài khoản) |

---

## 3. OAuth 2.0 PKCE Flow — Chi tiết từng bước

### 3.1 Tổng quan PKCE

PKCE (Proof Key for Code Exchange, RFC 7636) là extension của OAuth 2.0 Authorization Code Grant, dành cho client public (không có secret). Quy trình:

```
1. Client sinh "code_verifier" = random string
2. Client tính "code_challenge" = BASE64URL(SHA256(verifier))
3. Client gửi challenge tới authorize endpoint
4. User authenticate → server gửi code về redirect_uri
5. Client gửi code + verifier gốc tới token endpoint
6. Server verify SHA256(verifier) == challenge → trả token
```

### 3.2 Constants cần định nghĩa

```go
const (
    OpenAIAuthURL     = "https://auth.openai.com/oauth/authorize"
    OpenAITokenURL    = "https://auth.openai.com/oauth/token"
    OpenAIClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
    OpenAIScopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
    OpenAIRedirectURI = "http://localhost:1455/auth/callback"
    CallbackPort      = "1455"
)
```

### 3.3 Sinh PKCE

```go
import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
)

func generatePKCE() (verifier, challenge string, err error) {
    // 64 bytes random → 86 ký tự base64url (đủ dài, đủ entropy)
    buf := make([]byte, 64)
    if _, err := rand.Read(buf); err != nil {
        return "", "", fmt.Errorf("generate random bytes: %w", err)
    }
    verifier = base64.RawURLEncoding.EncodeToString(buf)
    h := sha256.Sum256([]byte(verifier))
    challenge = base64.RawURLEncoding.EncodeToString(h[:])
    return verifier, challenge, nil
}
```

**Lưu ý quan trọng:** dùng `base64.RawURLEncoding` (không padding `=`), không phải `base64.URLEncoding`. OpenAI server reject nếu có padding.

### 3.4 Sinh State (chống CSRF)

```go
stateBuf := make([]byte, 16)
rand.Read(stateBuf)
state := base64.RawURLEncoding.EncodeToString(stateBuf)
```

### 3.5 Build Authorize URL

```go
import "net/url"

params := url.Values{
    "client_id":                  {OpenAIClientID},
    "redirect_uri":               {OpenAIRedirectURI},
    "response_type":              {"code"},
    "scope":                      {OpenAIScopes},
    "code_challenge":             {challenge},
    "code_challenge_method":      {"S256"},
    "state":                      {state},
    "codex_cli_simplified_flow":  {"true"},   // ← BẮT BUỘC: bật flow CLI giản lược
    "id_token_add_organizations": {"true"},   // ← gắn org info vào id_token
    "originator":                 {"pi"},     // ← định danh "Codex CLI" (Programming Interface?)
}
authURL := OpenAIAuthURL + "?" + params.Encode()
```

**3 query params đặc biệt mà ai bỏ qua sẽ thất bại:**
- `codex_cli_simplified_flow=true` — không có → OpenAI hiển thị trang consent đầy đủ thay vì flow CLI
- `id_token_add_organizations=true` — không có → JWT thiếu org metadata
- `originator=pi` — magic value của Codex CLI

### 3.6 Local Callback Server

OpenAI auth server **chỉ chấp nhận redirect_uri = `http://localhost:1455/auth/callback`**. Phải listen đúng port 1455.

```go
import (
    "context"
    "errors"
    "html"
    "log/slog"
    "net"
    "net/http"
    "sync"
)

type PendingLogin struct {
    AuthURL  string
    codeCh   chan string
    errCh    chan error
    verifier string
    state    string
    srv      *http.Server
}

func StartLogin() (*PendingLogin, error) {
    verifier, challenge, err := generatePKCE()
    if err != nil {
        return nil, err
    }
    state := generateState()

    // Build auth URL như mục 3.5...
    authURL := buildAuthURL(challenge, state)

    listener, err := net.Listen("tcp", "127.0.0.1:"+CallbackPort)
    if err != nil {
        return nil, fmt.Errorf("listen :%s: %w", CallbackPort, err)
    }

    codeCh := make(chan string, 1)
    errCh := make(chan error, 1)
    var once sync.Once

    mux := http.NewServeMux()
    mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
        once.Do(func() {
            // Verify state (anti-CSRF)
            if r.URL.Query().Get("state") != state {
                w.Header().Set("Content-Type", "text/html")
                fmt.Fprint(w, `<html><body><h2>Authentication Failed</h2><p>Invalid state.</p></body></html>`)
                errCh <- fmt.Errorf("state mismatch (possible CSRF)")
                return
            }
            code := r.URL.Query().Get("code")
            if code == "" {
                errMsg := r.URL.Query().Get("error")
                w.Header().Set("Content-Type", "text/html")
                fmt.Fprintf(w, `<html><body><h2>Authentication Failed</h2><p>%s</p></body></html>`,
                    html.EscapeString(errMsg))
                errCh <- fmt.Errorf("oauth callback: %s", errMsg)
                return
            }
            w.Header().Set("Content-Type", "text/html")
            fmt.Fprint(w, `<html><body><h2>Authentication Successful!</h2><p>You can close this window.</p></body></html>`)
            codeCh <- code
        })
    })

    srv := &http.Server{Handler: mux}
    go func() {
        if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- fmt.Errorf("callback server: %w", err)
        }
    }()

    return &PendingLogin{
        AuthURL: authURL, codeCh: codeCh, errCh: errCh,
        verifier: verifier, state: state, srv: srv,
    }, nil
}

func (p *PendingLogin) Wait(ctx context.Context) (*TokenResponse, error) {
    defer p.srv.Shutdown(context.Background())
    select {
    case code := <-p.codeCh:
        return exchangeCode(code, p.verifier)
    case err := <-p.errCh:
        return nil, err
    case <-ctx.Done():
        return nil, fmt.Errorf("auth timeout: %w", ctx.Err())
    }
}
```

**Pitfall #1: Port conflict.** Vì 1455 cố định, **chỉ 1 OAuth flow chạy được mỗi thời điểm trên máy đó**. Phải có lock/mutex để chống concurrent flows. Trong GoClaw: trả về HTTP 409 nếu flow khác đang active.

**Pitfall #2: Browser-less environment (VPS).** Nếu user chạy trên VPS không có browser, không thể redirect về `localhost:1455`. Cách workaround: user mở browser **trên máy laptop của họ**, đăng nhập, sau khi redirect bị fail (localhost không kết nối), browser hiển thị URL → user copy URL paste vào UI → server parse `code` từ URL đó.

```go
func (p *PendingLogin) ExchangeRedirectURL(redirectURL string) (*TokenResponse, error) {
    u, err := url.Parse(redirectURL)
    if err != nil {
        return nil, fmt.Errorf("invalid URL: %w", err)
    }
    state := u.Query().Get("state")
    code := u.Query().Get("code")
    if code == "" {
        if errMsg := u.Query().Get("error"); errMsg != "" {
            return nil, fmt.Errorf("OAuth error: %s", errMsg)
        }
        return nil, fmt.Errorf("no code in URL")
    }
    if state != p.state {
        return nil, fmt.Errorf("invalid state (possible CSRF)")
    }
    return exchangeCode(code, p.verifier)
}
```

### 3.7 Browser Opener (CLI mode)

```go
import (
    "os/exec"
    "runtime"
)

func openBrowser(url string) {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", url)
    case "windows":
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
    default:
        for _, opener := range []string{"xdg-open", "sensible-browser", "x-www-browser"} {
            if path, err := exec.LookPath(opener); err == nil {
                cmd = exec.Command(path, url)
                break
            }
        }
    }
    if cmd != nil {
        _ = cmd.Start()
    }
}
```

Web UI không cần — chỉ cần `window.open(authURL, "_blank")` từ frontend.

### 3.8 Token Exchange

```go
type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`     // seconds (~28 ngày cho ChatGPT subscription)
    TokenType    string `json:"token_type"`
    Scope        string `json:"scope"`
    IDToken      string `json:"id_token,omitempty"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func exchangeCode(code, verifier string) (*TokenResponse, error) {
    data := url.Values{
        "grant_type":    {"authorization_code"},
        "client_id":     {OpenAIClientID},
        "code":          {code},
        "redirect_uri":  {OpenAIRedirectURI},
        "code_verifier": {verifier},
    }
    resp, err := httpClient.PostForm(OpenAITokenURL, data)
    if err != nil {
        return nil, fmt.Errorf("token exchange: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("token exchange HTTP %d: %s", resp.StatusCode, string(body))
    }

    var t TokenResponse
    if err := json.Unmarshal(body, &t); err != nil {
        return nil, fmt.Errorf("parse token: %w", err)
    }
    return &t, nil
}
```

**Quan trọng:** dùng `application/x-www-form-urlencoded` (PostForm tự set), không phải JSON.

### 3.9 Refresh Token

```go
func refreshToken(refreshToken string) (*TokenResponse, error) {
    data := url.Values{
        "grant_type":    {"refresh_token"},
        "client_id":     {OpenAIClientID},
        "refresh_token": {refreshToken},
    }
    resp, err := httpClient.PostForm(OpenAITokenURL, data)
    if err != nil {
        return nil, fmt.Errorf("refresh: %w", err)
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("refresh HTTP %d: %s", resp.StatusCode, string(body))
    }
    var t TokenResponse
    if err := json.Unmarshal(body, &t); err != nil {
        return nil, fmt.Errorf("parse: %w", err)
    }
    return &t, nil
}
```

**Lưu ý:** Mỗi lần refresh, server có thể trả refresh_token **mới** (rotation). PHẢI lưu lại refresh_token mới nếu có, không thì lần refresh sau sẽ thất bại.

---

## 4. Token Management & Lifecycle

### 4.1 Vấn đề cần giải quyết

- Access token có TTL ~28 ngày nhưng có thể bị revoke bất cứ lúc nào
- Refresh token cũng hết hạn (vài tháng tới 1 năm)
- Concurrent requests không được "đụng nhau" khi đang refresh
- Cache để giảm DB hit
- Khi refresh fail nhưng token cũ vẫn dùng được → fallback grace

### 4.2 Pattern: Token Source với cache + auto-refresh

```go
type DBTokenSource struct {
    db           *sql.DB
    providerName string

    mu          sync.Mutex
    cachedToken string
    expiresAt   time.Time
}

const refreshMargin = 5 * time.Minute  // refresh sớm 5p trước expiry

func (ts *DBTokenSource) Token() (string, error) {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    // 1. Cache hit?
    if ts.cachedToken != "" && time.Until(ts.expiresAt) > refreshMargin {
        return ts.cachedToken, nil
    }

    // 2. Load từ DB nếu cache empty
    if ts.cachedToken == "" {
        access, expiresAt, err := ts.loadFromDB()
        if err != nil {
            return "", err
        }
        ts.cachedToken = access
        ts.expiresAt = expiresAt
    }

    // 3. Refresh nếu sắp expire
    if time.Until(ts.expiresAt) < refreshMargin {
        if err := ts.refresh(); err != nil {
            // Grace: token cũ có thể vẫn dùng được
            if ts.cachedToken != "" {
                slog.Warn("refresh failed, using existing", "error", err)
                return ts.cachedToken, nil
            }
            return "", err
        }
    }
    return ts.cachedToken, nil
}

func (ts *DBTokenSource) refresh() error {
    // Lấy refresh token từ DB (secrets table)
    rt, err := ts.loadRefreshTokenFromDB()
    if err != nil {
        return err
    }

    // Gọi refresh
    newToken, err := refreshToken(rt)
    if err != nil {
        return err
    }

    // Cập nhật cache
    ts.cachedToken = newToken.AccessToken
    ts.expiresAt = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)

    // Persist DB: access_token + new refresh_token (nếu có)
    if err := ts.updateDB(newToken); err != nil {
        slog.Warn("persist refreshed token failed", "error", err)
    }
    if newToken.RefreshToken != "" {
        ts.updateRefreshTokenInDB(newToken.RefreshToken)
    }
    return nil
}
```

### 4.3 Encryption tại rest

**Strongly recommended:** mã hóa cả `access_token` và `refresh_token` trước khi lưu DB. Dùng AES-256-GCM:

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
)

func encrypt(plaintext, key []byte) (string, error) {
    block, err := aes.NewCipher(key) // key = 32 bytes
    if err != nil { return "", err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return "", err }
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    ct := gcm.Seal(nonce, nonce, plaintext, nil)
    return base64.StdEncoding.EncodeToString(ct), nil
}

func decrypt(ciphertext string, key []byte) ([]byte, error) {
    data, _ := base64.StdEncoding.DecodeString(ciphertext)
    block, _ := aes.NewCipher(key)
    gcm, _ := cipher.NewGCM(block)
    ns := gcm.NonceSize()
    return gcm.Open(nil, data[:ns], data[ns:], nil)
}
```

**Key management:** lấy 32-byte master key từ:
- Env var (`GOCLAW_MASTER_KEY`) — simple
- OS keyring (`go-keyring`) — desktop apps
- KMS (AWS KMS, GCP KMS) — production cloud

### 4.4 Concurrency

Mutex bảo vệ `cachedToken` và `expiresAt`. Nếu 2 goroutine cùng gọi `Token()`:
- Goroutine A vào `refresh()` → mutex giữ
- Goroutine B chờ → khi A xong, B thấy cache đã fresh → return ngay

Không cần singleflight vì mutex đã đủ.

---

## 5. Trích xuất Metadata từ JWT

### 5.1 Tại sao cần?

OpenAI nhúng metadata workspace vào JWT (cả `access_token` lẫn `id_token`). Bạn cần `account_id` để gọi `/wham/usage` (quota API). `plan_type` để hiển thị UI ("Plus", "Pro"...).

### 5.2 Cấu trúc claims

```json
{
  "https://api.openai.com/auth": {
    "chatgpt_account_id": "8a7b6c5d-...",
    "chatgpt_plan_type": "plus"
  },
  "https://api.openai.com/auth.chatgpt_account_id": "8a7b6c5d-...",
  "https://api.openai.com/auth.chatgpt_plan_type": "plus",
  "iss": "https://auth.openai.com",
  "sub": "user-...",
  "exp": 1735689600,
  ...
}
```

OpenAI dùng cả hai format: nested object lẫn dotted-key (để tương thích nhiều client).

### 5.3 Parse JWT KHÔNG verify signature

Vì token là **của chính user mình** (server đã verify khi cấp), client không cần verify lại — chỉ cần decode payload:

```go
import (
    "encoding/base64"
    "encoding/json"
    "strings"
)

type JWTClaims struct {
    Auth      *NestedAuth `json:"https://api.openai.com/auth"`
    AccountID string      `json:"https://api.openai.com/auth.chatgpt_account_id"`
    PlanType  string      `json:"https://api.openai.com/auth.chatgpt_plan_type"`
}

type NestedAuth struct {
    AccountID string `json:"chatgpt_account_id"`
    PlanType  string `json:"chatgpt_plan_type"`
}

type Metadata struct {
    AccountID string
    PlanType  string
}

func parseJWTMetadata(token string) (Metadata, bool) {
    token = strings.TrimSpace(token)
    parts := strings.Split(token, ".")
    if len(parts) < 2 {
        return Metadata{}, false
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil {
        return Metadata{}, false
    }
    var c JWTClaims
    if err := json.Unmarshal(payload, &c); err != nil {
        return Metadata{}, false
    }
    m := Metadata{
        AccountID: firstNonEmpty(c.AccountID, nested(c.Auth, "AccountID")),
        PlanType:  firstNonEmpty(c.PlanType, nested(c.Auth, "PlanType")),
    }
    if m.AccountID == "" && m.PlanType == "" {
        return Metadata{}, false
    }
    return m, true
}

func nested(a *NestedAuth, field string) string {
    if a == nil { return "" }
    switch field {
    case "AccountID": return a.AccountID
    case "PlanType":  return a.PlanType
    }
    return ""
}

func firstNonEmpty(vs ...string) string {
    for _, v := range vs {
        if v = strings.TrimSpace(v); v != "" {
            return v
        }
    }
    return ""
}

// Helper: thử cả id_token rồi access_token
func extractMetadata(t *TokenResponse) Metadata {
    for _, tok := range []string{t.IDToken, t.AccessToken} {
        if m, ok := parseJWTMetadata(tok); ok {
            return m
        }
    }
    return Metadata{}
}
```

**Lưu ý:** `id_token` thường đầy đủ metadata hơn `access_token`. Ưu tiên `id_token`.

### 5.4 Backfill cho legacy users

Nếu user đã đăng nhập từ trước (version cũ chưa parse metadata), `account_id` empty trong DB → quota API thất bại. Cần background job hoặc lazy backfill:

```go
func (ts *DBTokenSource) BackfillMetadata(ctx context.Context) error {
    provider, _ := ts.loadProvider(ctx)
    if provider.AccountID != "" {
        return nil // already have it
    }
    // Thử parse từ access token hiện tại
    if m, ok := parseJWTMetadata(provider.AccessToken); ok {
        return ts.updateMetadata(ctx, m)
    }
    // Force refresh — token mới có thể có metadata
    if err := ts.refresh(); err != nil {
        return err
    }
    provider, _ = ts.loadProvider(ctx)
    if m, ok := parseJWTMetadata(provider.AccessToken); ok {
        return ts.updateMetadata(ctx, m)
    }
    return errors.New("no metadata in token")
}
```

---

## 6. Gọi LLM qua Responses API

### 6.1 Endpoint

```
POST https://chatgpt.com/backend-api/codex/responses
```

**KHÔNG phải** `api.openai.com/v1/chat/completions`. Đây là API hoàn toàn khác (Responses API thay vì Chat Completions API).

### 6.2 Headers bắt buộc

```http
Authorization: Bearer <access_token>
Content-Type: application/json
OpenAI-Beta: responses=v1
```

`OpenAI-Beta: responses=v1` là **bắt buộc** — server từ chối nếu thiếu.

### 6.3 Request body

```json
{
  "model": "gpt-5.4",
  "stream": true,
  "store": false,
  "instructions": "You are a helpful assistant.",
  "input": [
    {"role": "user", "content": "Hello"}
  ]
}
```

**Khác biệt với Chat Completions API:**

| Chat Completions | Responses API |
|---|---|
| `messages: [{role, content}]` | `input: [...]` + `instructions: "..."` |
| System prompt trong messages | `instructions` field riêng |
| `temperature`, `top_p` | (nhiều param không có) |
| Tool result: `{role: "tool", tool_call_id, content}` | `{type: "function_call_output", call_id, output}` |
| `stream: true/false` | `stream: true` **luôn luôn bắt buộc** |

**Field `store: false`** — không lưu conversation server-side. Set false trừ khi muốn ChatGPT lưu lại.

### 6.4 Build request body — Đầy đủ

```go
type Message struct {
    Role       string
    Content    string
    Images     []ImageInput  // base64 + mime type
    ToolCalls  []ToolCall    // assistant với tool calls
    ToolCallID string        // tool result
}

type ImageInput struct {
    MimeType string
    Data     string // base64
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments map[string]any
}

type ToolDef struct {
    Type        string         // "function" hoặc "image_generation"
    Name        string
    Description string
    Parameters  map[string]any // JSON schema
}

type ChatRequest struct {
    Model    string
    Messages []Message
    Tools    []ToolDef
    Options  map[string]any // {"thinking_level": "medium"}
}

func buildCodexRequestBody(req ChatRequest, stream bool) map[string]any {
    var instructions string
    var input []any

    for _, m := range req.Messages {
        switch m.Role {
        case "system":
            if instructions == "" {
                instructions = m.Content
            } else {
                instructions += "\n\n" + m.Content
            }

        case "user":
            if len(m.Images) > 0 {
                var parts []map[string]any
                for _, img := range m.Images {
                    parts = append(parts, map[string]any{
                        "type":      "input_image",
                        "image_url": fmt.Sprintf("data:%s;base64,%s", img.MimeType, img.Data),
                    })
                }
                if m.Content != "" {
                    parts = append(parts, map[string]any{
                        "type": "input_text",
                        "text": m.Content,
                    })
                }
                input = append(input, map[string]any{
                    "role":    "user",
                    "content": parts,
                })
            } else {
                input = append(input, map[string]any{
                    "role":    "user",
                    "content": m.Content,
                })
            }

        case "assistant":
            // Tool calls
            for _, tc := range m.ToolCalls {
                argsJSON, _ := json.Marshal(tc.Arguments)
                callID := toFcID(tc.ID) // ép prefix "fc_"
                input = append(input, map[string]any{
                    "type":      "function_call",
                    "id":        callID,
                    "call_id":   callID,
                    "name":      tc.Name,
                    "arguments": string(argsJSON),
                })
            }
            // Text content
            if m.Content != "" {
                input = append(input, map[string]any{
                    "type": "message",
                    "role": "assistant",
                    "content": []map[string]any{
                        {"type": "output_text", "text": m.Content},
                    },
                })
            }

        case "tool":
            // Kết quả tool execution
            input = append(input, map[string]any{
                "type":    "function_call_output",
                "call_id": toFcID(m.ToolCallID),
                "output":  m.Content,
            })
        }
    }

    if instructions == "" {
        instructions = "You are a helpful assistant."
    }

    body := map[string]any{
        "model":        req.Model,
        "input":        input,
        "stream":       stream,
        "store":        false,
        "instructions": instructions,
    }

    if len(req.Tools) > 0 {
        var tools []map[string]any
        for _, t := range req.Tools {
            if t.Type == "image_generation" {
                tools = append(tools, map[string]any{
                    "type":           "image_generation",
                    "action":         "generate",
                    "model":          "gpt-image-2",
                    "output_format":  "png",
                    "partial_images": 1,
                })
            } else {
                tools = append(tools, map[string]any{
                    "type":        "function",
                    "name":        t.Name,
                    "description": t.Description,
                    "parameters":  t.Parameters,
                })
            }
        }
        body["tools"] = tools
    }

    if level, ok := req.Options["thinking_level"].(string); ok && level != "" && level != "off" {
        body["reasoning"] = map[string]any{"effort": level} // "minimal", "low", "medium", "high"
    }

    return body
}
```

### 6.5 Tool Call ID quy ước

Responses API yêu cầu tool call ID match regex `^[a-zA-Z0-9_-]+$` và Codex CLI dùng prefix `fc_`. Sanitizer:

```go
import "regexp"

var invalidFcID = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func toFcID(id string) string {
    // Strip prefix cũ
    for _, p := range []string{"tool_", "call_", "fc_"} {
        if strings.HasPrefix(id, p) {
            id = id[len(p):]
            break
        }
    }
    id = invalidFcID.ReplaceAllString(id, "_")
    return "fc_" + id
}
```

### 6.6 Gửi request

```go
func doRequest(ctx context.Context, accessToken string, body any) (io.ReadCloser, error) {
    data, _ := json.Marshal(body)

    req, err := http.NewRequestWithContext(ctx, "POST",
        "https://chatgpt.com/backend-api/codex/responses",
        bytes.NewReader(data))
    if err != nil { return nil, err }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("OpenAI-Beta", "responses=v1")

    client := &http.Client{Timeout: 0} // no timeout — stream có thể lâu
    resp, err := client.Do(req)
    if err != nil { return nil, err }

    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
        resp.Body.Close()
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
    }
    return resp.Body, nil
}
```

### 6.7 Parse SSE Stream

SSE format chuẩn (`data: <json>\n\n`). Mỗi data line là một event JSON.

```go
import "bufio"

func parseStream(body io.ReadCloser, onChunk func(StreamChunk)) (*ChatResponse, error) {
    defer body.Close()
    result := &ChatResponse{FinishReason: "stop"}
    toolCalls := map[string]*toolAcc{}

    sc := bufio.NewScanner(body)
    sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // 10MB max line

    for sc.Scan() {
        line := sc.Text()
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        payload := strings.TrimPrefix(line, "data: ")
        if payload == "[DONE]" {
            break
        }

        var event codexSSEEvent
        if err := json.Unmarshal([]byte(payload), &event); err != nil {
            continue
        }
        if err := processEvent(&event, result, toolCalls, onChunk); err != nil {
            return nil, err
        }
    }
    return result, sc.Err()
}
```

### 6.8 SSE Event Types — Bảng đầy đủ

| Event `type` | Trường quan trọng | Ý nghĩa |
|---|---|---|
| `response.output_item.added` | `item_id`, `output_index`, `item` | Bắt đầu output item mới (message/function_call/reasoning/image_generation_call) |
| `response.output_text.delta` | `item_id`, `delta` | Text token streaming (gọi `onChunk` ngay) |
| `response.output_text.done` | `item_id`, `text` | Text hoàn tất cho 1 item |
| `response.content_part.done` | `part.type=output_text`, `part.text` | Alternate completion event |
| `response.function_call_arguments.delta` | `item_id`, `delta` | Tool args streaming (JSON ghép từng chunk) |
| `response.output_item.done` | `item.type`, `item.*` | Item hoàn tất — chứa toàn bộ dữ liệu |
| `response.image_generation_call.partial_image` | `partial_image_b64`, `output_format` | Frame intermediate khi sinh ảnh |
| `response.completed` | `response.usage`, `response.output[]` | Stream xong, có usage tokens |
| `response.incomplete` | `response.status` | Bị truncated → `finish_reason = "length"` |
| `response.failed` | `response.error.{code,message}` | Lỗi mid-stream |

### 6.9 Process Event — Full implementation

```go
type StreamChunk struct {
    Content  string
    Thinking string
    Images   []ImageOutput
    Done     bool
}

type ImageOutput struct {
    MimeType string
    Data     string // base64
    Partial  bool
}

type ChatResponse struct {
    Content      string
    Thinking     string
    ToolCalls    []ToolCallOut
    Images       []ImageOutput
    Usage        *Usage
    FinishReason string
}

type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    ThinkingTokens   int
}

type toolAcc struct {
    callID  string
    name    string
    rawArgs string
}

func processEvent(event *codexSSEEvent, result *ChatResponse, toolCalls map[string]*toolAcc, onChunk func(StreamChunk)) error {
    switch event.Type {

    case "response.output_text.delta":
        result.Content += event.Delta
        if onChunk != nil {
            onChunk(StreamChunk{Content: event.Delta})
        }

    case "response.output_text.done":
        // text đã đầy đủ trong delta, không cần làm gì

    case "response.function_call_arguments.delta":
        if event.ItemID == "" { return nil }
        acc := toolCalls[event.ItemID]
        if acc == nil {
            acc = &toolAcc{}
            toolCalls[event.ItemID] = acc
        }
        acc.rawArgs += event.Delta

    case "response.output_item.done":
        if event.Item == nil { return nil }
        switch event.Item.Type {
        case "function_call":
            acc := toolCalls[event.Item.ID]
            if acc == nil { acc = &toolAcc{}; toolCalls[event.Item.ID] = acc }
            acc.callID = event.Item.CallID
            acc.name = event.Item.Name
            if event.Item.Arguments != "" {
                acc.rawArgs = event.Item.Arguments
            }
        case "reasoning":
            for _, s := range event.Item.Summary {
                if s.Text != "" {
                    result.Thinking += s.Text
                    if onChunk != nil { onChunk(StreamChunk{Thinking: s.Text}) }
                }
            }
        case "image_generation_call":
            if event.Item.Result != "" {
                img := ImageOutput{
                    MimeType: mimeFromFormat(event.Item.OutputFormat),
                    Data:     event.Item.Result,
                    Partial:  false,
                }
                result.Images = append(result.Images, img)
                if onChunk != nil { onChunk(StreamChunk{Images: []ImageOutput{img}}) }
            }
        }

    case "response.image_generation_call.partial_image":
        if onChunk != nil {
            onChunk(StreamChunk{Images: []ImageOutput{{
                MimeType: mimeFromFormat(event.OutputFormat),
                Data:     event.PartialImageB64,
                Partial:  true,
            }}})
        }

    case "response.completed", "response.incomplete":
        if event.Response != nil && event.Response.Usage != nil {
            u := event.Response.Usage
            result.Usage = &Usage{
                PromptTokens:     u.InputTokens,
                CompletionTokens: u.OutputTokens,
                TotalTokens:      u.TotalTokens,
            }
            if u.OutputTokensDetails != nil {
                result.Usage.ThinkingTokens = u.OutputTokensDetails.ReasoningTokens
            }
        }
        if event.Response != nil && event.Response.Status == "incomplete" {
            result.FinishReason = "length"
        }

    case "response.failed":
        msg := "response failed"
        if event.Response != nil && event.Response.Error != nil {
            if event.Response.Error.Message != "" {
                msg = event.Response.Error.Message
            } else {
                msg = event.Response.Error.Code
            }
        }
        return errors.New(msg)
    }

    return nil
}

func mimeFromFormat(format string) string {
    switch format {
    case "jpeg": return "image/jpeg"
    case "webp": return "image/webp"
    default:     return "image/png"
    }
}

// Sau khi stream xong, build tool_calls từ accumulator
func finalizeToolCalls(result *ChatResponse, toolCalls map[string]*toolAcc) {
    for _, acc := range toolCalls {
        if acc.name == "" { continue }
        args := map[string]any{}
        if acc.rawArgs != "" {
            json.Unmarshal([]byte(acc.rawArgs), &args)
        }
        result.ToolCalls = append(result.ToolCalls, ToolCallOut{
            ID:        acc.callID,
            Name:      acc.name,
            Arguments: args,
        })
    }
    if len(result.ToolCalls) > 0 && result.FinishReason != "length" {
        result.FinishReason = "tool_calls"
    }
}
```

### 6.10 Wire types tham chiếu

```go
type codexSSEEvent struct {
    Type              string            `json:"type"`
    Delta             string            `json:"delta,omitempty"`
    Text              string            `json:"text,omitempty"`
    ItemID            string            `json:"item_id,omitempty"`
    OutputIndex       int               `json:"output_index,omitempty"`
    ContentIndex      int               `json:"content_index,omitempty"`
    Item              *codexItem        `json:"item,omitempty"`
    Part              *codexContentPart `json:"part,omitempty"`
    Response          *codexAPIResponse `json:"response,omitempty"`
    OutputFormat      string            `json:"output_format,omitempty"`
    PartialImageB64   string            `json:"partial_image_b64,omitempty"`
    PartialImageIndex int               `json:"partial_image_index,omitempty"`
}

type codexItem struct {
    ID           string         `json:"id"`
    Type         string         `json:"type"` // "message"|"function_call"|"reasoning"|"image_generation_call"
    Role         string         `json:"role,omitempty"`
    Content      []codexContent `json:"content,omitempty"`
    CallID       string         `json:"call_id,omitempty"`
    Name         string         `json:"name,omitempty"`
    Arguments    string         `json:"arguments,omitempty"`
    Summary      []codexSummary `json:"summary,omitempty"`
    OutputFormat string         `json:"output_format,omitempty"`
    Result       string         `json:"result,omitempty"` // base64 image
}

type codexContent struct {
    Type string `json:"type"` // "output_text"
    Text string `json:"text"`
}

type codexSummary struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type codexAPIResponse struct {
    ID     string            `json:"id"`
    Status string            `json:"status"` // "completed"|"incomplete"|"failed"
    Output []codexItem       `json:"output"`
    Usage  *codexUsage       `json:"usage,omitempty"`
    Error  *codexErrorDetail `json:"error,omitempty"`
}

type codexUsage struct {
    InputTokens         int                 `json:"input_tokens"`
    OutputTokens        int                 `json:"output_tokens"`
    TotalTokens         int                 `json:"total_tokens"`
    OutputTokensDetails *codexTokensDetails `json:"output_tokens_details,omitempty"`
}

type codexTokensDetails struct {
    ReasoningTokens int `json:"reasoning_tokens"`
}

type codexErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type codexContentPart struct {
    Type string `json:"type"`
    Text string `json:"text,omitempty"`
}
```

### 6.11 Danh sách models hỗ trợ

Codex CLI cho phép các model sau (cập nhật theo Codex CLI version):
- `gpt-5.4` (default cho ChatGPT Plus)
- `gpt-5.3-codex`
- `gpt-5-mini`
- `o4-mini`, `o3-mini`
- `gpt-4.1`, `gpt-4o`

Lưu ý: tùy plan_type (Plus/Pro/Business) sẽ có quyền truy cập khác nhau.

---

## 7. Quota API — Kiểm tra hạn mức

### 7.1 Endpoint & Headers

```http
GET https://chatgpt.com/backend-api/wham/usage
Authorization: Bearer <access_token>
ChatGPT-Account-Id: <account_id từ JWT>
User-Agent: codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal
```

**3 thứ bắt buộc:**
1. `Bearer` token còn hạn
2. Header `ChatGPT-Account-Id` (lấy từ JWT)
3. `User-Agent` **mạo danh Codex CLI** — nếu dùng User-Agent khác có khả năng bị 403

### 7.2 Response

```json
{
  "plan_type": "plus",
  "rate_limit": {
    "primary_window": {
      "used_percent": 45.5,
      "reset_after_seconds": 3600
    },
    "secondary_window": {
      "used_percent": 12.3,
      "reset_after_seconds": 604800
    }
  },
  "code_review_rate_limit": {
    "primary_window": { "used_percent": 0, "reset_after_seconds": 3600 }
  }
}
```

- `primary_window` = 5-hour window (limit chính cho Plus)
- `secondary_window` = weekly window (limit tổng)
- `code_review_rate_limit` = limit riêng cho code review feature

OpenAI có thể trả cả `camelCase` (`rateLimit`, `primaryWindow`) lẫn `snake_case`. Parse cả 2:

```go
type usageResponse struct {
    PlanType            string         `json:"plan_type"`
    PlanTypeCamel       string         `json:"planType"`
    RateLimit           *usageWindows  `json:"rate_limit"`
    RateLimitCamel      *usageWindows  `json:"rateLimit"`
    CodeReviewRateLimit *usageWindows  `json:"code_review_rate_limit"`
    CodeReviewCamel     *usageWindows  `json:"codeReviewRateLimit"`
}

type usageWindows struct {
    PrimaryWindow   *usageWindow `json:"primary_window"`
    PrimaryCamel    *usageWindow `json:"primaryWindow"`
    SecondaryWindow *usageWindow `json:"secondary_window"`
    SecondaryCamel  *usageWindow `json:"secondaryWindow"`
}

type usageWindow struct {
    UsedPercent       *float64 `json:"used_percent"`
    UsedPercentCamel  *float64 `json:"usedPercent"`
    ResetAfterSeconds *int     `json:"reset_after_seconds"`
    ResetAfterCamel   *int     `json:"resetAfterSeconds"`
}
```

### 7.3 Implementation

```go
func fetchQuota(ctx context.Context, accessToken, accountID string) (*usageResponse, error) {
    req, _ := http.NewRequestWithContext(ctx, "GET",
        "https://chatgpt.com/backend-api/wham/usage", nil)
    req.Header.Set("Authorization", "Bearer "+accessToken)
    req.Header.Set("ChatGPT-Account-Id", accountID)
    req.Header.Set("User-Agent", "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal")

    client := &http.Client{Timeout: 12 * time.Second}
    resp, err := client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return nil, fmt.Errorf("quota HTTP %d: %s", resp.StatusCode, string(body))
    }
    var u usageResponse
    if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
        return nil, err
    }
    return &u, nil
}
```

### 7.4 Status code mapping

| HTTP status | Ý nghĩa | Action |
|---|---|---|
| 200 | OK | Hiển thị quota |
| 401 | Token invalid/expired | Force re-auth |
| 402 | Billing issue (subscription expired/cancelled) | Yêu cầu user check billing |
| 403 | Account không cho phép quota API | Disable quota UI, vẫn cho gọi chat |
| 404 | Endpoint không tồn tại (provider lạ) | Skip quota silent |
| 429 | Rate limited | Retry sau Retry-After |
| 5xx | Server error | Retry exponential backoff |

### 7.5 Caching quota

Quota không thay đổi theo từng request. Cache 20-60 giây:

```go
type QuotaCache struct {
    mu       sync.Mutex
    cached   *QuotaResult
    cachedAt time.Time
}

const quotaTTL = 20 * time.Second

func (qc *QuotaCache) Get(ctx context.Context, fetch func() (*QuotaResult, error)) *QuotaResult {
    qc.mu.Lock()
    if qc.cached != nil && time.Since(qc.cachedAt) < quotaTTL {
        defer qc.mu.Unlock()
        return qc.cached
    }
    cached := qc.cached
    qc.mu.Unlock()

    // Stale-while-revalidate: trả cache cũ ngay, refresh background
    if cached != nil {
        go func() {
            if fresh, err := fetch(); err == nil {
                qc.mu.Lock()
                qc.cached = fresh
                qc.cachedAt = time.Now()
                qc.mu.Unlock()
            }
        }()
        return cached
    }

    // Lần đầu — fetch sync
    fresh, err := fetch()
    if err != nil { return nil }
    qc.mu.Lock()
    qc.cached = fresh
    qc.cachedAt = time.Now()
    qc.mu.Unlock()
    return fresh
}
```

---

## 8. Database Schema

### 8.1 Minimal schema (PostgreSQL)

```sql
-- Bảng providers — lưu access_token + metadata
CREATE TABLE llm_providers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL,                    -- nếu multi-tenant; bỏ nếu single-user
    name          TEXT NOT NULL,                    -- "openai-codex" (slug)
    display_name  TEXT,                             -- "Tài khoản của Quân"
    provider_type TEXT NOT NULL,                    -- "chatgpt_oauth"
    api_base      TEXT NOT NULL DEFAULT 'https://chatgpt.com/backend-api',
    api_key       TEXT NOT NULL,                    -- access_token (encrypted)
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    settings      JSONB NOT NULL DEFAULT '{}',      -- {expires_at, scopes, account_id, plan_type}
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, name)
);

-- Bảng secrets — refresh_token (tách riêng để tăng bảo mật)
CREATE TABLE config_secrets (
    tenant_id  UUID NOT NULL,
    key        TEXT NOT NULL,                       -- "oauth.openai-codex.refresh_token"
    value      TEXT NOT NULL,                       -- encrypted
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, key)
);

CREATE INDEX idx_providers_type ON llm_providers (provider_type) WHERE enabled = TRUE;
```

### 8.2 SQLite (cho desktop app)

```sql
CREATE TABLE llm_providers (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL UNIQUE,
    display_name  TEXT,
    provider_type TEXT NOT NULL,
    api_base      TEXT NOT NULL,
    api_key       TEXT NOT NULL,
    enabled       INTEGER NOT NULL DEFAULT 1,
    settings      TEXT NOT NULL DEFAULT '{}',       -- JSON string
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE config_secrets (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

### 8.3 Settings JSONB structure

```json
{
  "expires_at": 1735689600,
  "scopes": "openid profile email offline_access ...",
  "account_id": "8a7b6c5d-1234-5678-abcd-ef0123456789",
  "plan_type": "plus"
}
```

### 8.4 Encryption ở đâu

- `llm_providers.api_key` (access_token) — **encrypted**
- `config_secrets.value` (refresh_token) — **encrypted**
- `llm_providers.settings` — clear text (chỉ metadata, không nhạy cảm)

---

## 9. HTTP API Design (REST cho UI)

5 endpoint cốt lõi. Tất cả gate bằng admin role.

### 9.1 GET /v1/auth/chatgpt/{provider}/status

Check xem provider đã đăng nhập chưa.

```http
GET /v1/auth/chatgpt/openai-codex/status

200 OK
{
  "authenticated": true,
  "provider_name": "openai-codex"
}

200 OK (chưa login)
{
  "authenticated": false
}
```

### 9.2 POST /v1/auth/chatgpt/{provider}/start

Start OAuth flow, trả về auth URL.

```http
POST /v1/auth/chatgpt/openai-codex/start
{
  "display_name": "Quân's ChatGPT Plus",
  "api_base": ""   // optional, default = chatgpt.com/backend-api
}

200 OK
{
  "auth_url": "https://auth.openai.com/oauth/authorize?...",
  "provider_name": "openai-codex"
}

200 OK (đã login)
{
  "status": "already_authenticated",
  "provider_name": "openai-codex"
}

409 Conflict (flow khác đang active)
{
  "error": "another OAuth flow is already active on this server"
}
```

Backend lưu pending flow vào map `flowKey → PendingLogin`, background goroutine `Wait()` để bắt code khi user authenticate xong. Timeout 6 phút.

### 9.3 POST /v1/auth/chatgpt/{provider}/callback

Manual paste URL (fallback cho remote/VPS).

```http
POST /v1/auth/chatgpt/openai-codex/callback
{
  "redirect_url": "http://localhost:1455/auth/callback?code=...&state=..."
}

200 OK
{
  "authenticated": true,
  "provider_name": "openai-codex",
  "provider_id": "..."
}

400 Bad Request
{
  "error": "invalid state parameter (possible CSRF)"
}
```

### 9.4 POST /v1/auth/chatgpt/{provider}/logout

Xóa tokens và provider khỏi DB.

```http
POST /v1/auth/chatgpt/openai-codex/logout

200 OK
{ "status": "logged out" }
```

### 9.5 GET /v1/auth/chatgpt/{provider}/quota

Lấy thông tin quota để hiển thị progress bar.

```http
GET /v1/auth/chatgpt/openai-codex/quota

200 OK
{
  "success": true,
  "plan_type": "plus",
  "windows": [
    {
      "label": "Primary",
      "used_percent": 45,
      "remaining_percent": 55,
      "reset_after_seconds": 3600,
      "reset_at": "2026-05-17T15:30:00Z"
    },
    {
      "label": "Secondary",
      "used_percent": 12,
      "remaining_percent": 88,
      "reset_after_seconds": 604800,
      "reset_at": "2026-05-24T10:30:00Z"
    }
  ],
  "core_usage": {
    "five_hour": { "label": "Primary", "remaining_percent": 55, ... },
    "weekly":    { "label": "Secondary", "remaining_percent": 88, ... }
  }
}

200 OK (quota fail, vẫn 200 với success:false)
{
  "success": false,
  "error": "Token expired or invalid.",
  "error_code": "reauth_required",
  "action_hint": "Sign in again to refresh.",
  "needs_reauth": true
}
```

---

## 10. UI/UX Flow

### 10.1 Sequence diagram

```
User                Web UI               Backend          OpenAI Auth        OpenAI API
 │                    │                    │                   │                  │
 │ Click "Sign in"    │                    │                   │                  │
 ├───────────────────►│                    │                   │                  │
 │                    │ POST /start        │                   │                  │
 │                    ├───────────────────►│                   │                  │
 │                    │                    │ Start :1455       │                  │
 │                    │                    │ Build authURL     │                  │
 │                    │ {auth_url}         │                   │                  │
 │                    │◄───────────────────┤                   │                  │
 │                    │                    │                   │                  │
 │                    │ window.open(authURL)                   │                  │
 │ Browser opens      │                    │                   │                  │
 │◄───────────────────┴─────────────────────────────────────────►                  │
 │                    │                    │                   │                  │
 │ Login + consent    │                    │                   │                  │
 ├──────────────────────────────────────────────────────────────►                  │
 │                    │                    │                   │                  │
 │ Redirect to localhost:1455/auth/callback?code=...           │                  │
 │◄─────────────────────────────────────────────────────────────┤                  │
 │                                                              │                  │
 │ Browser → localhost:1455 (backend's local server)            │                  │
 ├─────────────────────────────────────────►                    │                  │
 │                                          │                   │                  │
 │                                          │ POST /token       │                  │
 │                                          ├──────────────────►│                  │
 │                                          │ {access, refresh} │                  │
 │                                          │◄──────────────────┤                  │
 │                                          │                   │                  │
 │                                          │ Save to DB        │                  │
 │                                          │ Register provider │                  │
 │                                          │                   │                  │
 │ "Authentication Successful!" page        │                   │                  │
 │◄─────────────────────────────────────────┤                   │                  │
 │                                                              │                  │
 │                    │ GET /status (poll mỗi 2s)              │                  │
 │                    ├───────────────────►│                    │                  │
 │                    │ {authenticated: true}                  │                  │
 │                    │◄───────────────────┤                    │                  │
 │                    │                    │                                       │
 │                    │ Show "✅ Authenticated"                                    │
 │◄───────────────────┤                                                             │
```

### 10.2 React component skeleton

```tsx
function ChatGPTOAuthSection({ providerName }: { providerName: string }) {
  const [status, setStatus] = useState<"loading" | "logged_in" | "logged_out" | "waiting">("loading");
  const [authUrl, setAuthUrl] = useState<string | null>(null);
  const [pasteUrl, setPasteUrl] = useState("");
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    fetchStatus();
    return () => { if (pollRef.current) clearInterval(pollRef.current); };
  }, []);

  async function fetchStatus() {
    const res = await fetch(`/v1/auth/chatgpt/${providerName}/status`);
    const data = await res.json();
    setStatus(data.authenticated ? "logged_in" : "logged_out");
  }

  async function handleLogin() {
    const res = await fetch(`/v1/auth/chatgpt/${providerName}/start`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ display_name: "My Account" }),
    });
    const data = await res.json();

    if (data.status === "already_authenticated") {
      setStatus("logged_in");
      return;
    }
    if (data.auth_url) {
      setAuthUrl(data.auth_url);
      window.open(data.auth_url, "_blank", "noopener,noreferrer");
      setStatus("waiting");

      // Poll status mỗi 2s, max 6 phút
      pollRef.current = setInterval(async () => {
        const s = await fetch(`/v1/auth/chatgpt/${providerName}/status`).then(r => r.json());
        if (s.authenticated) {
          clearInterval(pollRef.current!);
          setStatus("logged_in");
        }
      }, 2000) as unknown as number;

      setTimeout(() => {
        if (pollRef.current) clearInterval(pollRef.current);
        if (status === "waiting") setStatus("logged_out");
      }, 6 * 60 * 1000);
    }
  }

  async function handlePasteFallback() {
    await fetch(`/v1/auth/chatgpt/${providerName}/callback`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ redirect_url: pasteUrl.trim() }),
    });
    setPasteUrl("");
    await fetchStatus();
  }

  async function handleLogout() {
    await fetch(`/v1/auth/chatgpt/${providerName}/logout`, { method: "POST" });
    setStatus("logged_out");
  }

  if (status === "loading") return <div>Đang kiểm tra...</div>;

  if (status === "logged_in") {
    return (
      <div>
        ✅ Đã đăng nhập <button onClick={handleLogout}>Đăng xuất</button>
      </div>
    );
  }

  if (status === "waiting") {
    return (
      <div>
        <p>Đang chờ bạn hoàn tất đăng nhập trong cửa sổ mới...</p>
        <p>Nếu browser không tự đóng, copy URL redirect (bắt đầu bằng <code>http://localhost:1455</code>) và paste vào đây:</p>
        <input value={pasteUrl} onChange={e => setPasteUrl(e.target.value)} placeholder="http://localhost:1455/auth/callback?code=..." />
        <button onClick={handlePasteFallback}>Submit</button>
      </div>
    );
  }

  return <button onClick={handleLogin}>Đăng nhập với ChatGPT</button>;
}
```

### 10.3 UX gotchas

- **Popup blocker:** một số browser block `window.open` nếu không gọi từ user click handler trực tiếp. PHẢI gọi `window.open` đồng bộ trong `onClick` — không qua await.
- **Browser cache:** sau khi login xong, tab cũ có thể vẫn hiển thị "Authentication Successful" — UX tốt: thêm `<meta http-equiv="refresh" content="3;url=about:blank">` để tự đóng.
- **Multiple tabs:** nếu user mở UI ở 2 tab và click login ở cả 2 → tab 2 nhận 409. UX: hiển thị toast "Đang có flow OAuth khác active, vui lòng thử lại sau."

---

## 11. Multi-Account Pool (Nâng cao)

Cho phép user login **nhiều tài khoản ChatGPT** để vòng tránh khi 1 account hết quota.

### 11.1 Strategy

- **Round-robin:** lần lượt account 1, 2, 3, 1, 2, 3, ...
- **Priority order:** ưu tiên account 1; chỉ rơi xuống account 2 khi account 1 fail

### 11.2 Route Eligibility (sức khỏe account)

```go
type RouteEligibility struct {
    Class  string // "healthy" | "unknown" | "blocked"
    Reason string // "exhausted" | "reauth" | "forbidden" | "billing" | ...
}

// Healthy: còn quota
// Unknown: chưa biết (cache empty hoặc đang fetch)
// Blocked: hết quota / token expired / billing fail
```

Mỗi account có quota cache (mục 7.5). `RouteEligibility` tính từ quota:
- `remaining_percent > 0` cho ít nhất 1 window → healthy
- Tất cả window 0% → blocked với reason "exhausted"
- Quota fetch fail vì 401 → blocked "reauth"

### 11.3 Router implementation skeleton

```go
type Pool struct {
    accounts []*Account // mỗi account = 1 CodexProvider + token source
    strategy string     // "round_robin" | "priority_order"
    counter  atomic.Uint64
}

func (p *Pool) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    ordered := p.orderedAccounts(ctx)
    if len(ordered) == 0 {
        return nil, errors.New("no available accounts")
    }
    var lastErr error
    for i, acc := range ordered {
        resp, err := acc.provider.Chat(ctx, req)
        if err == nil { return resp, nil }
        lastErr = err
        if !isRetryable(err) || i == len(ordered)-1 {
            return nil, err
        }
        slog.Warn("failover", "from", acc.name, "to", ordered[i+1].name, "err", err)
    }
    return nil, lastErr
}

func (p *Pool) orderedAccounts(ctx context.Context) []*Account {
    candidates := p.healthyAccounts(ctx) // filter blocked
    if p.strategy == "priority_order" {
        return candidates // giữ thứ tự
    }
    // Round-robin: shift start position
    if len(candidates) <= 1 { return candidates }
    start := int(p.counter.Add(1)-1) % len(candidates)
    return append(candidates[start:], candidates[:start]...)
}
```

### 11.4 Modality-aware round-robin

Chat traffic và image traffic có thể rotate độc lập (chat dùng nhiều quota chính, image dùng quota riêng). GoClaw dùng counter riêng cho mỗi modality:

```go
counters := map[string]*atomic.Uint64{
    "chat":  {},
    "image": {},
}
```

---

## 12. Production Hardening — Edge Cases & Errors

### 12.1 Error classification

| Error | Hành động |
|---|---|
| HTTP 401 (chat) | Refresh token → retry 1 lần → nếu vẫn fail, force re-auth |
| HTTP 402 | Hiển thị "Subscription expired/cancelled", disable provider |
| HTTP 403 (chat) | Account bị restrict → disable provider, log warning |
| HTTP 429 (chat) | Parse `Retry-After` header → exponential backoff |
| HTTP 5xx (chat) | Retry với jitter (3 attempts, base 1s, max 30s) |
| Network timeout | Retry với jitter |
| Stream interrupted | Trả về partial content + log warning |
| `response.failed` SSE event | Treat as error, propagate `error.message` |
| Refresh token 400 `invalid_grant` | Refresh token đã hết hạn → force re-auth |

### 12.2 Retry pattern

```go
type RetryConfig struct {
    Attempts int
    MinDelay time.Duration
    MaxDelay time.Duration
    Jitter   float64
}

func RetryDo[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
    var zero T
    var lastErr error
    delay := cfg.MinDelay
    for i := 0; i < cfg.Attempts; i++ {
        v, err := fn()
        if err == nil { return v, nil }
        lastErr = err
        if !isRetryable(err) { return zero, err }
        if i == cfg.Attempts-1 { break }

        // Jitter ±10%
        jitterMs := float64(delay) * cfg.Jitter * (rand.Float64()*2 - 1)
        sleep := delay + time.Duration(jitterMs)
        select {
        case <-time.After(sleep):
        case <-ctx.Done():
            return zero, ctx.Err()
        }
        delay = time.Duration(math.Min(float64(delay)*2, float64(cfg.MaxDelay)))
    }
    return zero, lastErr
}

func isRetryable(err error) bool {
    var httpErr *HTTPError
    if errors.As(err, &httpErr) {
        return httpErr.Status == 408 || httpErr.Status == 429 || httpErr.Status >= 500
    }
    return errors.Is(err, context.DeadlineExceeded) || isNetErr(err)
}
```

### 12.3 Context cancellation cho SSE stream

SSE stream có thể chạy lâu. Phải đảm bảo `ctx.Cancel()` thực sự ngắt stream:

```go
// CtxBody: wrap response body, đóng khi ctx done
type CtxBody struct {
    body io.ReadCloser
    done chan struct{}
    once sync.Once
}

func NewCtxBody(ctx context.Context, body io.ReadCloser) *CtxBody {
    cb := &CtxBody{body: body, done: make(chan struct{})}
    go func() {
        select {
        case <-ctx.Done():
            cb.Close() // đóng socket → Scan() unblocks với err
        case <-cb.done:
        }
    }()
    return cb
}

func (cb *CtxBody) Read(p []byte) (int, error) { return cb.body.Read(p) }

func (cb *CtxBody) Close() error {
    var err error
    cb.once.Do(func() {
        close(cb.done)
        err = cb.body.Close()
    })
    return err
}
```

### 12.4 Logging & Observability

Log những event này:
- `oauth.login_started`, `oauth.login_completed`, `oauth.login_failed`
- `oauth.token_refreshed`, `oauth.refresh_failed`
- `oauth.callback.state_mismatch` (security)
- `chat.request` (model, prompt_tokens, completion_tokens)
- `chat.error` (status_code, error_message)
- `quota.fetched` (used_percent, remaining)
- `quota.exhausted` (cảnh báo)

**Đừng log:** access_token, refresh_token, full prompt content.

### 12.5 Rate limiting tự bảo vệ

Ngay cả khi OpenAI cho phép, giới hạn tốc độ request từ phía bạn để tránh user spam:

```go
import "golang.org/x/time/rate"

limiter := rate.NewLimiter(rate.Every(time.Second), 10) // 10 req/s burst, 1 req/s sustained

func handleChat(...) {
    if err := limiter.Wait(ctx); err != nil { return err }
    // ... gọi Codex
}
```

### 12.6 Single-flight cho concurrent refresh

Nếu 100 request đồng thời vào lúc token expire → 100 refresh call. Dùng mutex hoặc `singleflight`:

```go
import "golang.org/x/sync/singleflight"

var sg singleflight.Group

func (ts *DBTokenSource) refresh() error {
    _, err, _ := sg.Do(ts.providerName, func() (any, error) {
        // actual refresh logic
        return nil, doRefresh()
    })
    return err
}
```

(Trong code GoClaw đang dùng mutex `ts.mu` — cũng OK vì mutex serialize tất cả.)

### 12.7 Bảo mật

- **Always use HTTPS** trừ localhost (OAuth chấp nhận http cho localhost)
- **Validate state strictly** — không match → fail (CSRF protection)
- **Encrypt tokens at rest** — AES-256-GCM hoặc OS keyring
- **Rotate master encryption key** — periodic re-encrypt
- **Audit log:** ai login/logout, IP, timestamp
- **Admin-only access** — chỉ admin role được manage OAuth providers
- **Tenant isolation:** mỗi tenant chỉ thấy providers của mình
- **Refresh token một lần:** nếu OpenAI rotate, lưu ngay token mới — token cũ bị invalid

---

## 13. Reference Code — Standalone Go

File complete copy-paste được, không phụ thuộc GoClaw. ~400 LOC.

### 13.1 `oauth.go` — Tất cả OAuth logic

```go
// Package chatgpt implements OAuth login + Codex API client for ChatGPT subscription.
package chatgpt

import (
    "bytes"
    "context"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "html"
    "io"
    "net"
    "net/http"
    "net/url"
    "os/exec"
    "runtime"
    "strings"
    "sync"
    "time"
)

const (
    AuthURL     = "https://auth.openai.com/oauth/authorize"
    TokenURL    = "https://auth.openai.com/oauth/token"
    ClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
    Scopes      = "openid profile email offline_access api.connectors.read api.connectors.invoke"
    RedirectURI = "http://localhost:1455/auth/callback"
    CallbackPort = "1455"

    APIBase     = "https://chatgpt.com/backend-api"
    ChatPath    = "/codex/responses"
    QuotaPath   = "/wham/usage"
    UserAgent   = "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal"

    RefreshMargin = 5 * time.Minute
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type TokenResponse struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
    IDToken      string `json:"id_token,omitempty"`
    Scope        string `json:"scope"`
}

type StoredToken struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt    time.Time
    AccountID    string
    PlanType     string
}

type PendingLogin struct {
    AuthURL  string
    codeCh   chan string
    errCh    chan error
    verifier string
    state    string
    srv      *http.Server
}

func generatePKCE() (verifier, challenge string, err error) {
    buf := make([]byte, 64)
    if _, err := rand.Read(buf); err != nil { return "", "", err }
    verifier = base64.RawURLEncoding.EncodeToString(buf)
    h := sha256.Sum256([]byte(verifier))
    challenge = base64.RawURLEncoding.EncodeToString(h[:])
    return
}

func generateState() string {
    buf := make([]byte, 16)
    rand.Read(buf)
    return base64.RawURLEncoding.EncodeToString(buf)
}

func StartLogin() (*PendingLogin, error) {
    verifier, challenge, err := generatePKCE()
    if err != nil { return nil, err }
    state := generateState()

    params := url.Values{
        "client_id":                  {ClientID},
        "redirect_uri":               {RedirectURI},
        "response_type":              {"code"},
        "scope":                      {Scopes},
        "code_challenge":             {challenge},
        "code_challenge_method":      {"S256"},
        "state":                      {state},
        "codex_cli_simplified_flow":  {"true"},
        "id_token_add_organizations": {"true"},
        "originator":                 {"pi"},
    }
    authURL := AuthURL + "?" + params.Encode()

    listener, err := net.Listen("tcp", "127.0.0.1:"+CallbackPort)
    if err != nil {
        return nil, fmt.Errorf("listen :%s: %w", CallbackPort, err)
    }

    codeCh := make(chan string, 1)
    errCh := make(chan error, 1)
    var once sync.Once

    mux := http.NewServeMux()
    mux.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
        once.Do(func() {
            if r.URL.Query().Get("state") != state {
                w.Header().Set("Content-Type", "text/html")
                fmt.Fprint(w, `<html><body><h2>Failed</h2><p>State mismatch.</p></body></html>`)
                errCh <- fmt.Errorf("state mismatch")
                return
            }
            code := r.URL.Query().Get("code")
            if code == "" {
                msg := r.URL.Query().Get("error")
                w.Header().Set("Content-Type", "text/html")
                fmt.Fprintf(w, `<html><body><h2>Failed</h2><p>%s</p></body></html>`, html.EscapeString(msg))
                errCh <- fmt.Errorf("oauth error: %s", msg)
                return
            }
            w.Header().Set("Content-Type", "text/html")
            fmt.Fprint(w, `<html><body><h2>Success!</h2><p>You can close this window.</p></body></html>`)
            codeCh <- code
        })
    })

    srv := &http.Server{Handler: mux}
    go func() {
        if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
            errCh <- err
        }
    }()

    return &PendingLogin{
        AuthURL: authURL, codeCh: codeCh, errCh: errCh,
        verifier: verifier, state: state, srv: srv,
    }, nil
}

func (p *PendingLogin) Wait(ctx context.Context) (*TokenResponse, error) {
    defer p.srv.Shutdown(context.Background())
    select {
    case code := <-p.codeCh:
        return ExchangeCode(code, p.verifier)
    case err := <-p.errCh:
        return nil, err
    case <-ctx.Done():
        return nil, fmt.Errorf("timeout: %w", ctx.Err())
    }
}

func (p *PendingLogin) Shutdown() { p.srv.Shutdown(context.Background()) }

func (p *PendingLogin) ExchangeRedirectURL(redirectURL string) (*TokenResponse, error) {
    u, err := url.Parse(redirectURL)
    if err != nil { return nil, fmt.Errorf("invalid url: %w", err) }
    code := u.Query().Get("code")
    if code == "" {
        if msg := u.Query().Get("error"); msg != "" {
            return nil, fmt.Errorf("oauth error: %s", msg)
        }
        return nil, errors.New("no code in url")
    }
    if u.Query().Get("state") != p.state {
        return nil, errors.New("state mismatch")
    }
    return ExchangeCode(code, p.verifier)
}

func ExchangeCode(code, verifier string) (*TokenResponse, error) {
    data := url.Values{
        "grant_type":    {"authorization_code"},
        "client_id":     {ClientID},
        "code":          {code},
        "redirect_uri":  {RedirectURI},
        "code_verifier": {verifier},
    }
    return postTokenForm(data)
}

func RefreshToken(refreshToken string) (*TokenResponse, error) {
    data := url.Values{
        "grant_type":    {"refresh_token"},
        "client_id":     {ClientID},
        "refresh_token": {refreshToken},
    }
    return postTokenForm(data)
}

func postTokenForm(data url.Values) (*TokenResponse, error) {
    resp, err := httpClient.PostForm(TokenURL, data)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, string(body))
    }
    var t TokenResponse
    if err := json.Unmarshal(body, &t); err != nil {
        return nil, fmt.Errorf("parse: %w", err)
    }
    return &t, nil
}

func LoginCLI(ctx context.Context) (*TokenResponse, error) {
    p, err := StartLogin()
    if err != nil { return nil, err }
    fmt.Printf("Open in browser:\n%s\n", p.AuthURL)
    openBrowser(p.AuthURL)
    return p.Wait(ctx)
}

func openBrowser(url string) {
    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "darwin":
        cmd = exec.Command("open", url)
    case "windows":
        cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
    default:
        for _, o := range []string{"xdg-open", "sensible-browser", "x-www-browser"} {
            if path, err := exec.LookPath(o); err == nil {
                cmd = exec.Command(path, url); break
            }
        }
    }
    if cmd != nil { _ = cmd.Start() }
}

// --- JWT parsing ---

type Metadata struct {
    AccountID string
    PlanType  string
}

func ParseJWTMetadata(token string) (Metadata, bool) {
    token = strings.TrimSpace(token)
    parts := strings.Split(token, ".")
    if len(parts) < 2 { return Metadata{}, false }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil { return Metadata{}, false }

    var c struct {
        Auth      *struct {
            AccountID string `json:"chatgpt_account_id"`
            PlanType  string `json:"chatgpt_plan_type"`
        } `json:"https://api.openai.com/auth"`
        AccountID string `json:"https://api.openai.com/auth.chatgpt_account_id"`
        PlanType  string `json:"https://api.openai.com/auth.chatgpt_plan_type"`
    }
    if err := json.Unmarshal(payload, &c); err != nil { return Metadata{}, false }

    m := Metadata{
        AccountID: firstNonEmpty(c.AccountID, nestedField(c.Auth, "AccountID")),
        PlanType:  firstNonEmpty(c.PlanType, nestedField(c.Auth, "PlanType")),
    }
    if m.AccountID == "" && m.PlanType == "" { return Metadata{}, false }
    return m, true
}

func nestedField(a *struct {
    AccountID string `json:"chatgpt_account_id"`
    PlanType  string `json:"chatgpt_plan_type"`
}, field string) string {
    if a == nil { return "" }
    switch field {
    case "AccountID": return a.AccountID
    case "PlanType":  return a.PlanType
    }
    return ""
}

func firstNonEmpty(vs ...string) string {
    for _, v := range vs {
        if v = strings.TrimSpace(v); v != "" { return v }
    }
    return ""
}

// --- TokenStore interface ---

type TokenStore interface {
    Load(ctx context.Context, name string) (*StoredToken, error)
    Save(ctx context.Context, name string, t *StoredToken) error
    Delete(ctx context.Context, name string) error
}

// --- TokenSource: cache + auto-refresh ---

type TokenSource struct {
    store TokenStore
    name  string
    mu    sync.Mutex
    cache *StoredToken
}

func NewTokenSource(store TokenStore, name string) *TokenSource {
    return &TokenSource{store: store, name: name}
}

func (ts *TokenSource) AccessToken(ctx context.Context) (string, error) {
    ts.mu.Lock()
    defer ts.mu.Unlock()

    if ts.cache == nil {
        t, err := ts.store.Load(ctx, ts.name)
        if err != nil { return "", err }
        ts.cache = t
    }

    if time.Until(ts.cache.ExpiresAt) > RefreshMargin {
        return ts.cache.AccessToken, nil
    }

    // Refresh
    newToken, err := RefreshToken(ts.cache.RefreshToken)
    if err != nil {
        // Grace: return cached if still valid for a bit
        if time.Until(ts.cache.ExpiresAt) > 0 {
            return ts.cache.AccessToken, nil
        }
        return "", err
    }

    ts.cache.AccessToken = newToken.AccessToken
    ts.cache.ExpiresAt = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)
    if newToken.RefreshToken != "" {
        ts.cache.RefreshToken = newToken.RefreshToken
    }
    // Backfill metadata if missing
    if ts.cache.AccountID == "" {
        if m, ok := ParseJWTMetadata(firstNonEmpty(newToken.IDToken, newToken.AccessToken)); ok {
            ts.cache.AccountID = m.AccountID
            ts.cache.PlanType = m.PlanType
        }
    }
    _ = ts.store.Save(ctx, ts.name, ts.cache)
    return ts.cache.AccessToken, nil
}

func (ts *TokenSource) Metadata(ctx context.Context) (Metadata, error) {
    ts.mu.Lock()
    defer ts.mu.Unlock()
    if ts.cache == nil {
        t, err := ts.store.Load(ctx, ts.name)
        if err != nil { return Metadata{}, err }
        ts.cache = t
    }
    return Metadata{AccountID: ts.cache.AccountID, PlanType: ts.cache.PlanType}, nil
}

func SaveLoginResult(ctx context.Context, store TokenStore, name string, t *TokenResponse) error {
    stored := &StoredToken{
        AccessToken:  t.AccessToken,
        RefreshToken: t.RefreshToken,
        ExpiresAt:    time.Now().Add(time.Duration(t.ExpiresIn) * time.Second),
    }
    if m, ok := ParseJWTMetadata(firstNonEmpty(t.IDToken, t.AccessToken)); ok {
        stored.AccountID = m.AccountID
        stored.PlanType = m.PlanType
    }
    return store.Save(ctx, name, stored)
}
```

### 13.2 `client.go` — Codex Responses API client

```go
package chatgpt

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "regexp"
    "strings"
    "sync"
    "time"
)

type Client struct {
    apiBase     string
    tokenSource *TokenSource
    httpClient  *http.Client
}

func NewClient(ts *TokenSource) *Client {
    return &Client{
        apiBase:     APIBase,
        tokenSource: ts,
        httpClient:  &http.Client{}, // no timeout for streaming
    }
}

type Message struct {
    Role       string       `json:"role"`
    Content    string       `json:"content,omitempty"`
    Images     []ImageInput `json:"-"`
    ToolCalls  []ToolCall   `json:"-"`
    ToolCallID string       `json:"-"`
}

type ImageInput struct {
    MimeType string
    Data     string // base64
}

type ToolCall struct {
    ID        string
    Name      string
    Arguments map[string]any
}

type ToolDef struct {
    Type        string         // "function" | "image_generation"
    Name        string
    Description string
    Parameters  map[string]any
}

type ChatRequest struct {
    Model    string
    Messages []Message
    Tools    []ToolDef
    Thinking string // "off" | "minimal" | "low" | "medium" | "high"
}

type StreamChunk struct {
    Content  string
    Thinking string
    Images   []ImageOutput
    Done     bool
}

type ImageOutput struct {
    MimeType string
    Data     string
    Partial  bool
}

type ChatResponse struct {
    Content      string
    Thinking     string
    ToolCalls    []ToolCall
    Images       []ImageOutput
    Usage        *Usage
    FinishReason string
}

type Usage struct {
    PromptTokens, CompletionTokens, TotalTokens, ThinkingTokens int
}

var fcRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func toFcID(id string) string {
    for _, p := range []string{"tool_", "call_", "fc_"} {
        if strings.HasPrefix(id, p) { id = id[len(p):]; break }
    }
    return "fc_" + fcRegex.ReplaceAllString(id, "_")
}

func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    return c.ChatStream(ctx, req, nil)
}

func (c *Client) ChatStream(ctx context.Context, req ChatRequest, onChunk func(StreamChunk)) (*ChatResponse, error) {
    body := c.buildBody(req, true)
    data, _ := json.Marshal(body)

    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.apiBase+ChatPath, bytes.NewReader(data))
    if err != nil { return nil, err }

    token, err := c.tokenSource.AccessToken(ctx)
    if err != nil { return nil, err }

    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("Authorization", "Bearer "+token)
    httpReq.Header.Set("OpenAI-Beta", "responses=v1")

    resp, err := c.httpClient.Do(httpReq)
    if err != nil { return nil, err }

    if resp.StatusCode != 200 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
        resp.Body.Close()
        return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
    }

    return c.parseStream(ctx, resp.Body, onChunk)
}

func (c *Client) buildBody(req ChatRequest, stream bool) map[string]any {
    var instructions string
    var input []any

    for _, m := range req.Messages {
        switch m.Role {
        case "system":
            if instructions == "" { instructions = m.Content } else { instructions += "\n\n" + m.Content }
        case "user":
            if len(m.Images) > 0 {
                var parts []map[string]any
                for _, img := range m.Images {
                    parts = append(parts, map[string]any{
                        "type": "input_image",
                        "image_url": fmt.Sprintf("data:%s;base64,%s", img.MimeType, img.Data),
                    })
                }
                if m.Content != "" {
                    parts = append(parts, map[string]any{"type": "input_text", "text": m.Content})
                }
                input = append(input, map[string]any{"role": "user", "content": parts})
            } else {
                input = append(input, map[string]any{"role": "user", "content": m.Content})
            }
        case "assistant":
            for _, tc := range m.ToolCalls {
                args, _ := json.Marshal(tc.Arguments)
                id := toFcID(tc.ID)
                input = append(input, map[string]any{
                    "type": "function_call", "id": id, "call_id": id,
                    "name": tc.Name, "arguments": string(args),
                })
            }
            if m.Content != "" {
                input = append(input, map[string]any{
                    "type": "message", "role": "assistant",
                    "content": []map[string]any{{"type": "output_text", "text": m.Content}},
                })
            }
        case "tool":
            input = append(input, map[string]any{
                "type": "function_call_output",
                "call_id": toFcID(m.ToolCallID),
                "output": m.Content,
            })
        }
    }

    if instructions == "" { instructions = "You are a helpful assistant." }

    body := map[string]any{
        "model": req.Model, "input": input, "stream": stream, "store": false,
        "instructions": instructions,
    }

    if len(req.Tools) > 0 {
        var tools []map[string]any
        for _, t := range req.Tools {
            if t.Type == "image_generation" {
                tools = append(tools, map[string]any{
                    "type": "image_generation", "action": "generate",
                    "model": "gpt-image-2", "output_format": "png", "partial_images": 1,
                })
            } else {
                tools = append(tools, map[string]any{
                    "type": "function", "name": t.Name,
                    "description": t.Description, "parameters": t.Parameters,
                })
            }
        }
        body["tools"] = tools
    }
    if req.Thinking != "" && req.Thinking != "off" {
        body["reasoning"] = map[string]any{"effort": req.Thinking}
    }
    return body
}

type sseEvent struct {
    Type              string          `json:"type"`
    Delta             string          `json:"delta,omitempty"`
    Text              string          `json:"text,omitempty"`
    ItemID            string          `json:"item_id,omitempty"`
    Item              *sseItem        `json:"item,omitempty"`
    Response          *sseResponse    `json:"response,omitempty"`
    OutputFormat      string          `json:"output_format,omitempty"`
    PartialImageB64   string          `json:"partial_image_b64,omitempty"`
}

type sseItem struct {
    ID           string       `json:"id"`
    Type         string       `json:"type"`
    Role         string       `json:"role,omitempty"`
    Content      []sseContent `json:"content,omitempty"`
    CallID       string       `json:"call_id,omitempty"`
    Name         string       `json:"name,omitempty"`
    Arguments    string       `json:"arguments,omitempty"`
    Summary      []sseSummary `json:"summary,omitempty"`
    OutputFormat string       `json:"output_format,omitempty"`
    Result       string       `json:"result,omitempty"`
}

type sseContent struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type sseSummary struct {
    Type string `json:"type"`
    Text string `json:"text"`
}

type sseResponse struct {
    Status string    `json:"status"`
    Output []sseItem `json:"output"`
    Usage  *sseUsage `json:"usage,omitempty"`
    Error  *sseError `json:"error,omitempty"`
}

type sseUsage struct {
    InputTokens         int `json:"input_tokens"`
    OutputTokens        int `json:"output_tokens"`
    TotalTokens         int `json:"total_tokens"`
    OutputTokensDetails *struct {
        ReasoningTokens int `json:"reasoning_tokens"`
    } `json:"output_tokens_details,omitempty"`
}

type sseError struct {
    Code, Message string
}

type toolAcc struct{ callID, name, rawArgs string }

func (c *Client) parseStream(ctx context.Context, body io.ReadCloser, onChunk func(StreamChunk)) (*ChatResponse, error) {
    cb := newCtxBody(ctx, body)
    defer cb.Close()

    result := &ChatResponse{FinishReason: "stop"}
    toolCalls := map[string]*toolAcc{}

    sc := bufio.NewScanner(cb)
    sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

    for sc.Scan() {
        line := sc.Text()
        var payload string
        if after, ok := strings.CutPrefix(line, "data: "); ok { payload = after
        } else if after, ok := strings.CutPrefix(line, "data:"); ok { payload = after
        } else { continue }
        if payload == "[DONE]" { break }

        var e sseEvent
        if err := json.Unmarshal([]byte(payload), &e); err != nil { continue }
        if err := processEvent(&e, result, toolCalls, onChunk); err != nil { return nil, err }
    }
    if err := sc.Err(); err != nil { return nil, err }

    // Finalize tool calls
    for _, acc := range toolCalls {
        if acc.name == "" { continue }
        args := map[string]any{}
        if acc.rawArgs != "" { json.Unmarshal([]byte(acc.rawArgs), &args) }
        result.ToolCalls = append(result.ToolCalls, ToolCall{
            ID: acc.callID, Name: acc.name, Arguments: args,
        })
    }
    if len(result.ToolCalls) > 0 && result.FinishReason != "length" {
        result.FinishReason = "tool_calls"
    }
    if onChunk != nil { onChunk(StreamChunk{Done: true}) }
    return result, nil
}

func processEvent(e *sseEvent, result *ChatResponse, toolCalls map[string]*toolAcc, onChunk func(StreamChunk)) error {
    switch e.Type {
    case "response.output_text.delta":
        result.Content += e.Delta
        if onChunk != nil { onChunk(StreamChunk{Content: e.Delta}) }
    case "response.function_call_arguments.delta":
        if e.ItemID == "" { return nil }
        acc := toolCalls[e.ItemID]
        if acc == nil { acc = &toolAcc{}; toolCalls[e.ItemID] = acc }
        acc.rawArgs += e.Delta
    case "response.output_item.done":
        if e.Item == nil { return nil }
        switch e.Item.Type {
        case "function_call":
            acc := toolCalls[e.Item.ID]
            if acc == nil { acc = &toolAcc{}; toolCalls[e.Item.ID] = acc }
            acc.callID = e.Item.CallID; acc.name = e.Item.Name
            if e.Item.Arguments != "" { acc.rawArgs = e.Item.Arguments }
        case "reasoning":
            for _, s := range e.Item.Summary {
                if s.Text != "" {
                    result.Thinking += s.Text
                    if onChunk != nil { onChunk(StreamChunk{Thinking: s.Text}) }
                }
            }
        case "image_generation_call":
            if e.Item.Result != "" {
                img := ImageOutput{MimeType: mimeFromFormat(e.Item.OutputFormat), Data: e.Item.Result}
                result.Images = append(result.Images, img)
                if onChunk != nil { onChunk(StreamChunk{Images: []ImageOutput{img}}) }
            }
        }
    case "response.image_generation_call.partial_image":
        if onChunk != nil {
            onChunk(StreamChunk{Images: []ImageOutput{{
                MimeType: mimeFromFormat(e.OutputFormat),
                Data: e.PartialImageB64, Partial: true,
            }}})
        }
    case "response.completed", "response.incomplete":
        if e.Response != nil && e.Response.Usage != nil {
            u := e.Response.Usage
            result.Usage = &Usage{
                PromptTokens: u.InputTokens, CompletionTokens: u.OutputTokens, TotalTokens: u.TotalTokens,
            }
            if u.OutputTokensDetails != nil {
                result.Usage.ThinkingTokens = u.OutputTokensDetails.ReasoningTokens
            }
        }
        if e.Response != nil && e.Response.Status == "incomplete" {
            result.FinishReason = "length"
        }
    case "response.failed":
        msg := "response failed"
        if e.Response != nil && e.Response.Error != nil {
            if e.Response.Error.Message != "" { msg = e.Response.Error.Message
            } else { msg = e.Response.Error.Code }
        }
        return errors.New(msg)
    }
    return nil
}

func mimeFromFormat(f string) string {
    switch f {
    case "jpeg": return "image/jpeg"
    case "webp": return "image/webp"
    default:     return "image/png"
    }
}

// --- ctx-aware body ---

type ctxBody struct {
    body io.ReadCloser
    done chan struct{}
    once sync.Once
}

func newCtxBody(ctx context.Context, body io.ReadCloser) *ctxBody {
    cb := &ctxBody{body: body, done: make(chan struct{})}
    go func() {
        select {
        case <-ctx.Done(): cb.Close()
        case <-cb.done:
        }
    }()
    return cb
}

func (cb *ctxBody) Read(p []byte) (int, error) { return cb.body.Read(p) }
func (cb *ctxBody) Close() error {
    var err error
    cb.once.Do(func() { close(cb.done); err = cb.body.Close() })
    return err
}

// --- Quota ---

type QuotaResult struct {
    PlanType    string
    Windows     []QuotaWindow
    Success     bool
    Error       string
    NeedsReauth bool
}

type QuotaWindow struct {
    Label              string
    UsedPercent        int
    RemainingPercent   int
    ResetAfterSeconds  int
}

func (c *Client) FetchQuota(ctx context.Context) (*QuotaResult, error) {
    token, err := c.tokenSource.AccessToken(ctx)
    if err != nil { return &QuotaResult{Error: err.Error(), NeedsReauth: true}, nil }
    meta, _ := c.tokenSource.Metadata(ctx)
    if meta.AccountID == "" {
        return &QuotaResult{Error: "missing account_id"}, nil
    }

    req, _ := http.NewRequestWithContext(ctx, "GET", c.apiBase+QuotaPath, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    req.Header.Set("ChatGPT-Account-Id", meta.AccountID)
    req.Header.Set("User-Agent", UserAgent)

    client := &http.Client{Timeout: 12 * time.Second}
    resp, err := client.Do(req)
    if err != nil { return nil, err }
    defer resp.Body.Close()

    if resp.StatusCode == 401 {
        return &QuotaResult{Error: "reauth required", NeedsReauth: true}, nil
    }
    if resp.StatusCode != 200 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return &QuotaResult{Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(b))}, nil
    }

    var raw struct {
        PlanType  string `json:"plan_type"`
        RateLimit *struct {
            PrimaryWindow   *quotaWin `json:"primary_window"`
            SecondaryWindow *quotaWin `json:"secondary_window"`
        } `json:"rate_limit"`
    }
    json.NewDecoder(resp.Body).Decode(&raw)

    r := &QuotaResult{Success: true, PlanType: raw.PlanType}
    if raw.RateLimit != nil {
        if raw.RateLimit.PrimaryWindow != nil {
            r.Windows = append(r.Windows, raw.RateLimit.PrimaryWindow.toWindow("Primary"))
        }
        if raw.RateLimit.SecondaryWindow != nil {
            r.Windows = append(r.Windows, raw.RateLimit.SecondaryWindow.toWindow("Secondary"))
        }
    }
    return r, nil
}

type quotaWin struct {
    UsedPercent       *float64 `json:"used_percent"`
    ResetAfterSeconds *int     `json:"reset_after_seconds"`
}

func (q *quotaWin) toWindow(label string) QuotaWindow {
    used := 0.0
    if q.UsedPercent != nil { used = *q.UsedPercent }
    if used < 0 { used = 0 }; if used > 100 { used = 100 }
    reset := 0
    if q.ResetAfterSeconds != nil { reset = *q.ResetAfterSeconds }
    return QuotaWindow{
        Label: label, UsedPercent: int(used + 0.5),
        RemainingPercent: 100 - int(used+0.5), ResetAfterSeconds: reset,
    }
}
```

### 13.3 Ví dụ sử dụng

```go
package main

import (
    "context"
    "fmt"
    "log"

    "yourapp/chatgpt"
)

type FileStore struct{ /* implement chatgpt.TokenStore với JSON file */ }

func main() {
    ctx := context.Background()
    store := &FileStore{}

    // First login
    tokenResp, err := chatgpt.LoginCLI(ctx)
    if err != nil { log.Fatal(err) }
    chatgpt.SaveLoginResult(ctx, store, "default", tokenResp)

    // Use it
    ts := chatgpt.NewTokenSource(store, "default")
    client := chatgpt.NewClient(ts)

    resp, err := client.ChatStream(ctx, chatgpt.ChatRequest{
        Model: "gpt-5.4",
        Messages: []chatgpt.Message{
            {Role: "user", Content: "Hello!"},
        },
    }, func(chunk chatgpt.StreamChunk) {
        fmt.Print(chunk.Content)
    })
    if err != nil { log.Fatal(err) }
    fmt.Printf("\n[%d/%d tokens]\n", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
}
```

---

## 14. Port sang Node.js / Python

### 14.1 Node.js (TypeScript)

```typescript
import * as crypto from "crypto";
import * as http from "http";
import { URL, URLSearchParams } from "url";
import fetch from "node-fetch";

const CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann";
const REDIRECT = "http://localhost:1455/auth/callback";
const SCOPES = "openid profile email offline_access api.connectors.read api.connectors.invoke";

function generatePKCE() {
  const verifier = crypto.randomBytes(64).toString("base64url");
  const challenge = crypto.createHash("sha256").update(verifier).digest("base64url");
  return { verifier, challenge };
}

export async function login(): Promise<TokenResponse> {
  const { verifier, challenge } = generatePKCE();
  const state = crypto.randomBytes(16).toString("base64url");

  const params = new URLSearchParams({
    client_id: CLIENT_ID,
    redirect_uri: REDIRECT,
    response_type: "code",
    scope: SCOPES,
    code_challenge: challenge,
    code_challenge_method: "S256",
    state,
    codex_cli_simplified_flow: "true",
    id_token_add_organizations: "true",
    originator: "pi",
  });
  const authURL = `https://auth.openai.com/oauth/authorize?${params}`;

  return new Promise((resolve, reject) => {
    const server = http.createServer(async (req, res) => {
      const url = new URL(req.url!, "http://localhost");
      if (url.pathname !== "/auth/callback") { res.writeHead(404).end(); return; }

      const code = url.searchParams.get("code");
      const gotState = url.searchParams.get("state");
      if (gotState !== state) {
        res.writeHead(400).end("State mismatch");
        server.close();
        reject(new Error("state mismatch"));
        return;
      }
      if (!code) {
        res.writeHead(400).end("No code");
        server.close();
        reject(new Error("no code"));
        return;
      }
      res.writeHead(200, { "Content-Type": "text/html" })
         .end("<h2>Success!</h2><p>You can close this window.</p>");

      try {
        const token = await exchangeCode(code, verifier);
        resolve(token);
      } catch (e) { reject(e); }
      finally { server.close(); }
    });
    server.listen(1455, "127.0.0.1");

    console.log(`Open in browser:\n${authURL}`);
    // Optional: spawn `open` (mac) / `xdg-open` (linux) / `start` (windows)
  });
}

async function exchangeCode(code: string, verifier: string): Promise<TokenResponse> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: CLIENT_ID,
    code, redirect_uri: REDIRECT, code_verifier: verifier,
  });
  const res = await fetch("https://auth.openai.com/oauth/token", { method: "POST", body });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json() as Promise<TokenResponse>;
}

export async function refreshToken(rt: string): Promise<TokenResponse> {
  const body = new URLSearchParams({
    grant_type: "refresh_token", client_id: CLIENT_ID, refresh_token: rt,
  });
  const res = await fetch("https://auth.openai.com/oauth/token", { method: "POST", body });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);
  return res.json() as Promise<TokenResponse>;
}

// Streaming chat
export async function* chatStream(token: string, req: ChatRequest): AsyncGenerator<StreamChunk> {
  const body = buildBody(req, true);
  const res = await fetch("https://chatgpt.com/backend-api/codex/responses", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${token}`,
      "OpenAI-Beta": "responses=v1",
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}: ${await res.text()}`);

  const reader = res.body!;
  let buffer = "";
  for await (const chunk of reader) {
    buffer += chunk.toString();
    const lines = buffer.split("\n");
    buffer = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      const payload = line.slice(6);
      if (payload === "[DONE]") return;
      try {
        const event = JSON.parse(payload);
        // process event → yield chunks
        if (event.type === "response.output_text.delta") {
          yield { content: event.delta };
        }
        // ... handle other event types
      } catch {}
    }
  }
}

interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  id_token?: string;
  scope: string;
}

interface ChatRequest {
  model: string;
  messages: Array<{ role: string; content: string }>;
}

interface StreamChunk {
  content?: string;
  thinking?: string;
}

function buildBody(req: ChatRequest, stream: boolean) {
  // Tương tự buildCodexRequestBody của Go
  let instructions = "You are a helpful assistant.";
  const input: any[] = [];
  for (const m of req.messages) {
    if (m.role === "system") instructions = m.content;
    else input.push({ role: m.role, content: m.content });
  }
  return { model: req.model, input, stream, store: false, instructions };
}
```

### 14.2 Python

```python
import base64
import hashlib
import http.server
import json
import secrets
import threading
import urllib.parse
import requests
import sseclient  # pip install sseclient-py

CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
REDIRECT = "http://localhost:1455/auth/callback"
SCOPES = "openid profile email offline_access api.connectors.read api.connectors.invoke"
AUTH_URL = "https://auth.openai.com/oauth/authorize"
TOKEN_URL = "https://auth.openai.com/oauth/token"
API_BASE = "https://chatgpt.com/backend-api"


def generate_pkce():
    verifier = base64.urlsafe_b64encode(secrets.token_bytes(64)).rstrip(b"=").decode()
    challenge = base64.urlsafe_b64encode(
        hashlib.sha256(verifier.encode()).digest()
    ).rstrip(b"=").decode()
    return verifier, challenge


def login():
    verifier, challenge = generate_pkce()
    state = base64.urlsafe_b64encode(secrets.token_bytes(16)).rstrip(b"=").decode()

    params = {
        "client_id": CLIENT_ID, "redirect_uri": REDIRECT, "response_type": "code",
        "scope": SCOPES, "code_challenge": challenge, "code_challenge_method": "S256",
        "state": state, "codex_cli_simplified_flow": "true",
        "id_token_add_organizations": "true", "originator": "pi",
    }
    auth_url = f"{AUTH_URL}?{urllib.parse.urlencode(params)}"
    print(f"Open in browser:\n{auth_url}")

    received = {}
    event = threading.Event()

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            url = urllib.parse.urlparse(self.path)
            q = urllib.parse.parse_qs(url.query)
            if q.get("state", [None])[0] != state:
                self.send_response(400); self.end_headers()
                received["error"] = "state mismatch"; event.set(); return
            code = q.get("code", [None])[0]
            if not code:
                self.send_response(400); self.end_headers()
                received["error"] = "no code"; event.set(); return
            self.send_response(200)
            self.send_header("Content-Type", "text/html")
            self.end_headers()
            self.wfile.write(b"<h2>Success!</h2><p>You can close this window.</p>")
            received["code"] = code
            event.set()

        def log_message(self, *a, **kw): pass

    httpd = http.server.HTTPServer(("127.0.0.1", 1455), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True).start()
    event.wait(timeout=360)  # 6 min
    httpd.shutdown()

    if "error" in received:
        raise Exception(received["error"])
    return exchange_code(received["code"], verifier)


def exchange_code(code, verifier):
    r = requests.post(TOKEN_URL, data={
        "grant_type": "authorization_code",
        "client_id": CLIENT_ID, "code": code,
        "redirect_uri": REDIRECT, "code_verifier": verifier,
    })
    r.raise_for_status()
    return r.json()


def refresh_token(rt):
    r = requests.post(TOKEN_URL, data={
        "grant_type": "refresh_token", "client_id": CLIENT_ID, "refresh_token": rt,
    })
    r.raise_for_status()
    return r.json()


def chat_stream(access_token, model, messages, on_chunk=None):
    body = {
        "model": model, "stream": True, "store": False,
        "instructions": "You are a helpful assistant.",
        "input": [{"role": m["role"], "content": m["content"]} for m in messages],
    }
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}",
        "OpenAI-Beta": "responses=v1",
    }
    resp = requests.post(f"{API_BASE}/codex/responses", json=body, headers=headers, stream=True)
    resp.raise_for_status()

    content = ""
    client = sseclient.SSEClient(resp)
    for event in client.events():
        if event.data == "[DONE]": break
        try:
            e = json.loads(event.data)
        except Exception: continue
        if e.get("type") == "response.output_text.delta":
            content += e.get("delta", "")
            if on_chunk: on_chunk(e["delta"])
    return content


if __name__ == "__main__":
    token = login()
    print(f"Got access token expiring in {token['expires_in']}s")
    out = chat_stream(token["access_token"], "gpt-5.4",
                     [{"role": "user", "content": "Hi!"}],
                     on_chunk=lambda c: print(c, end="", flush=True))
    print()
```

---

## 15. Testing & Debugging

### 15.1 Unit tests

- **PKCE generation:** verifier dài 86 chars, challenge = base64url(sha256(verifier))
- **JWT parsing:** test với fixture access_token (lấy từ login thật, expired OK)
- **State validation:** mismatch → reject
- **Token refresh:** mock token endpoint, assert form fields đúng
- **SSE parser:** feed các event fixture, assert state machine đúng

### 15.2 Integration test

```go
func TestEndToEnd(t *testing.T) {
    if os.Getenv("CHATGPT_OAUTH_TEST") != "1" { t.Skip() }

    ctx := context.Background()
    tokenResp, err := chatgpt.LoginCLI(ctx)
    if err != nil { t.Fatal(err) }

    store := &MemStore{}
    chatgpt.SaveLoginResult(ctx, store, "test", tokenResp)

    ts := chatgpt.NewTokenSource(store, "test")
    client := chatgpt.NewClient(ts)

    resp, err := client.Chat(ctx, chatgpt.ChatRequest{
        Model: "gpt-5.4",
        Messages: []chatgpt.Message{{Role: "user", Content: "Say 'hello'"}},
    })
    if err != nil { t.Fatal(err) }
    if !strings.Contains(strings.ToLower(resp.Content), "hello") {
        t.Errorf("expected 'hello', got: %s", resp.Content)
    }
}
```

### 15.3 Debug tips

- **Bật log full request:** dùng `httputil.DumpRequestOut(req, true)` (LƯU Ý xóa Authorization header trước khi log)
- **Capture SSE stream:** lưu raw body vào file `.sse`, replay offline
- **JWT debugging:** copy access_token vào https://jwt.io để verify claims
- **Inspect Codex CLI bằng `mitmproxy`:** chạy Codex CLI thật với HTTPS proxy → xem nó gửi gì
- **Reproduce 401 sớm:** xóa secrets, retry call → confirm refresh flow chạy

### 15.4 Common errors

| Error | Nguyên nhân thường gặp |
|---|---|
| `state mismatch` | User mở 2 tab login cùng lúc; hoặc bookmark URL cũ |
| `port 1455 in use` | Flow trước chưa cleanup; restart app |
| `invalid_grant` (refresh) | Refresh token expired hoặc đã bị rotate (lưu mới chưa) |
| `HTTP 401` (chat) | Token expired; check expiry, refresh logic |
| `HTTP 403` (chat) | Account bị restrict; thường rate limit weekly hết |
| `HTTP 429` | Quota window primary hết; chờ reset |
| Stream cut giữa chừng | Network drop; cần reconnect/retry logic ở caller |
| `state mismatch` ở manual paste | User paste URL của flow cũ |

---

## 16. Pháp lý & Chính sách

### 16.1 Tính hợp pháp kỹ thuật

- Codex CLI là **phần mềm chính thức của OpenAI**, open-source (MIT/Apache). Client_id `app_EMoamEEZ73f0CkXaXp7hrann` là **public credential**, không phải secret bị rò rỉ.
- Sử dụng OAuth flow với client_id công khai là **practice chuẩn** (tương tự GitHub CLI dùng client_id công khai của họ).
- API `chatgpt.com/backend-api` là API mà ChatGPT web app sử dụng — không có private undocumented backdoor.

### 16.2 Điều khoản dịch vụ ChatGPT

OpenAI ToS cho ChatGPT có điều khoản:
- *"You may not access the Services by any automated means except through the APIs that OpenAI makes publicly available."*

→ Vùng xám: Responses API qua subscription **không phải** API công khai (api.openai.com), nhưng cũng không phải scraping (đi qua OAuth + Bearer token hợp lệ). OpenAI:
- **Không công bố** API này cho dev rộng rãi
- **Cho phép** Codex CLI dùng
- **Có thể** thay đổi/rate-limit/cấm bất cứ lúc nào

### 16.3 Khuyến nghị

✅ **Nên:**
- Mạo danh User-Agent của Codex CLI để OpenAI khó phân biệt (kỹ thuật chính)
- Theo dõi Codex CLI repo (https://github.com/openai/codex) để cập nhật khi protocol thay đổi
- Hiển thị disclaimer cho user: "Tính năng này dùng tài khoản ChatGPT của bạn; tuân theo ToS của OpenAI"
- Implement quota check để user thấy hạn mức rõ ràng

⚠️ **Không nên:**
- Bán "ChatGPT API access" qua tài khoản của user khác (rõ ràng vi phạm ToS)
- Build dịch vụ proxy cho phép nhiều user chia nhau 1 account ChatGPT Plus
- Spam OpenAI với hàng nghìn account fake

❌ **Tuyệt đối tránh:**
- Reverse-engineer + bypass rate limit
- Crack/abuse free trial accounts

### 16.4 Risk mitigation

Nếu xây dựng commercial product:
- Đặt feature này là "Bring Your Own ChatGPT Account" (BYOC)
- Để user tự đăng nhập account của họ, không lưu credentials chung
- Có toggle "Use ChatGPT subscription" hoặc "Use API key" để user chọn
- Document rõ trong privacy policy: bạn lưu token tại đâu, mã hóa thế nào

---

## 17. Roadmap mở rộng

Các tính năng nâng cao có thể thêm sau khi MVP chạy:

| Tính năng | Mô tả |
|---|---|
| **Multi-account pool** | Đăng nhập nhiều ChatGPT Plus, round-robin |
| **Quota dashboard** | UI hiển thị 5h/weekly usage với progress bar |
| **Auto-failover** | Account hết quota → tự chuyển sang account khác |
| **Model alias** | `chatgpt/gpt-5.4` prefix để route qua pool này |
| **Image generation** | Wire `image_generation` tool với gpt-image-2 |
| **Thinking mode** | Expose `reasoning.effort` cho user toggle |
| **Code Review** | Sử dụng `code_review_rate_limit` riêng cho task code review |
| **Session memory** | Cache conversation server-side với `store: true` |
| **Cost estimation** | Convert token usage → quota % để hiển thị "đã dùng X%" |
| **Per-user accounts** | Mỗi end-user trong app dùng account riêng |
| **Tenant isolation** | Multi-tenant SaaS: tenant A không thấy tokens tenant B |
| **Token rotation alerts** | Email user khi refresh fail / token sắp expire |
| **Codex CLI compat layer** | Map sang format Chat Completions cho code cũ |

---

## Phụ lục A: Bảng tổng kết hằng số

| Hằng | Giá trị | Bắt buộc đúng? |
|---|---|---|
| `client_id` | `app_EMoamEEZ73f0CkXaXp7hrann` | ✅ Yes |
| `redirect_uri` | `http://localhost:1455/auth/callback` | ✅ Yes (port 1455) |
| `scope` | `openid profile email offline_access api.connectors.read api.connectors.invoke` | ✅ Yes |
| `codex_cli_simplified_flow` | `true` | ✅ Yes |
| `id_token_add_organizations` | `true` | ✅ Yes |
| `originator` | `pi` | ✅ Yes |
| `code_challenge_method` | `S256` | ✅ Yes |
| User-Agent (quota) | `codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal` | ⚠️ Recommended |
| Header `OpenAI-Beta` | `responses=v1` | ✅ Yes |
| Header `ChatGPT-Account-Id` | từ JWT | ✅ Yes (quota only) |

## Phụ lục B: Bảng endpoint tóm tắt

| Method | URL | Purpose |
|---|---|---|
| GET | `https://auth.openai.com/oauth/authorize` | OAuth start (user browser) |
| POST | `https://auth.openai.com/oauth/token` | Exchange code & refresh (server) |
| POST | `https://chatgpt.com/backend-api/codex/responses` | LLM streaming call |
| GET | `https://chatgpt.com/backend-api/wham/usage` | Quota check |

## Phụ lục C: Reference Implementation trong GoClaw

Để tham khảo code thật:

| File | Mô tả |
|---|---|
| [internal/oauth/openai.go](../../internal/oauth/openai.go) | PKCE, callback server, token exchange |
| [internal/oauth/openai_claims.go](../../internal/oauth/openai_claims.go) | JWT parse |
| [internal/oauth/token.go](../../internal/oauth/token.go) | DBTokenSource, cache, refresh |
| [internal/oauth/openai_quota.go](../../internal/oauth/openai_quota.go) | Quota fetcher |
| [internal/oauth/openai_quota_transport.go](../../internal/oauth/openai_quota_transport.go) | Quota HTTP transport |
| [internal/providers/codex.go](../../internal/providers/codex.go) | Codex provider (streaming chat) |
| [internal/providers/codex_build.go](../../internal/providers/codex_build.go) | Request body builder |
| [internal/providers/codex_types.go](../../internal/providers/codex_types.go) | Wire types |
| [internal/providers/codex_stream_state.go](../../internal/providers/codex_stream_state.go) | Stream state machine |
| [internal/providers/sse_reader.go](../../internal/providers/sse_reader.go) | SSE scanner |
| [internal/providers/chatgpt_oauth_router.go](../../internal/providers/chatgpt_oauth_router.go) | Multi-account pool |
| [internal/http/oauth.go](../../internal/http/oauth.go) | REST API handler |
| [ui/web/src/pages/providers/provider-oauth-section.tsx](../../ui/web/src/pages/providers/provider-oauth-section.tsx) | UI component |

---

## Kết luận

Tính năng "ChatGPT Subscription (OAuth)" là sự kết hợp của:
1. **OAuth 2.0 PKCE** chuẩn — mọi ngôn ngữ đều có thư viện
2. **Mạo danh Codex CLI** — copy đúng client_id + 3 magic query params + User-Agent
3. **Responses API** — không phải Chat Completions, request body khác hẳn
4. **SSE streaming** — parse events theo state machine
5. **Token lifecycle** — refresh trước expiry, encrypt storage, single-flight

Bạn có thể implement **MVP trong 1-2 ngày** với reference code ở mục 13. Nâng cao (multi-account, quota UI, failover) trong 1 tuần. Bottleneck chính là **OAuth flow UX** và **error handling** — đầu tư thời gian cho 2 phần này.

Khi OpenAI cập nhật Codex CLI (https://github.com/openai/codex), check diff để biết có thay đổi protocol không. Thường client_id giữ nguyên, query params có thể thêm bớt.

**Chúc bạn thành công với việc port tính năng sang phần mềm mới.** Nếu có thắc mắc cụ thể về phần nào (vd. làm sao test mà không cần ChatGPT Plus thật, hoặc gốc rễ một edge case), hãy hỏi tiếp.
