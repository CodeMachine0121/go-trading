# Oracle — 外掛授權

每一列的「預期」**只從 PRD 與跨專案約定推導**，在任何實作存在之前寫下。斷言一律抄這裡。
`T` = 2026-09-27T08:00:00Z。公開網址 `https://trading-api.example.com`，網頁 `https://web.example.com`。
待授權請求 10 分鐘、授權碼 5 分鐘、登入憑證 15 分鐘、續用憑證 30 天。
PKCE 範例（RFC 7636 附錄 B）：verifier `dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk` → challenge `E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM`。

## A · 送回地址與外掛登記（`ConnectorRedirectUriDomain` / `ConnectorClientRegistrationDomain`）

| # | Given | Then |
| :-- | :--- | :--- |
| A1 | `http://localhost:33418/callback` | 登記成功；回 `grant_types`=[authorization_code, refresh_token]、`response_types`=[code]、`token_endpoint_auth_method`=none、`client_id_issued_at`=T 的 unix 秒 |
| A2 | `http://127.0.0.1:1/x`、`http://[::1]:8765/` | 成功 |
| A3 | `https://evil.example.com/cb` | `ErrConnectorRedirectUriInvalid` → 400 `invalid_redirect_uri` |
| A4 | `https://localhost:33418/callback` | `ErrConnectorRedirectUriInvalid` |
| A5 | 空清單 | `ErrConnectorRedirectUriInvalid` |
| A6 | `token_endpoint_auth_method`=`client_secret_basic` | `ErrConnectorClientMetadataInvalid` → 400 `invalid_client_metadata` |
| A7 | `token_endpoint_auth_method`=`none` 明給 | 成功 |
| A8 | `grant_types`=[client_credentials] | `ErrConnectorClientMetadataInvalid` |
| A9 | `response_types`=[token] | `ErrConnectorClientMetadataInvalid` |
| A10 | `http://localhost:1/cb#frag`、`http://user@localhost/cb` | `ErrConnectorRedirectUriInvalid` |
| A11 | 名稱空白 | 名稱記為「未命名外掛」；名稱 129 字 → `ErrConnectorClientMetadataInvalid` |
| A12 | 11 個地址 | `ErrConnectorRedirectUriInvalid` |
| A13 | 本文不是 JSON | 400 `invalid_client_metadata` |

## B · 開始授權（`StartConnectorAuthorization`）

登記地址 `http://localhost:33418/callback`，外掛 `client-A`。

| # | Given | Then |
| :-- | :--- | :--- |
| B1 | 完整請求，地址同登記，state `abc`，resource `https://mcp.example.com/mcp` | 存一筆待授權請求（ExpiresAt T+10m、RedirectUri 原樣、challenge、state、resource）；302 到 `https://web.example.com/connector-authorization?request=<id>` |
| B2 | 地址 `http://localhost:51000/callback` | 照常；待授權請求 RedirectUri = `http://localhost:51000/callback` |
| B3 | 地址 `http://localhost:33418/other` | `ErrConnectorRedirectUriNotRegistered` → 400 `invalid_request`，不 302 |
| B4 | 外掛代號不存在 | `ErrConnectorClientNotFound` → 400 `invalid_client`，不 302 |
| B5 | 缺 code_challenge，state `abc` | 302 到 `http://localhost:33418/callback?error=invalid_request&state=abc`（可附 error_description），不存任何東西 |
| B6 | method `plain` | 302 回外掛 `error=invalid_request` |
| B7 | response_type `token` | 302 回外掛 `error=invalid_request` |
| B8 | 帶 scope `read` | 與 B1 同 |
| B9 | 缺 state 的錯誤送回 | 送回網址不含 state |
| B10 | 地址 `http://LOCALHOST:5/callback` | 視為已登記 |

## C · 待授權請求（查詢 / 允許 / 拒絕）

