# Contract Traceability Matrix — market-data-writes-require-sign-in

Contract: PRD.md (Section 3 Gherkin AC, Section 4 Core Business Rules, Section 6 NFR)
Design map: ARCH.md (Section 7 Traceability)
Implementation: branch `fix/market-data-writes-require-sign-in` vs `main`
Oracle: Acceptance Criteria (24 clauses: 18 AC, 4 BR, 2 NFR)

Ceiling: static conformance audit. Oracles were derived from the PRD before reading code. The mapped
tests were run once as corroboration and pass. The route test runs the real `registerRoutes` without a
database, so "a handler was reached" shows up as a recovered 500 rather than a 401.

Path legend:

- `DEP` = `cmd/server/dependencies.go`
- `MW` = `internal/controller/middlewares/authentication_middleware.go`
- `T-ROUTE` = `cmd/server/market_data_sign_in_test.go`
- `T-KC` / `T-BF` / `T-HS` / `T-CT` / `T-TS` = `internal/controller/tests/{k_candle_controller,k_candle_backfill_controller,k_candle_history_sync_controller,contract_controllers,trading_symbol_controller}_test.go`
- `T-MW` = `internal/controller/middlewares/tests/authentication_middleware_test.go`

Controller fixtures mount every change route behind `doorOpenFor` (the real middleware resolving an
activated user), exactly as `DEP` does; strict gomock mocks with no expectation fail the test if a
refused request reaches storage or the market source.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01-1 | 已開通的使用者新增一根 K 線 | 存下並照今天的回覆 | DEP:92 | T-KC:86 (success through the door, body pinned) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-2 | 沒有登入就新增 K 線 | 拒絕、說要先登入；沒存 | DEP:92, MW | T-ROUTE:50; T-KC:585 (401 + 「請重新登入」, no Save expected) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-3 | 帶著過期的身分證明刪除 K 線 | 拒絕、說要先登入；那根仍在 | DEP:97, MW | T-ROUTE:50 (real expired token → 401; handler not reached) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-4 | 待開通的使用者修改 K 線 | 拒絕、附開通指示，不是請先登入；沒改 | DEP:96, MW:32-40 | T-ROUTE:50 proves the route runs `requiresSignIn`; T-MW:97/107 prove that door answers a pending user with the activation instruction | asserts-oracle (by composition) | produces-oracle | ✅ conforms |
| AC-01-5 | 已開通的使用者手動回補 | 照常回補、回報補進幾根 | DEP:112 | T-BF:70 (through the door, body pinned) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-6 | 沒有登入就手動回補 | 拒絕、說要先登入；沒向來源抓 | DEP:112, MW | T-ROUTE:50; T-BF:136 (no source expectation) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-7 | 已開通的使用者啟動歷史同步 | 收下、回覆輪次 | DEP:120 | T-HS:152 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-8 | 沒有登入就啟動歷史同步 | 拒絕、說要先登入；沒記輪次 | DEP:120, MW | T-ROUTE:50; T-HS:379 "starting records no run" (no run-repository expectation) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-9 | 沒有登入就查歷史同步進度 | 拒絕、說要先登入 | DEP:121, MW | T-ROUTE:50; T-HS:379 "asking for progress" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-10 | 已開通的使用者查歷史同步進度 | 回覆已完成幾段、共幾段 | DEP:121 | T-HS:167 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-11 | 已開通的使用者把標的加進觀察清單 | ETHUSDT 變成追蹤中 | DEP:138 | T-TS:184 (saved as watched, 204) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-12 | 沒有登入就從觀察清單移除 | 拒絕、說要先登入；仍在清單上 | DEP:139, MW | T-ROUTE:50; T-TS:145 "removing" (no lookup/save expectation) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-13 | 合約那邊的改動與現貨一模一樣（4 列 Examples） | 8 條合約改動全部拒絕、說要先登入 | DEP:219,222,231,233,237,239,280,281 | T-ROUTE:50 (8 contract rows, missing and expired proof); T-CT:677; positive paths T-CT:156/299/368/452/489 through the door | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-1 | 沒有登入照常查 K 線 | 照常回答 | DEP:93-95 (no door) | T-ROUTE:86 (not refused); T-KC:128/185/321 answer with no door mounted | asserts-oracle (by composition) | produces-oracle | ✅ conforms |
| AC-02-2 | 沒有登入照常跟盤 | 照常開始跟盤 | DEP:448-449 (no door) | T-ROUTE:86; existing live-follow tests stream with no header | asserts-oracle (by composition) | produces-oracle | ✅ conforms |
| AC-02-3 | 沒有登入照常查交易標的清單 | 照常回答，含追蹤中 | DEP:136, 278 (no door) | T-ROUTE:86; T-TS:77 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-4 | 沒有登入照常查合約補充資料 | 照常回答 | DEP:220-221,235,246-256 (no door) | T-ROUTE:86 | asserts-oracle (not refused; answers covered by each controller's existing tests) | produces-oracle | ✅ conforms |
| AC-02-5 | 帶著壞掉的身分證明看行情 | 照常回答，不因憑證被擋 | no door on reads | T-ROUTE:86 (real expired token, 13 reads never 401) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 改動行情＝16 件事，每一件先確認已開通的目前登入者 | 16 條路由都掛 `requiresSignIn` | DEP (16 sites above) | T-ROUTE:50 table of all 16 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 查歷史同步進度歸在改這邊 | 兩條進度查詢要登入 | DEP:121,233 | T-ROUTE:50; T-HS:379; T-CT:677 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 拒絕的說法沿用既有兩句 | 401「請重新登入」／403 開通指示 | MW (unchanged) | T-KC:585 message; T-MW | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 已開通的使用者之間沒有差別 | handler 不讀目前登入者 | market-data handlers unchanged | — (absence; no handler calls `CurrentUserID`) | asserts-oracle (by inspection) | produces-oracle | ✅ conforms |
| NFR-1 | 改動要登入且已開通；看不必登入 | 同 BR-1 / AC-02-* | DEP | T-ROUTE | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 看行情不因這道門多一次檢查 | 讀路由不經過 middleware | DEP (reads registered without the door) | T-ROUTE:86 with an expired proof | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans

| Behavior | Location | Classification |
|----|----|----|
| none | — | — |

Out-of-scope check: no roles, no audit trail of who changed a candle, scheduled ingestion and live-follow
storage untouched, live follow not rate-limited, watchlist still shared — none implemented.

## Summary

- Found during the audit and fixed before this matrix was final: AC-01-11 was 🟡 partial (only removal
  was proven through the door); a controller test now adds a symbol as an activated user.
- ✅ 24 conforms · 🔴 0 · 🟠 0 · 🟡 0 · ❌ 0 · ❔ 0 · ⚠️ 0
- Conformance: 100%
