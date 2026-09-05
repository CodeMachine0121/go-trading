# Contract Traceability Matrix — 登入階段的續用與結束

Contract: `.sdd/2026-09-05-session-renewal/PRD.md`
Design map: `.sdd/2026-09-05-session-renewal/ARCH.md`
Implementation: `internal/domain/models/{entities,domains,dto,vo}/`、`internal/domain/service/user_service.go`、
`internal/application/user_application.go`、`internal/controller/user_controller.go`、
`internal/infrastructure/{persistence,security}/`
Oracle: Acceptance Criteria（US-01…US-06，共 26 個 scenario）

## US-01 — 登入給的是一對憑證，並開一段登入階段

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 登入回覆一對憑證 | 兩份憑證與兩個到期時刻 | `user_service.go:SignIn` | `user_application_test.go`「hands back both halves」、`user_controller_test.go`「answers with both proofs and both moments」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 登入留下一段登入階段 | `SessionRepository.Save` 收到那一段 | `user_service.go:SignIn` | 「stores a session holding the digest」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 兩個到期時刻各自照自己的期限算 | 08:15 與隔月 5 日 08:00 | `newSessionMaterial` + `SignIn` | 「hands back both halves」（兩個值都斷言）＋「a shorter access token expires sooner」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 兩台裝置各一段、彼此獨立 | 兩條不同的換發鏈 | `SignIn`（每次登入以新的留存樣起一條鏈） | `session_repository_test.go`「leaves other chains alone」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 續用憑證的原文不留存 | 表裡只有留存樣 | `entities.Session` 的欄位 | 「stores a session holding the digest」（明確斷言存的不等於原文）＋實跑驗證：原文在表中出現 0 次 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 建立帳號後仍然直接就是登入狀態 | 不必再填一次 | `RegisterUser`（不變） | `user_application_test.go` 的 RegisterUser 那一組 | asserts-oracle | produces-oracle | ✅ conforms |

## US-02 — 拿續用憑證換一對新的

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-7 | 換得到一對全新的、新舊不同 | 回的是新的那一份 | `RenewSession` | 「trades the proof for a fresh pair」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 續用不需要登入憑證也不需要密碼 | 簽名裡沒有它們 | `RenewSession(ctx, SessionRenewalDto)` | `session_renewal_dto.go` 只有一個欄位；controller 測試只送 `refreshToken` | structural | produces-oracle | ✅ conforms |
| AC-9 | 新的到期時刻從換發當下重算 | 換發當下 + 30 天 | `SessionDomain.Renewed` | `session_domain_test.go`「starts the clock again from now」（兩個不同的當下） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 續用之後舊的那一份不能再用 | 舊的當場作廢 | `SessionRepository.Rotate` | `session_repository_test.go`「ends the old and opens the new」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-03 — 續用被拒絕永遠只有一種說法

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-11 | 查無這份續用憑證 | 「請重新登入」 | `RenewSession` + `sessionHolding` | 「a proof matching nothing is refused」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 續用憑證已經過期 | 同上 | `SessionDomain.Expired` | 「an expired proof is refused without tearing the chain down」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 使用者已經不在 | 同上 | `RenewSession` | 「a proof for somebody who is gone is refused」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 四種失敗一字不差 | 三次 `Error()` 相同且等於「請重新登入」 | 同一個 error 值 | 「every way of failing says exactly the same sentence」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-04 — 一份用過的續用憑證再出現，整條鏈全部作廢

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-15 | 用過的再出現 → 整條鏈作廢 | `RevokeChain(該鏈)` 被呼叫 | `RenewSession` 的 `Revoked()` 分支 | 「a proof that was already used tears down the whole chain」＋`session_repository_test.go`「ends every session of one sign-in」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 真正的持有者也被登出 | 目前那一份也不能用 | `RevokeChain` 撤整條（含未作廢的） | `session_repository_test.go` 斷言鏈上三段全部被撤；實跑驗證步驟 5 得到 401 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 另一條鏈照常成功 | 不受影響 | `RevokeChain` 只照 `ChainID` | `session_repository_test.go`「leaves other chains alone」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-05 — 登出是真的登出

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-18 | 登出後那份續用憑證換不到東西 | 之後續用被拒絕 | `RevokeSession` → `RevokeChain` | 「ends the whole sign-in」＋實跑驗證步驟 8 得到 401 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 登出撤的是整條換發鏈 | 鏈上每一份都失效 | `RevokeChain` | `session_repository_test.go`「ends every session of one sign-in」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 登出不存在的登入階段也算成功 | 成功 | `sessionHolding` 回「沒有」→ `RevokeSession` 回 nil | 「a proof matching nothing is success」＋controller「answers no content even when there was nothing to end」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 重複登出也算成功 | 成功 | 同上 | 「signing out twice is success both times」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 登出的是這一台，不是這個人 | 另一台仍然有效 | `RevokeChain` 只照 `ChainID` | `session_repository_test.go`「leaves other chains alone」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-06 — 使用者不在了，他的登入階段也不在

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-23 | 刪除使用者連帶刪掉登入階段 | 一段都不剩 | `Session.UserID` 的 `OnDelete:CASCADE` | `session_repository_test.go`「loses every session of a deleted user」（真資料庫） | asserts-oracle | produces-oracle | ✅ conforms |

