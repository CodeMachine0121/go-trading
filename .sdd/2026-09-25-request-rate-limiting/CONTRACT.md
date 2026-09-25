# Contract Traceability Matrix — 請求節流

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/models/domains`、`internal/domain/service/request_admission_service.go`、`internal/controller/middlewares`、`cmd/server/request_limits.go`、`cmd/server/serve.go`
Oracle: Acceptance Criteria（25 條）+ Core Business Rules（4 條）+ NFR（2 條），預期值見 `ORACLE.md`（實作前寫下）

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 同一個位置的兩位使用者各算各的 | 小明用完後小華仍被服務 | `request_admission_service.go` `requesterOf` + `requester_domain.go` `Key` | `request_admission_application_test.go` two users behind one address | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 登入憑證已過期就算在來源位置頭上 | 被拒絕，等 1 秒（0.1 秒進位） | `requesterOf` 驗證失敗回退 | 同檔 an expired token counts against the address | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 沒有信得過的轉手時不採信請求自稱的來源 | 來源＝直接連線方 | `request_limits.go` `SetTrustedProxies` | `request_limits_test.go` with no trusted proxy | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 經由信得過的轉手轉來時採信原始來源 | 來源＝1.2.3.4 | `request_limits.go` `RemoteIPHeaders` | 同檔 relayed by the ingress | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 不是從信得過的轉手來的就不採信 | 來源＝位置 C | 同上 | 同檔 outside the trusted range | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 相鄰的新式位址算同一個來源 | 共用一份額度（同一個請求者） | `requester_domain.go` /64 | `requester_domain_test.go` /64 兩案 | asserts-oracle（額度以鍵為身分） | produces-oracle | ✅ conforms |
| AC-7 | 累積的額度容得下一口氣的爆發 | 120 個都被服務 | `request_allowance_ledger_domain.go` `Admit` | ledger「saved-up allowance」、middleware「within the burst」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 用完的那一次被拒絕並說出要等多久 | 拒絕、「請 1 秒後再試」、等待 1 秒 | `Admit` + `request_admission_errors.go` + `request_rate_limit_middleware.go` | ledger「past the burst」、middleware「past the burst」（429、`Retry-After: 1`、handler 未執行） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 被拒絕的那一次不消耗額度 | 0.1 秒後被服務 | `Admit` `CancelAt` | ledger「refused request spends nothing」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 一位請求者用完不影響別人 | 位置 D 被服務 | `Admit`（每鍵一桶） | ledger「does not touch another」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 一分鐘十次以內都受理 | 10 次都受理 | 身分相關 ledger | middleware「the tenth sign-in」、`request_limits_test.go` 四條路由前 10 次 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 第十一次被拒絕並說出要等多久 | 拒絕、「請 6 秒後再試」 | 同上 | ledger「credential budget」、middleware「eleventh sign-in」、routes `Retry-After: 6` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 帶著有效登入憑證也一樣只認來源位置 | 拒絕、等 6 秒 | `AdmitCredentialRequest` | application「a valid token buys no fresh credential allowance」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 兩份額度各算各的 | 查 K 線被服務 | 兩個獨立 ledger | application「leaves the general one alone」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 上限以內開得起來 | 第 20 條開始更新 | `live_stream_occupancy_domain.go` `Occupy` | occupancy「twentieth」、middleware「twentieth」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 到上限時新開的被拒絕，已開的不受影響 | 第 21 條被拒並說明；原本 20 條照常 | `Occupy` + `live_stream_limit_middleware.go` | middleware「twenty-first」 | **shallow**（第二個 Then「原本 20 條照常」沒有斷言） → 已補：held streams 在釋放後都以 200 完成 | produces-oracle | ✅ conforms（補測後） |
| AC-17 | 關掉一條名額立刻空出來 | 再開的那條開始更新 | `Vacate` + middleware `defer` | occupancy「closing one」、middleware「places free」、application「closing one makes room」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 全服務的上限 | 被拒 | `Occupy` total | occupancy「service-wide cap」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 掛很久也不會被切斷 | 超過送達容許時間仍收到更新 | `serve.go` `newServer`（無 WriteTimeout） | `serve_test.go` a live stream outlives the read timeout | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 一般大小的策略腳本照常受理 | 受理 | `request_body_limit_middleware.go` | body「an ordinary script」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 剛好在上限上 | 受理 | 同上 | body「exactly the limit」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 超過上限 | 拒絕、「送入的內容太大，上限為 1024 KB」 | 同上 | body「one byte over, declared」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 送太慢 | 連線被放棄 | `newServer` `ReadTimeout` | `serve_test.go` body never finishes | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 額度補滿的請求者被忘掉 | 只記得最新一位 | `Admit` 清理 | ledger「ten thousand one-off sources」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 額度還沒補滿的不被忘掉 | 仍記得，剩下的不憑空補滿 | 同上 | ledger「still refilling」（記得 2 位；6 次中 5 次通過） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 等待秒數無條件進位、至少一秒 | 0.1 秒 → 1、6 秒 → 6 | `RetryAfterSeconds` | ledger 訊息斷言 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 瀏覽器事先詢問不扣額度、看得到等待秒數 | 130 次預檢後仍被服務；CORS 公開 `Retry-After` | CORS 先於節流；`cors_middleware.go` expose | `request_limits_test.go` preflight、`cors_middleware_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 清理跟著請求發生，不受背景工作總開關影響 | 無排程 | `Admit` 內清理 | ledger 清理案例（不依賴任何 job） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 身分相關動作掛在四條路由 | 四條路由第 11 次被拒 | `dependencies.go` 四條路由 | `request_limits_test.go` 四案 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 不合理的值沿用預設 | 零、負、讀不懂 → 預設 | `application_config.go` | `request_limit_config_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 信得過的轉手寫錯時拒絕啟動 | 程序結束、非零 | `request_limits.go` `log.Fatalf` | `request_limits_test.go` invalid trusted proxies | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `request_body_limit_middleware.go` + controller bind | 未宣告長度而超過上限時，由既有 bind 錯誤回「請求有誤」而非「內容太大」 | undocumented in PRD；ARCH 決策 9 已記錄並接受（瀏覽器與外掛都會宣告長度） |
| `dependencies.go` 兩條即時跟盤路由的跟盤上限掛載 | 由 `TestLiveRoutesHoldALiveStreamPlace` 驗證掛載 | 屬 AC-15～18 的組裝，非孤兒 |

## Summary

- Conforms: 31/31 clauses ✅ (100%)，其中 AC-16 在稽核時為 shallow，已補斷言
- Violations: —
- Mis-asserted: —（AC-16 已修）
- Partial: —
- Gaps: —
- Unclear: —
- Orphans: 1（已在 ARCH 記錄）

> 靜態一致性稽核：以 PRD 驗收條件為準判斷測試斷言與程式路徑，不以整包測試綠燈為依據。