| # | Given | Then |
| :-- | :--- | :--- |
| C1 | 建於 T−9m，外掛名稱 `Claude Code` | `{"clientName":"Claude Code","expiresAt":"<T+1m RFC3339>"}` |
| C2 | 建於 T−10m | `ErrConnectorAuthorizationRequestNotFound` → 404 |
| C3 | 有效、state `abc`、使用者 7 允許 | 回 `redirectTo` = `<redirect>?code=<mint 值>&state=abc`；存的授權碼：CodeDigest=mint 留存樣、UserID 7、client、原樣地址、challenge、resource、ExpiresAt T+5m |
| C4 | 未登入允許 | 401（既有中介層） |
| C5 | 待開通允許 | 403 附開通指示（既有中介層） |
| C6 | 有效、state `abc`、拒絕 | `redirectTo` = `<redirect>?error=access_denied&state=abc` |
| C7 | 已決定（DecidedAt 有值） | 查詢/允許/拒絕皆 404 |
| C8 | 允許時條件式更新落空（同時被決定） | 404 |
| C9 | 不存在的代號 | 404 |

## D · 換授權 / 續用

| # | Given | Then |
| :-- | :--- | :--- |
| D1 | 授權碼建於 T−4m（ExpiresAt T+1m）、client-A、`http://localhost:51000/callback`、正確 verifier、resource R、使用者 7 | 200 `{"access_token":<jwt>,"token_type":"Bearer","expires_in":900,"refresh_token":<mint>}`；`Cache-Control: no-store`；新登入階段 ChainID=留存樣、ConnectorClientIdentifier=client-A、Audience=R、ExpiresAt T+30d；access token 以 Audience R、UserID 7、ExpiresAt T+15m 簽發 |
| D2 | 授權碼 ExpiresAt = T | `invalid_grant` |
| D3 | verifier 錯 | `invalid_grant` |
| D4 | 地址 `http://localhost:33418/callback`（授權碼是 51000） | `invalid_grant` |
| D5 | client-B（存在）換 client-A 的碼 | `invalid_grant` |
| D6 | 授權碼已兌換（SessionChainID `chain-1`，非空即已兌換） | `invalid_grant`，且 `RevokeChain("chain-1")` 被呼叫 |
| D7 | Redeem 回報已兌換（同時兩次） | `invalid_grant`，重讀後 `RevokeChain(該鏈)` |
| D8 | 缺 code / verifier / redirect_uri / client_id | `invalid_request` |
| D9 | grant_type `password` | `unsupported_grant_type`；缺 grant_type → `invalid_request` |
| D10 | client_id 不存在 | `invalid_client` |
| D11 | 外掛續用，session 的 client=client-A、Audience R | 200，新 access token 以 Audience R 簽發；Rotate 的新 session 帶 client-A 與 R |
| D12 | 已作廢的續用憑證 | `invalid_grant`，RevokeChain |
| D13 | 以 client-B 續用 client-A 的登入 | `invalid_grant`，不 Rotate |
| D14 | 網頁續用路徑續用外掛登入 | 401 請重新登入 |
| D15 | 外掛續用缺 refresh_token 或 client_id | `invalid_request` |
| D16 | 授權碼對應的使用者已不存在 | `invalid_grant` |
| D17 | 未知授權碼 | `invalid_grant` |

## E · 查驗 / 說明書 / 憑證

| # | Given | Then |
| :-- | :--- | :--- |
| E1 | 憑證 claims UserID 7、Audience R、到期 T+15m，使用者在 | `{"active":true,"sub":"7","aud":R,"exp":<unix>}` |
| E2 | 網頁憑證（無 aud） | `{"active":true,"sub":"7","aud":"","exp":...}` |
| E3 | 壞憑證 / 空字串 | `{"active":false}`（只有這一個欄位） |
| E4 | 使用者不存在 | `{"active":false}` |
| E5 | JWT Issue(Audience R) → ClaimsOf | Audience R；Issue(無 Audience) → JWT 無 `aud` claim |
| E6 | 帶 aud 的憑證 → UserIdentifiedBy | 使用者 7（接受） |
| E7 | PUBLIC_BASE_URL `https://trading-api.example.com/` | issuer `https://trading-api.example.com`，四個 endpoint 都在其下；response_types [code]、grant_types [authorization_code, refresh_token]、code_challenge_methods [S256]、auth methods [none] |
| E8 | 未設定 | PUBLIC_BASE_URL `http://localhost:8080`、FRONTEND_BASE_URL `http://localhost:3000` |
