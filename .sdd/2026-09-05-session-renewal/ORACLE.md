# 登入階段的續用與結束 — Oracle（預期結果，先於實作寫下）

每一列的「預期結果」都只從 `PRD.md` 的 AC 場景推導，**不參考任何實作**。
測試的每一個斷言值一律取自本表；若某個值只能靠跑實作才得知，就是本表不完整，回頭補或問。

固定值：現在 = `2026-09-05T08:00:00Z`；登入憑證期限 15 分鐘；續用憑證期限 30 天。
因此登入憑證到期 = `2026-09-05T08:15:00Z`，續用憑證到期 = `2026-10-05T08:00:00Z`。

## SessionDomain — 這一段還算不算數（US-02、US-03、US-04）

| # | 這一段的狀態 | 預期結果 |
|---|---|---|
| D1 | 沒作廢、到期於 `2026-10-05T08:00:00Z` | `Revoked()` = false、`Expired(now)` = false、`Usable(now)` = true |
| D2 | 已作廢、尚未到期 | `Revoked()` = true、`Usable(now)` = false |
| D3 | 沒作廢、到期於 `2026-09-05T07:59:59Z` | `Expired(now)` = true、`Usable(now)` = false |
| D4 | 沒作廢、到期**正好**是 `2026-09-05T08:00:00Z` | `Expired(now)` = **true**——到期時刻是第一個不能用的瞬間 |
| D5 | 沒作廢、到期於 `2026-09-05T08:00:01Z` | `Expired(now)` = false |
| D6 | 已作廢**且**已過期 | `Revoked()` = true、`Expired(now)` = true——兩件事分開問，因為它們導向不同的動作 |
| D7 | `Renewed("新的留存樣", now, 30 天)` | 產出的下一段：`ChainID` **與這一段相同**、`UserID` 相同、`RefreshTokenDigest` = `"新的留存樣"`、`ExpiresAt` = `2026-10-05T08:00:00Z`、`RevokedAt` = nil、`ID` = 0 |
| D8 | 承 D7，換發當下改為 `2026-09-10T08:00:00Z` | `ExpiresAt` = `2026-10-10T08:00:00Z`——**從當下重算，不沿用舊的到期時刻** |

## RandomRefreshTokenProxy — 續用憑證的產生與留存樣（US-01）

| # | 情境 | 預期結果 |
|---|---|---|
| T1 | `Mint()` | `Value` 非空；`Digest` 非空；**`Digest` ≠ `Value`** |
| T2 | `Mint()` 呼叫兩次 | 兩次的 `Value` **不同**（猜不到才有意義） |
| T3 | `Mint()` 的產出 | `DigestOf(Value)` 等於同一次產出的 `Digest` |
| T4 | `DigestOf("x")` 呼叫兩次 | **相同**——它必須查得到，所以不摻隨機料（與密碼證明相反） |
| T5 | `DigestOf("x")` vs `DigestOf("y")` | 不同 |
| T6 | `Digest` 的長度 | 固定 64（SHA-256 的十六進位表示），與輸入長度無關 |
| T7 | `Value` 的長度 | ≥ 43（32 位元組以上的密碼學亂數編碼後的長度） |
| T8 | `DigestOf("")` | 回一段 64 長的字串，不 panic |

## UserService.SignIn — 登入現在會開一段登入階段（US-01）

| # | 情境 | 預期結果 |
|---|---|---|
| S1 | 帳密正確 | 回 `SessionTokensDto`：`AccessToken` 非空、`ExpiresAt` = `08:15:00Z`、`RefreshToken` = `Mint()` 的 `Value`、`RefreshTokenExpiresAt` = `2026-10-05T08:00:00Z` |
| S2 | 同上 | `SessionRepository.Save` 收到的那一段：`UserID` = 該使用者、`RefreshTokenDigest` = `Mint()` 的 `Digest`、`ExpiresAt` = `2026-10-05T08:00:00Z`、`RevokedAt` = nil、`ChainID` **非空**、`ID` = 0 |
| S3 | 連續登入兩次 | 兩次的 `ChainID` **不同**——兩台裝置是兩條鏈 |
| S4 | 帳密不正確 | 回 `ErrCredentialsRejected`；**`Mint` 與 `Save` 都不被呼叫** |
| S5 | `Mint` 失敗 | 原樣回傳；`Save` 不被呼叫 |
| S6 | `Save` 失敗 | 原樣回傳；**不回 `ErrAuthenticationRequired`**（系統壞了，不是憑證的問題） |
| S7 | 簽登入憑證失敗（`ErrAccessTokenUnavailable`） | 原樣回傳 |

## UserService.RenewSession — 續用（US-02、US-03、US-04）