## §4 核心業務規則

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 登入憑證仍然不留存，一般請求不多讀資料庫 | `IdentifyUser` 只看簽章 | `IdentifyUser`（本切片一字未改） | 上一個切片的 `IdentifyUser` 測試全數沿用且照樣通過 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 續用憑證只能用一次 | 換發即輪替 | `Rotate` | 見 AC-10、AC-15 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 留存樣算不回去、固定長度、**查得到** | 同一份輸入永遠同一個留存樣 | `RandomRefreshTokenProxy` | `random_refresh_token_proxy_test.go`（同一份兩次相同、與原文不同、長度固定 64） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 作廢與寫入必須同一個交易 | 新的寫不成時舊的不得被動到 | `Rotate` 的 `Transaction` | `session_repository_test.go`「leaves the old alone when the new cannot be written」；mutation 把交易拆掉即被抓到 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 盜用偵測撤整條，不是那一份 | 見 AC-15、AC-16 | `RenewSession` + `RevokeChain` | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 每一種續用失敗說同一句 | 見 AC-14 | 同一個 error 值 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 續用時資料庫壞掉 ≠ 請重新登入 | 原樣回傳 | `RenewSession` | 「storage being broken is not a reason to sign in again」、「storage failing on the owner lookup…」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 沒有簽章鑰匙時續用也簽不出來 | 503 | `newSessionMaterial` → controller | 「a renewal that cannot be signed for rotates nothing」＋controller「answers service unavailable」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 兩個續用同時到達 → 其中一個被判盜用 | 可接受的誤判 | `RefreshTokenDigest` 的唯一索引 + `Revoked()` 分支 | `session_repository_test.go`「refuses two sessions holding the same digest」 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans（有程式碼、沒有條款）

| Code | Description | Verdict |
|------|-------------|---------|
| `Session.ChainID` 首段等於自己的留存樣 | 新開一條鏈時，鏈的識別碼就用起頭那一份的留存樣 | undocumented——它已經是為這次登入產生的、帶唯一索引的隨機值，再產一個等於同一件事有兩個來源。已在程式碼註解寫明 |
| `RevokeChain` 只更新尚未作廢的那幾列 | 保住每一段第一次被撤掉的時刻 | undocumented——PRD 沒說，但覆寫掉就等於抹掉事後追查的起點 |
| `Rotate` 的作廢時刻取自資料庫的 `now()` | 與同一列的其他時間戳走同一條時間軸 | undocumented——兩個時鐘比「這是什麼時候結束的」這個問題本身多一個 |

## Summary

- Conforms: 32/32 clauses ✅（100%）
- Violations / Mis-asserted / Partial / Gaps / Unclear：**無**
- Orphans: 3（皆為實作層面的必要收緊，無一違反條款）

### 三件必須據實說明的事

1. **`SessionRepository.Rotate` 的分支覆蓋率是 91.7%，不是 100%。**
   未覆蓋的是「作廢舊的那一列這個動作本身失敗」。要讓它從外面發生，得讓連線壞掉，
   而連線壞掉時交易根本開不起來、走不到那一行。保留該分支、逐行人工稽核並在此記下，
   不把數字說成 100%。其餘每一個新增函式的敘述覆蓋率皆為 100%。

2. **ORACLE 的 T7 在實作前被改過一次，而且是往「更誠實」的方向改。**
   它原本寫「續用憑證長度 ≥ 43」——那是先假設了實作會自己取 32 位元組再編碼。
   改用標準函式庫專為此用途提供的產生器之後長度是 26（至少 128 位元的亂數）。
   **改的是一個寫錯的假設，不是為了遷就程式碼調低標準**：標準本來就該是「猜不到」，
   而不是某一個特定的位元組數。已在 ORACLE 內就地註記。

3. **盜用偵測會誤傷，這一點沒有被測試「證明不會發生」，因為它就是會發生。**
   兩個分頁同時續用、或網路不穩導致的重試，都會踩到 AC-15 那條路徑，
   結果是真正的持有者被登出一次。PRD §7 明列它為已知風險並接受；
   要處理的話做法是給剛換發過的舊憑證幾秒寬限期，**不是**拿掉偵測。
