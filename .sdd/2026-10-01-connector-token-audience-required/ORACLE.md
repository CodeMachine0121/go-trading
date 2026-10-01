# Oracle — 外掛授權必須指明對象服務

每一列的「預期」只從 PRD 推導，寫於實作之前。登記地址 `http://localhost:33418/callback`、回執記號 `s1`。
`R` = `https://trading-mcp.coding-afternoon.com/mcp`。

## A · 請求授權（`ConnectorAuthorizationStartDomain` / `StartConnectorAuthorization`）

| # | 對象服務 | 預期 |
| :-- | :--- | :--- |
| A1 | `R` | 不拒絕；存下待授權請求（Resource=`R`），轉到網頁授權頁 |
| A2 | `http://localhost:8787/mcp`、`http://127.0.0.1:8787/mcp`、`http://[::1]:8787/mcp` | 不拒絕 |
| A3 | 空字串 | 送回 `http://localhost:33418/callback?error=invalid_request&error_description=缺少對象服務（resource）&state=s1`；不存請求 |
| A4 | `trading-mcp`、`/mcp`、`http://trading-mcp.example.com/mcp`、`https://trading-mcp.example.com/mcp#part`、`https://someone:secret@example.com/mcp`、`https://trading-mcp.example.com/mcp?x=1`、`ftp://example.com/mcp`、`https:///mcp` | 送回 `error=invalid_target`、`error_description`=`ErrConnectorResourceInvalid` 訊息、`state=s1`；不存請求 |
| A5 | 外掛代號不存在 + 空對象服務 | `ErrConnectorClientNotFound`，無 redirect |
| A6 | 2049 字元的對象服務 | 仍為 `invalid_request`「請求內容過長」 |

| A7 | 修正前記下、沒有對象服務的待授權請求 | 查詢 / 允許 / 拒絕一律 `ErrConnectorAuthorizationRequestNotFound`，不發授權碼 |

## B · 換授權（`ExchangeAuthorizationCode`）

| # | Given | 預期 |
| :-- | :--- | :--- |
| B1 | 授權碼 Resource=`R`、外掛 `client-A` | 簽發 claims：Audience=`R`、ConnectorClientIdentifier=`client-A` |
| B2 | 授權碼 Resource 空 | `ErrConnectorGrantInvalid`；不 Redeem、不簽發 |

## C · 續用（`RenewConnectorSession` / `RenewSession`）

| # | Given | 預期 |
| :-- | :--- | :--- |
| C1 | 外掛登入階段 Audience=`R`、外掛 `client-A` | 簽發 claims：Audience=`R`、ConnectorClientIdentifier=`client-A` |
| C2 | 外掛登入階段 Audience 空、外掛 `client-A`、以 `client-A` 續用 | `ErrAuthenticationRequired`（controller → 400 `invalid_grant`）；不 Rotate、不 RevokeChain |
| C3 | 網頁登入階段續用 | 簽發 claims：Audience 空、ConnectorClientIdentifier 空（照舊） |

## D · 網頁專屬（`IdentifyActivatedWebUser`）

| # | 憑證 claims | 預期 |
| :-- | :--- | :--- |
| D1 | Audience 空、ConnectorClientIdentifier 空 | 通過，回使用者 |
| D2 | Audience=`R` | `ErrAuthenticationRequired`（401） |
| D3 | Audience 空、ConnectorClientIdentifier=`client-A` | `ErrAuthenticationRequired`（401） |

## E · 憑證簽章（`JwtAccessTokenProxy`）

| # | Given | 預期 |
| :-- | :--- | :--- |
| E1 | Issue claims ConnectorClientIdentifier=`client-A`、Audience=`R` | ClaimsOf 讀回相同兩值 |
| E2 | Issue 網頁 claims | ClaimsOf 讀回兩者皆空；token payload 不含 `client_id` |
