# Contract Traceability Matrix — 使用者登入

Contract: `.sdd/2026-09-05-user-authentication/PRD.md`
Design map: `.sdd/2026-09-05-user-authentication/ARCH.md`
Implementation: `internal/domain/models/{entities,domains,dto,vo}/`, `internal/domain/service/user_service.go`,
`internal/application/user_application.go`, `internal/controller/user_controller.go`,
`internal/infrastructure/{persistence,security}/`
Oracle: Acceptance Criteria（US-01…US-05，共 28 個 scenario）

每一列的 Spec-expected 都先只從 PRD 推導，再回頭看實作與測試各自是否產出／斷言了它。

## US-01 — 用電子郵件建立自己的使用者

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 系統一位使用者都沒有時也建得起來 | 建立成功，回覆識別碼與 `james@example.com` | `user_service.go:48` | `user_application_test.go` 「turns the password into a proof and stores what came back」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 回覆內容永遠不含密碼，也不含密碼證明 | 序列化後的回覆只有 `id` 與 `email` | `user_dto.go`、`user.go:ToDto` | `user_test.go`「carries no trace of the password proof」（比對整份 JSON）、`user_controller_test.go`「the answer carries no trace of the password」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 電子郵件的前後空白不予保留 | 留下 `james@example.com` | `email_domain.go:37` | `email_domain_test.go`「the blanks around an address are not part of it」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 電子郵件一律以小寫留存 | `James@Example.com` → `james@example.com` | `email_domain.go:37` | `email_domain_test.go`「capitals are the same address as lower case」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 大小寫不同的同一個電子郵件算同一個人 | 拒絕，說明已經有人用了 | `email_domain.go` + `user.go` 唯一索引 + `user_repository.go:39` | `user_repository_test.go`「refuses an address already held」、`user_controller_test.go`「answers conflict」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 空白的電子郵件被拒絕 | 拒絕，說「必須給一個電子郵件」 | `email_domain.go:37` | `email_domain_test.go`（空字串／半形／全形空白三列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 不像電子郵件的字串被拒絕 | 拒絕，說明格式不對 | `email_domain.go:37` | `email_domain_test.go`（六種不合格拼法） | asserts-oracle | produces-oracle | ✅ conforms |

## US-02 — 密碼有下限也有上限，而且從不以原樣留存

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-8 | 剛好 8 個字元的密碼可以用 | 建立成功 | `password_domain.go:42` | `password_domain_test.go`「exactly the fewest characters allowed」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 7 個字元的密碼被拒絕 | 拒絕，說「至少要 8 個字元」 | `password_domain.go:42` | `password_domain_test.go`「one character short」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 空白的密碼被拒絕 | 拒絕，說「必須給一組密碼」 | `password_domain.go:42` | `password_domain_test.go`「nothing at all」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 剛好 72 個位元組的密碼可以用 | 建立成功 | `password_domain.go:42` | `password_domain_test.go`（72 個 `a`、24 個中文字兩列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 73 個位元組的密碼被拒絕，而不是被截短 | 拒絕，說明太長，且不建立任何使用者 | `password_domain.go:42`；另有第二道鎖在 `bcrypt_password_proof_proxy.go:59` | `password_domain_test.go`（73 個 `a`、25 個中文字）、`bcrypt..._test.go`「refuses a password it could only read the front of」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 留存下來的不是密碼 | 存的字串不等於密碼 | `bcrypt_password_proof_proxy.go:59` | `bcrypt..._test.go`「never hands back the password」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 兩個人用同一組密碼，留下的證明並不相同 | 兩份證明彼此不同，且都驗得過 | `bcrypt_password_proof_proxy.go:59`（bcrypt 自己摻鹽） | `bcrypt..._test.go`「gives a different proof every time」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-03 — 登入並取得一份登入憑證

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-15 | 對得上就發憑證 | 回覆憑證與到期時刻 | `user_service.go:75` | `user_application_test.go`「issues a proof good for as long as a session lasts」、`user_controller_test.go`「answers with the proof and the moment it stops counting」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 到期時刻是現在加上有效期限 | 24 小時設定下為 `2026-09-06T08:00:00Z` | `user_service.go:75` | 同上，另有「a shorter session expires sooner」以 1 小時交叉驗證 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 登入時電子郵件不分大小寫、忽略前後空白 | `　JAMES@Example.com　` 登得進去 | `sign_in_domain.go:37` → `email_domain.go` | `sign_in_domain_test.go`「capitals and blanks are the same account」；application 測試全程用帶空白的大寫位址 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 登入不會留下任何新的東西 | 使用者數量不變 | `user_service.go:75`（整條路只讀不寫） | `user_application_test.go`：整組登入測試對 `Save` 不設任何 expectation，呼叫到即測試失敗 | asserts-oracle | produces-oracle | ✅ conforms |

## US-04 — 登入失敗永遠只有一種說法

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-19 | 密碼錯了 | 拒絕，說「電子郵件或密碼不正確」，且不含憑證 | `user_service.go:75` | `user_application_test.go`「a wrong password issues nothing」、`user_controller_test.go`（斷言整份 body 等於那一句） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 查無這個電子郵件 | 同一句拒絕 | `user_service.go:75` | `user_application_test.go`「an address nobody holds is still put through the check」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 兩種失敗的說法一字不差 | 兩次 `Error()` 完全相同 | `user_errors.go`（同一個 error 值，沒有兩個拼法可寫） | `user_application_test.go`「both failures say exactly the same sentence」、`sign_in_domain_test.go`「says the same sentence whichever half is wrong」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 空白的登入內容一樣被拒絕 | 拒絕 | `sign_in_domain.go:37` | `sign_in_domain_test.go`（四列）、`user_application_test.go`「a sign-in that cannot even be read never reaches storage」 | asserts-oracle | produces-oracle | ✅ conforms |