| # | 情境 | 預期結果 |
|---|---|---|
| N1 | 一段有效的登入階段 | 回一對全新的憑證；`FindOneByDigest` 收到的是 `DigestOf(送進來那一份)` |
| N2 | 承 N1 | `Rotate` 收到：`previousID` = 找到那一段的 ID；`next.ChainID` **與舊的相同**、`next.RefreshTokenDigest` = 新 `Mint()` 的 `Digest`、`next.ExpiresAt` = 換發當下 + 30 天 |
| N3 | 承 N1 | 回覆的 `RefreshToken` 是**新的**那一份，**不等於**送進來的那一份 |
| N4 | 承 N1，換發當下 `2026-09-10T08:00:00Z` | `next.ExpiresAt` = `2026-10-10T08:00:00Z` |
| N5 | 續用時**沒有**帶登入憑證 | 照樣成功——`RenewSession` 的簽名裡根本沒有登入憑證 |
| N6 | `FindOneByDigest` 回 `ErrSessionNotFound` | 回 `ErrAuthenticationRequired`；**`RevokeChain` 與 `Rotate` 都不被呼叫** |
| N7 | 找到的那一段**已作廢** | 回 `ErrAuthenticationRequired`；**`RevokeChain(該 ChainID)` 被呼叫一次**；`Rotate` 不被呼叫 |
| N8 | 找到的那一段**已過期**（未作廢） | 回 `ErrAuthenticationRequired`；**`RevokeChain` 不被呼叫**（過期不是盜用）；`Rotate` 不被呼叫 |
| N9 | 找到的那一段有效，但 `FindOne(使用者)` 回 `ErrUserNotFound` | 回 `ErrAuthenticationRequired`；`Rotate` 不被呼叫 |
| N10 | N6、N7、N8、N9 的錯誤訊息 | **一字不差**，且都等於 `"請重新登入"` |
| N11 | 送進來的續用憑證是空字串 | 回 `ErrAuthenticationRequired`；**儲存層完全不被觸碰** |
| N12 | `FindOneByDigest` 回其他儲存失敗 | 原樣回傳（不偽裝成請重新登入） |
| N13 | `RevokeChain` 在盜用路徑上失敗 | 原樣回傳該儲存失敗——**撤不掉整條鏈時不能假裝只是要重新登入** |
| N14 | `Rotate` 失敗 | 原樣回傳 |

## UserService.RevokeSession — 登出（US-05）

| # | 情境 | 預期結果 |
|---|---|---|
| V1 | 一段有效的登入階段 | 成功（無錯誤）；`RevokeChain(該 ChainID)` 被呼叫一次 |
| V2 | `FindOneByDigest` 回 `ErrSessionNotFound` | **成功**——目的已經達成；`RevokeChain` 不被呼叫 |
| V3 | 找到的那一段已經作廢 | **成功**；`RevokeChain` 仍然被呼叫一次（撤一條已經撤過的，結果相同） |
| V4 | 找到的那一段已經過期 | **成功**；`RevokeChain` 被呼叫——過期不影響「把這條鏈收掉」這件事 |
| V5 | 送進來的續用憑證是空字串 | **成功**；儲存層完全不被觸碰 |
| V6 | `FindOneByDigest` 回其他儲存失敗 | 原樣回傳失敗 |
| V7 | `RevokeChain` 失敗 | 原樣回傳失敗 |
| V8 | 全程 | **後端的登入憑證不受影響**（沒有任何撤銷它的呼叫存在） |

## SessionRepository — 儲存（需 `TEST_POSTGRES_DSN`，未設則跳過）

| # | 情境 | 預期結果 |
|---|---|---|
| Q1 | `Save` 一段新的 | 回傳的 `ID` > 0、`CreatedAt` 非零值 |
| Q2 | `FindOneByDigest(存在的留存樣)` | 回那一段，欄位與存進去的相同 |
| Q3 | `FindOneByDigest(不存在的)` | 回 `ErrSessionNotFound` |
| Q4 | `Save` 兩段留存樣相同的 | 第二次失敗——留存樣帶唯一索引 |
| Q5 | `Rotate(舊 ID, 新的一段)` | 回傳新的那一段（`ID` > 0）；舊的那一段的 `RevokedAt` **不再是 nil** |
| Q6 | `Rotate` 之後 `FindOneByDigest(舊的留存樣)` | 仍然找得到，但已作廢——**歷史留著，盜用偵測才有東西可以撞上** |
| Q7 | `Rotate` 的新那一段違反唯一索引 | 整個交易回滾：舊的那一段**仍然沒有被作廢** |
| Q8 | `RevokeChain(chainID)` | 該鏈上**每一段**的 `RevokedAt` 都不再是 nil，包含原本有效的那一段 |
| Q9 | `RevokeChain` 對另一條鏈 | 另一條鏈**完全不受影響** |
| Q10 | `RevokeChain(不存在的 chainID)` | 不算失敗 |
| Q11 | `RevokeChain` 對一條已經全部作廢的鏈 | 不算失敗，且不改動原本的作廢時刻 |
| Q12 | 刪除一位有兩段登入階段的使用者 | 那兩段**跟著消失**（由 CASCADE 保證） |
| Q13 | 連線已關閉時的每一個方法 | 回儲存失敗，**不是**「找不到」 |
| Q14 | `SchemaMigrator.Migrate()` | 回傳的表名清單含 `Sessions` |

## UserController — 狀態碼對映（全部 US）

| # | 情境 | 預期結果 |
|---|---|---|
| C1 | `POST /sessions` 成功 | 200，body 含 `accessToken`／`expiresAt`／`refreshToken`／`refreshTokenExpiresAt` |
| C2 | `POST /sessions/renewal` 成功 | 200，body 同上四個欄位 |
| C3 | `POST /sessions/renewal` body 不是合法 JSON | 400 |
| C4 | `POST /sessions/renewal` 回 `ErrAuthenticationRequired` | 401，`message` = `"請重新登入"` |
| C5 | `POST /sessions/renewal` 回 `ErrAccessTokenUnavailable` | 503 |
| C6 | `POST /sessions/renewal` 回其他錯誤 | 502 |
| C7 | `POST /sessions/revocation` 成功 | **204**，且沒有 body |
| C8 | `POST /sessions/revocation` 帶一份從未存在的續用憑證 | **204**——目的已經達成 |
| C9 | `POST /sessions/revocation` body 不是合法 JSON | 400 |
| C10 | `POST /sessions/revocation` 回儲存失敗 | 502 |
| C11 | `GET /users/me` | **行為與上一個切片完全相同**——它一次資料庫都不讀 |
