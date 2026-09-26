# Contract Traceability Matrix — 外掛授權

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` + `cmd/server/`（branch `feat/connector-authorization`）
Oracle: Acceptance Criteria (43 scenarios) + Core Business Rules (8) + Non-Functional (4)

Legend for test sites: **A** = `internal/application/tests/connector_authorization_application_test.go`,
**C** = `internal/controller/tests/connector_authorization_controller_test.go`,
**D** = `internal/domain/models/domains/tests/connector_authorization_domain_test.go`,
**P** = `internal/infrastructure/persistence/tests/connector_authorization_repositories_test.go`,
**J** = `internal/infrastructure/security/tests/jwt_access_token_proxy_test.go`,
**S** = `cmd/server/{request_limits,market_data_sign_in,routes}_test.go`, **G** = `internal/config/tests/application_config_test.go`.
Impl sites: `svc` = `internal/domain/service/connector_authorization_service.go`, `user` = `internal/domain/service/user_service.go`,
`ctl` = `internal/controller/connector_authorization_controller.go`, `deps` = `cmd/server/dependencies.go`.

> Static conformance audit, run once before and once after fixes. The first pass found four clauses without a test that pinned the oracle
> (AC-01.7, AC-03.5, AC-04.12, AC-06.2) and one spec error (NFR-1 said introspection uses the credential allowance, contradicting the
> cross-repo decision and the implementation). Tests were added for the four, and the PRD/UL-MAP wording was corrected; the matrix below is the post-fix state.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01.1 | 送回地址是本機即登記成功 | 成功並拿到新外掛代號；說明只用授權碼/續用、不用祕密 | svc:58, `entities/connector_client.go` ToDto | A:126, C RegistersConnectors | asserts-oracle | produces-oracle | ✅ |
| AC-01.2 | 本機新式位址任意埠也可以 | 成功 | `connector_redirect_uri_domain.go:17` | D LoopbackHttp "IPv6 loopback" | asserts-oracle | produces-oracle | ✅ |
| AC-01.3 | 指向別台機器的送回地址被拒 | 拒絕「送回地址不合格」 | redirect_uri_domain:17 | D, A:152, C "an address elsewhere" | asserts-oracle | produces-oracle | ✅ |
| AC-01.4 | 本機但使用加密協定被拒 | 拒絕「送回地址不合格」 | redirect_uri_domain:17 | D "loopback over https" | asserts-oracle | produces-oracle | ✅ |
| AC-01.5 | 沒有送回地址被拒 | 拒絕「送回地址不合格」 | registration_domain:28 | D "no address at all", C "no address" | asserts-oracle | produces-oracle | ✅ |
| AC-01.6 | 要求使用外掛祕密被拒 | 拒絕「外掛資料不合格」 | registration_domain:28 | D metadata, C "a secret-based method" | asserts-oracle | produces-oracle | ✅ |
| AC-01.7 | 登記套用身分相關動作額度 | 同一來源用完後被請稍後再來 | deps:442 | S TestCredentialRoutesHaveTheStricterAllowance "connector registration" | asserts-oracle | produces-oracle | ✅ (fixed) |
| AC-02.1 | 完整的授權請求把使用者送到網頁授權頁 | 記下 10 分鐘有效的請求；送到網頁公開網址的授權頁並帶代號 | svc:82 | A:199, C StartsAuthorization | asserts-oracle | produces-oracle | ✅ |
| AC-02.2 | 本機地址比對不看連接埠 | 視為同一地址；日後送回用請求時的 51000 | `connector_client_domain.go:14` | D RecognisesItsAddresses, A:199 (stored 51000), A:382 (redirect 51000) | asserts-oracle | produces-oracle | ✅ |
| AC-02.3 | 路徑不同直接拒絕、不送回 | 拒絕「送回地址不合格」，不送往任何地方 | client_domain:14, ctl:59 | A:224, C "an unregistered path"（無 Location） | asserts-oracle | produces-oracle | ✅ |
| AC-02.4 | 外掛代號不存在直接拒絕、不送回 | 拒絕「外掛不存在」，不送往任何地方 | svc:82 | A:224, C "an unknown connector" | asserts-oracle | produces-oracle | ✅ |
| AC-02.5 | 缺挑戰時把使用者送回外掛 | 送回附「請求無效」與 abc；不記下任何請求 | start_domain:30 | A:250（未設定 Save 期待，呼叫即失敗）, D | asserts-oracle | produces-oracle | ✅ |
| AC-02.6 | 挑戰方式不支援時送回外掛 | 送回附「請求無效」 | start_domain:30 | D "plain challenge method", C incomplete | asserts-oracle | produces-oracle | ✅ |
| AC-02.7 | 要求的不是授權碼時送回外掛 | 送回附「請求無效」 | start_domain:30 | D "not asking for a code" | asserts-oracle | produces-oracle | ✅ |
| AC-02.8 | 權限範圍被忽略 | 照常記下請求 | ctl:59（不讀 scope） | C "a complete request, scope and all" | asserts-oracle | produces-oracle | ✅ |
| AC-03.1 | 查詢有效的待授權請求 | 看得到 Claude Code 與到期時刻 | svc:121 | A:316, C "an open request" | asserts-oracle | produces-oracle | ✅ |
| AC-03.2 | 剛好 10 分鐘即過期 | 回覆不存在/過期/已決定 | request_domain:22 | D IsOpenUntil, A GetConnectorAuthorizationRequest, C "an expired request" | asserts-oracle | produces-oracle | ✅ |
| AC-03.3 | 已登入且開通的使用者允許 | 送回地址附授權碼與 abc；請求不再存在 | svc:143, `connector_authorization_request_repository.go` Approve | A:382, C approval, P ApprovesOnce | asserts-oracle | produces-oracle | ✅ |
| AC-03.4 | 未登入不能允許 | 要求先登入；請求仍在 | deps:447 | C "approval without signing in", S TestApprovingAConnectorRequiresSignIn | asserts-oracle | produces-oracle | ✅ |
| AC-03.5 | 待開通的使用者不能允許 | 拒絕並附開通指示 | deps:447 + AuthenticationMiddleware | C TestConnectorAuthorizationControllerRefusesApprovalFromAPendingAccount | asserts-oracle | produces-oracle | ✅ (fixed) |
| AC-03.6 | 未登入也能拒絕 | 送回附「使用者拒絕」與 abc；請求消失 | svc:175, deps:449 | A:450, C "denial needs no sign-in", S | asserts-oracle | produces-oracle | ✅ |
| AC-03.7 | 已決定過的請求不能再決定 | 回覆不存在/過期/已決定 | request_domain:22 + 條件式更新 | A:461, C "a decided request", P DeniesOnce | asserts-oracle | produces-oracle | ✅ |
| AC-03.8 | 過期的請求不能決定 | 回覆不存在/過期/已決定 | request_domain:22 | A:416, C "approving an expired request" | asserts-oracle | produces-oracle | ✅ |
| AC-04.1 | 以授權碼換到授權 | 登入憑證標 R、有效秒數、續用憑證；另起新登入 | svc:216, code_domain:60 | A:517（Issue 帶 R、Redeem 的新鏈、900 秒）, C token | asserts-oracle | produces-oracle | ✅ |
| AC-04.2 | 剛好 5 分鐘的授權碼已失效 | 授權碼無效 | code_domain:42 | D ExpiresAfterFiveMinutes, A table | asserts-oracle | produces-oracle | ✅ |
| AC-04.3 | 挑戰答案不對 | 授權碼無效 | code_domain:47 | D Accepts, A table | asserts-oracle | produces-oracle | ✅ |
| AC-04.4 | 送回地址差一個埠號 | 授權碼無效 | code_domain:47 | D "a different port", A table | asserts-oracle | produces-oracle | ✅ |
| AC-04.5 | 別的外掛拿去換 | 授權碼無效 | code_domain:47 | D "another connector", A table | asserts-oracle | produces-oracle | ✅ |
| AC-04.6 | 授權碼被用第二次 | 授權碼無效，且第一次換出的登入整段作廢 | svc:216/302 | A:576, A:603, P RedeemsOnce | asserts-oracle | produces-oracle | ✅ |
| AC-04.7 | 缺少必要資料 | 請求無效 | code_exchange_domain:9 | D NeedsEveryField, A:653, C "a missing verifier" | asserts-oracle | produces-oracle | ✅ |
| AC-04.8 | 不支援的換法 | 不支援的換法 | ctl:124 | C "the password grant" | asserts-oracle | produces-oracle | ✅ |
| AC-04.9 | 續用後仍標著同一個對象服務 | 新登入憑證標 R；舊續用憑證作廢 | user:230, session_domain Renewed | A:731, C connector renews, D CarriesTheConnectorAndAudience | asserts-oracle | produces-oracle | ✅ |
| AC-04.10 | 重用已用過的續用憑證 | 授權無效，整段作廢 | user:230 | A:760 | asserts-oracle | produces-oracle | ✅ |
| AC-04.11 | 續用時外掛代號不符 | 授權無效 | user:250 | A:774, C "from another connector", D RenewsOnlyThroughTheChannel | asserts-oracle | produces-oracle | ✅ |
| AC-04.12 | 換授權套用身分相關動作額度 | 同一來源用完後被請稍後再來 | deps:451 | S TestCredentialRoutesHaveTheStricterAllowance "connector token exchange" | asserts-oracle | produces-oracle | ✅ (fixed) |
| AC-04.13 | 換授權的回覆不被暫存 | 成功或失敗都要求不得暫存 | ctl:124 | C（每個 token 案例都斷言 no-store） | asserts-oracle | produces-oracle | ✅ |
| AC-05.1 | 外掛換到的憑證有效 | 有效，使用者 7、R、到期時刻 | svc:316, `vo/access_token_claims_vo.go` | A:816, C active token | asserts-oracle | produces-oracle | ✅ |
| AC-05.2 | 網頁登入的憑證有效但沒有對象服務 | 有效，對象服務為空 | svc:316 | A:832 | asserts-oracle | produces-oracle | ✅ |
| AC-05.3 | 過期或被竄改的憑證 | 只回無效 | svc:316, jwt ClaimsOf | A inactive table, C inactive, J ReadsNoClaims | asserts-oracle | produces-oracle | ✅ |
| AC-05.4 | 使用者已不存在 | 只回無效 | svc:316 | A "a token whose user is gone" | asserts-oracle | produces-oracle | ✅ |
| AC-05.5 | 交易服務自己的功能接受兩種憑證 | 帶對象服務的憑證同樣被放行 | `jwt_access_token_proxy.go` UserIdentifiedBy | J AcceptsAConnectorTokenForTheServiceItself | asserts-oracle | produces-oracle | ✅ |
| AC-06.1 | 說明書以營運者設定的公開網址為準 | 發行者去結尾斜線；四個位置在其下；只支援授權碼/續用/S256/無祕密 | `vo/connector_authorization_policy_vo.go`, config Load | A:107, C DescribesItself, G PublicAddresses | asserts-oracle | produces-oracle | ✅ |
| AC-06.2 | 來訪請求宣稱的協定不影響說明書 | 仍是加密協定網址 | policy VO（不讀請求） | C TestConnectorAuthorizationControllerIgnoresTheSchemeAProxyClaims | asserts-oracle | produces-oracle | ✅ (fixed) |
| BR-1 | 送回地址只能是本機、非加密；至少一個 | 其餘一律拒絕 | redirect_uri_domain:17 | D | asserts-oracle | produces-oracle | ✅ |
| BR-2 | 比對忽略埠號；記下請求時原樣地址 | 同 AC-02.2 | client_domain:14 | D, A | asserts-oracle | produces-oracle | ✅ |
| BR-3 | 10 分鐘 / 5 分鐘，到期那一刻即失效 | 邊界即失效 | request_domain:22, code_domain:42 | D | asserts-oracle | produces-oracle | ✅ |
| BR-4 | 只能決定一次；授權碼只能換一次 | 條件式寫入 | request/code repositories | P ApprovesOnce / ConcurrentDecisions / RedeemsOnce | asserts-oracle | produces-oracle | ✅ |
| BR-5 | 代號與授權碼為隨機值；授權碼只留指紋 | 存的是留存樣 | svc:143 | A:382（CodeDigest=留存樣） | asserts-oracle | produces-oracle | ✅ |
| BR-6 | 外掛登入與網頁登入分屬不同換發鏈，規則相同 | 新鏈；續用規則共用 | code_domain:60, user:230 | A:517, A:786 | asserts-oracle | produces-oracle | ✅ |
| BR-7 | 查驗只有有效/無效 | 無效不說原因 | svc:316 | A, C | asserts-oracle | produces-oracle | ✅ |
| BR-8 | 同時允許 / 同時兌換只成功一次；不留半成品 | 另一個得到不存在／被當作重用；失敗不留半成品 | repositories | P ConcurrentDecisions, UndoTheDecisionWhenTheSecondWriteFails; A:404, A:603 | asserts-oracle | produces-oracle | ✅ |
| NFR-1 | 登記與換授權套用身分相關額度；查驗只套一般額度 | 登記/換授權第 11 次被擋；查驗 11 次都通過 | deps:442/451/454 | S TestCredentialRoutes…, TestIntrospectionStaysOnTheGeneralAllowance | asserts-oracle | produces-oracle | ✅ (spec fixed) |
| NFR-2 | 換授權的回覆不得被暫存 | no-store | ctl:124 | C | asserts-oracle | produces-oracle | ✅ |
| NFR-3 | 不可信的送回地址絕不作為送回目的地 | 不 302、不回 redirectTo | ctl:59, request_domain:73 | C（無 Location）, A:436/482 | asserts-oracle | produces-oracle | ✅ |
| NFR-4 | 授權碼只留指紋；續用沿用留存樣 | 同 BR-5 | svc | A | asserts-oracle | produces-oracle | ✅ |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `connector_client_registration_domain.go:28` | 送回地址最多 10 個、每個 ≤ 2048 字、不得含 fragment 或帳密段；名稱 ≤ 128 字、空白記為「未命名外掛」；`grant_types`/`response_types` 只收支援的值 | undocumented in PRD — recorded as ARCH decisions 2/3; benign hardening |
| `connector_authorization_start_domain.go:30` | 挑戰 > 128 字、回執記號或對象服務 > 2048 字時送回「請求無效」 | undocumented in PRD — storage bound, benign |
| `svc:121` | 待授權請求的外掛已不存在時回「不存在、已過期或已經決定過」 | undocumented — consistent with the single-answer rule |
| `svc:216` | 換授權時外掛代號不存在 → `invalid_client`（存在但不符 → `invalid_grant`） | recorded as ARCH decision 8 |
| `svc:216` | 授權碼的使用者已不存在 → 授權碼無效 | recorded as ARCH decision 9 |
| `user:215` | 網頁續用路徑不能續用外掛登入（反之亦然） | recorded as ARCH decision 6 |

None of the orphans implements an Out-of-Scope item.

## Summary

- Conforms: 55/55 clauses ✅ (100%)
- Violations: none
- Mis-asserted: none
- Partial: none (AC-01.7, AC-03.5, AC-04.12, AC-06.2 were partial in the first pass; tests added)
- Gaps: none
- Unclear: none
- Orphans: 6 (all benign hardening or recorded ARCH decisions)
- Spec correction: NFR-1 wording (introspection allowance) aligned with the cross-repo decision; UL-MAP row updated.