## US-05 — 帶著憑證，系統認得我是誰

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-23 | 帶著剛簽發的憑證問我是誰 | 回覆識別碼與電子郵件 | `user_service.go:116` | `user_application_test.go`「says who a proof belongs to」、`jwt..._test.go`「issues a proof it can read back」、`user_controller_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 被改過一個字的憑證不成立 | 拒絕，說明要重新登入 | `jwt_access_token_proxy.go:83` | `jwt..._test.go`「altered after it was signed」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 過期的憑證不成立 | 拒絕，說明要重新登入 | `jwt_access_token_proxy.go:83` | `jwt..._test.go`「perfectly signed, but its moment has passed」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 沒帶憑證 | 拒絕，說明要重新登入 | `user_controller.go:95` + `user_service.go:116` | `user_controller_test.go`「turns away a request presenting nothing」（四種缺席方式） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 憑證指向一位已經不存在的使用者 | 拒絕，說明要重新登入 | `user_service.go:116` | `user_application_test.go`「a valid proof for somebody who is gone means signing in again」 | asserts-oracle | produces-oracle | ✅ conforms |

## §4 核心業務規則（PRD Business Flow & Logic）

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 電子郵件的正規化只有一套 | 建得起來的必定登得進去 | `email_domain.go`（建立與登入共用） | `email_domain_test.go` + `sign_in_domain_test.go` 對同一組輸入得到同一個結果 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 唯一性由儲存層的索引決定，不由「先查再寫」 | 沒有任何先查的呼叫 | `user_service.go:48`、`user_repository.go:39` | `user_application_test.go` 對 `FindOneByEmail` 不設 expectation；`user_repository_test.go` 驗真索引 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 密碼證明必須摻隨機料 | 同一組密碼兩次結果不同 | `bcrypt_password_proof_proxy.go:59` | `bcrypt..._test.go`「gives a different proof every time」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 位元組上限是拒絕，不截斷 | 73 位元組不建立 | `password_domain.go:42` | 見 AC-12 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 查無使用者時仍然要花掉比對密碼的時間 | 沒有證明可比對時的回絕耗時 > 10ms | `bcrypt_password_proof_proxy.go:78`（內部換成誘餌） | `bcrypt..._test.go`「refusing no proof costs what refusing a wrong one costs」（實測耗時）；`user_application_test.go` 驗 `Matches` 確實被呼叫 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 憑證不留存，因此無法提前失效 | 沒有任何憑證的儲存 | `jwt_access_token_proxy.go`（無 repository） | 結構上成立：`SchemaMigrator` 沒有憑證的表，`IAccessTokenProxy` 沒有儲存方法 | structural | produces-oracle | ✅ conforms |
| BR-7 | 建立到一半資料庫壞掉 → 不留下半位使用者 | 整次失敗 | `user_repository.go:39`（單一 `Create`） | `user_repository_test.go`「says storage broke rather than answering with nothing」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 沒有簽章鑰匙 → 登入整條路失敗並說明原因 | `ErrAccessTokenUnavailable` → 503 | `jwt_access_token_proxy.go:53`、`user_controller.go:107` | `jwt..._test.go`「with no key refuses to sign anything」、`user_controller_test.go`「answers service unavailable」 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans（有程式碼、沒有條款）

| Code | Description | Verdict |
|------|-------------|---------|
| `email_domain.go` 的 320 位元組上限 | 電子郵件位址的長度天花板 | undocumented——PRD 沒有這一條，但欄位總得有個寬度，且它是電子郵件本身的事實而非本系統的選擇。已補進 UL-MAP 的定義欄 |
| `jwt_access_token_proxy.go` 的 `acceptedSigningMethods` | 只認一種簽章方式，不聽憑證自己說它是怎麼簽的 | undocumented——PRD §6 只說「憑證帶簽章」。這是實作該有的收緊，且 `jwt..._test.go` 有兩個案例守著它 |
| `user_controller.go` 對 `bearer` 不分大小寫 | HTTP 規定 scheme 不分大小寫 | undocumented——傳輸協定的規定，不是業務條款 |
| CORS 允許 `Authorization` 標頭 | 沒有它瀏覽器會靜靜丟掉每一次的憑證 | undocumented——PRD 沒寫，但沒有它前端就用不了這個功能 |

## Summary

- Conforms: 35/35 clauses ✅（100%）
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 4（皆為協定／欄位層面的必要收緊，無一違反條款）

### 兩件必須據實說明的事

1. **`JwtAccessTokenProxy.Issue` 的分支覆蓋率是 85.7%，不是 100%。**
   未覆蓋的是「簽章函式本身失敗」那一條。以位元組鑰匙走 HMAC 時它結構上不可能發生，
   要讓它可測就得為了覆蓋率多開一個注入點——那正是這套做法明講要避免的憑空一般化。
   保留該分支、逐行人工稽核，並在此記下，而不是把數字說成 100%。

2. **PRD 中「US-04 空白的登入內容」原先在 ORACLE 被對到 `SignInDomain` 的長度規則上，
   那是對錯了元件。** 建立時的長度規則不適用於登入（短密碼只是「不對」，不是「格式錯」），
   已於實作前修正 ORACLE 的 S6 並補上 service 層的 I11，而不是回頭改測試去遷就程式碼。
