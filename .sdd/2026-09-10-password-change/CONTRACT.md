# Contract Traceability Matrix — 變更密碼

Contract: `.sdd/2026-09-10-password-change/PRD.md`
Design map: `.sdd/2026-09-10-password-change/ARCH.md`
Implementation: `internal/{domain,application,controller,infrastructure}` · `cmd/server`
Oracle: Acceptance Criteria（19 個情境）＋ 核心業務規則（7 條）＋ 非功能需求（4 條）

> 這是一次**靜態一致性稽核**：逐條把測試的斷言與程式的執行路徑各自對照規格導出的預期結果，
> 不是「跑一次測試看綠燈」。判定不採信 pass/fail。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 給對目前的密碼就換得成功 | 變更成功；之後新密碼登得進去、舊密碼被回「電子郵件或密碼不正確」 | `user_service.go:357` · `user_repository.go:123` | `user_application_test.go:922` · `user_repository_test.go:154` · `postman:變更密碼/舊密碼登不進去了`、`新密碼登得進去` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 目前的密碼填錯就整次拒絕 | 拒絕並顯示「目前的密碼不正確」；舊密碼仍登得進去 | `user_service.go:381` | `user_application_test.go:941`（mock 未註冊寫入 → 觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 沒有登入就換不了 | 拒絕並顯示「請重新登入」 | `authentication_middleware.go` · `dependencies.go` 路由 | `user_controller_test.go:555` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 帶著過期的身分也換不了 | 拒絕並顯示「請重新登入」 | 同上（過期由 `IdentifyUser` 判） | `middlewares/tests`（既有）＋ `user_application_test.go:972`（使用者已不在） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 只換得動自己的 | 換掉的是發動者的密碼；別人的不動 | `user_controller.go:142`（識別碼取自門） | `user_application_test.go:1072` · `user_repository_test.go:200` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 一般長度的新密碼換得成功 | 變更成功 | `password_change_domain.go:36` | `password_change_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 剛好 8 個字元換得成功 | 變更成功 | `password_domain.go`（既有） | `password_change_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 7 個字元被拒絕 | 拒絕並顯示「密碼至少要 8 個字元」 | `password_domain.go`（既有） | `password_change_domain_test.go:50` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 剛好 72 個位元組換得成功 | 變更成功 | `password_domain.go`（既有） | `password_change_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 75 個位元組被拒絕而不是被截短 | 拒絕並說明上限；密碼沒被換成前 24 個字 | `password_domain.go`（既有）＋ `bcrypt_password_proof_proxy.go`（第二道鎖） | `password_change_domain_test.go:50` · `user_application_test.go:1034` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 空的新密碼被拒絕 | 拒絕並顯示「必須給一組密碼」 | `password_domain.go`（既有） | `password_change_domain_test.go:50` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 8 個空白字元是一組（很糟的）合法密碼 | 變更成功；不予去除空白 | `password_change_domain.go:36`（不 trim） | `password_change_domain_test.go:13` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 新舊完全相同被拒絕 | 拒絕並顯示「新密碼不得與目前的密碼相同」 | `password_change_domain.go:52` | `password_change_domain_test.go:88` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 只差一個大寫就是不同的密碼 | 變更成功 | `password_change_domain.go:52`（字面比對） | `password_change_domain_test.go:100` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 兩台裝置一起失效 | 兩段登入階段都不再成立 | `user_repository.go:123` 交易內 | `user_repository_test.go:171` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 變更失敗不會把人踢出去 | 兩段登入階段仍成立；密碼沒變 | 失敗路徑皆在寫入之前回傳 | `user_application_test.go:941`（未註冊 `ChangePasswordProof`，觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 換完之後舊的續用憑證也換不到東西 | 換發被拒並顯示「請重新登入」 | 既有 `RenewSession` 讀到 `Revoked()` | `postman:變更密碼/換完之後，舊的續用憑證也換不到東西` · `user_repository_test.go:171`（`revoked_at` 已寫入） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 回覆裡沒有密碼也沒有密碼證明 | 回覆內容為空 | `user_controller.go:167`（204） | `user_controller_test.go:502` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 留存下來的是換過的密碼證明，不是密碼 | 找不到明文；證明與變更前不同 | `user_service.go:385`（`Prove`）· `user_repository.go:123` | `user_repository_test.go:154` · `user_application_test.go:922` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 只有本人 | 變更對象一律是目前登入者 | `user_controller.go:142` · `models/password_change_request.go`（無使用者欄位） | `user_application_test.go:1072` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 目前的密碼講得清楚，且與登入的含糊回覆不同 | 明說「目前的密碼不正確」，不是「電子郵件或密碼不正確」 | `user_errors.go:45` | `user_application_test.go:960`（`NotErrorIs` 兩種） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 新密碼與建立帳號同一套規則，太長是拒絕 | 見 AC-6…AC-12 | `password_change_domain.go:37`（委給 `PasswordDomain`） | `password_change_domain_test.go:13/50` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 新舊不得相同 | 見 AC-13 | `password_change_domain.go:52` | `password_change_domain_test.go:88` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 換成功即全域登出 | 該使用者每一條換發鏈一併作廢 | `user_repository.go:141`（依 `user_id` 而非 `chain_id`） | `user_repository_test.go:171` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 換證明與作廢登入階段要嘛都成、要嘛都不成 | 不存在「換了密碼卻留著舊登入階段」的中間狀態 | `user_repository.go:126`（單一 GORM 交易） | `user_repository_test.go:263`（寫失敗時密碼原封不動＝交易回滾） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 失敗即完全不變 | 見 AC-16 | 同上 | `user_application_test.go:941/1015` · `user_repository_test.go:263` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 判斷順序：先判新密碼合法與新舊相同，再比對目前的密碼 | 純填錯格式的請求不必等 bcrypt | `user_service.go:362`（驗證在 `FindOne`／`Matches` 之前） | `user_application_test.go:1034`（未註冊任何 repository／proxy，觸及即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 一次變更的等待在一秒以內 | 一次 bcrypt 比對＋一次寫入 | `user_service.go:357` | — | no-test | produces-oracle（cost 12，數百毫秒；無新增的來回） | 🟡 partial |
| NFR-2 | 兩組密碼都不留存、不回覆、不寫進紀錄檔 | 任何路徑都不寫出密碼 | 全路徑無 log；回覆 204 | `user_controller_test.go:502`（回覆）；紀錄檔無測試 | shallow（只涵蓋回覆） | produces-oracle | 🟠 mis-asserted |
| NFR-3 | 新的密碼證明摻新的隨機料 | 與舊的比對不出關係 | `bcrypt_password_proof_proxy.go`（既有，bcrypt 自帶 salt） | `security/tests`（既有） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 目前的密碼填錯不透露額外資訊 | 只回那一句 | `user_errors.go:45` | `user_controller_test.go:525` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `user_repository.go:145` | 交易內作廢登入階段失敗時的錯誤傳遞分支 | 防禦性分支，無測試涵蓋（見下方說明）；不屬 out-of-scope，非違規 |
| `user_service.go:376`／`user_application_test.go:986` | 「留存處讀寫失敗回報為系統問題」 | PRD §4 Edge Cases 有寫但未列為 AC；建議日後補一條 AC |

## Summary

- Conforms: 28/30 clauses ✅（93%）
- Violations: 無
- Mis-asserted: `NFR-2`（「不寫進紀錄檔」沒有任何測試守著；目前靠人工檢視全路徑無 log 呼叫）
- Partial: `NFR-1`（沒有效能測試，靠既有 bcrypt cost 推論）
- Gaps: 無
- Unclear: 無
- Orphans: 2（皆非違規；一個是無法由測試觸發的防禦性分支，一個是 Edge Case 未升格為 AC）

**已知未涵蓋的程式路徑**：`user_repository.go:145`（交易內作廢登入階段時資料庫報錯）。
沒有呼叫端可控的輸入能觸發它——它是驅動層失敗的傳遞。刻意保留而不刪除。

**這次稽核的天花板**：靜態一致性稽核。它閱讀測試的斷言與程式的執行路徑並各自對照規格，
**不自行撰寫或執行新的探針**。動態證明請走 `/tdd`。
