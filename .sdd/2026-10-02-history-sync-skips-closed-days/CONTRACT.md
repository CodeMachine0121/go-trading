# Contract Traceability Matrix — 歷史同步跳過休市日

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/service/k_candle_ingestion_service.go`, `internal/domain/service/k_candle_history_sync_runner.go`, `internal/infrastructure/marketdata/fugle_market_data_proxy.go`, `internal/domain/models/{domains,dto,entities}/`
Oracle: Acceptance Criteria + Core Business Rules + NFR (18 clauses)

Test files: `internal/domain/service/tests/k_candle_ingestion_service_test.go`（下稱 svc）、`internal/infrastructure/marketdata/tests/fugle_market_data_proxy_test.go`（下稱 fugle）、`internal/infrastructure/persistence/tests/k_candle_history_sync_run_repository_test.go`（下稱 repo）。

Bridge（UL-MAP / ARCH）：「推定休市天數」= `PresumedClosedDayCount`；「來源拒絕原因」= `FetchFailureReason`；「系統自己的失敗原因」= `FailureReason`；「完成 / 失敗」= `succeeded` / `failed`；「來源說沒有這個標的的資料」= `domains.ErrMarketDataNotHeld`（Fugle 的 404）。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 中間有一天沒資料，跳過它繼續補完（2023/10/06 五、10/09 一、10/10 二沒資料、10/11 三） | 三個有回答的平日存下、10/10 不存；週末不問也不算；完成、推定休市 1、無拒絕原因 | `k_candle_ingestion_service.go:243` | svc `TestSyncingHistorySkipsAWeekdayHolidayBetweenTradingDays`（初版只有 svc `TestSyncingHistoryCarriesOnPastADayTheSourceHoldsNothingFor/a day in the middle`，用全天候市場、沒有週末） | asserts-oracle（初版 shallow：週末夾在中間的部分沒被斷言） | produces-oracle | ✅ conforms（已修） |
| AC-2 | 長區間裡很多天沒資料，每一天都被跳過 | 每一個有回答的平日都存下；推定休市天數＝沒資料的平日數；完成、無拒絕原因 | `k_candle_ingestion_service.go:243` | svc `…CarriesOnPastADay…/several days` | asserts-oracle（以 3 天代表多天；規則逐段判、與天數無關） | produces-oracle | ✅ conforms |
| AC-3 | 第一天就沒資料，從第二天接著走 | 第二天起都存下；完成、推定休市 1 | 同上 | svc `…/the first day` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 最後一天沒資料 | 其餘平日都存下；完成、推定休市 1 | 同上；`fugle_market_data_proxy.go:133`（今天走 intraday 位址也翻成沒資料） | svc `…/the last day`；fugle `TestFugleSaysWhenItHoldsNothingForTheDayAskedAbout/today` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 整段每一天都沒資料 | 不存任何 K 線；完成、推定休市＝整段平日數、無拒絕原因 | 同上 | svc `…/every day` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 沒有任何一天沒資料時與今天相同 | 每天都存下；完成、推定休市 0 | 同上 | svc `…/no day` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 來源拒絕時停下，前面已存的保留（請求太多／自己故障／連不上） | 前 2 天存下；第 3 天之後不再問；完成、拒絕原因說明該拒絕；推定休市 0 | `k_candle_ingestion_service.go:250-256`；`fugle_market_data_proxy.go:138-140` | svc `TestSyncingHistoryStillGivesUpOnASourceThatRefuses`；fugle `TestFugleReportsASourceThatWillNotAnswer`、`TestFugleReportsASourceItCannotReach`（斷言不是「沒資料」） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 先遇到沒資料、後遇到拒絕 | 第 1、3、4 天存下；第 5 天之後不問；完成、推定休市 1、拒絕原因說明請求太多 | 同上 | svc `TestSyncingHistoryKeepsTheClosedDaysItFoundBeforeASourceRefused` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 兩件事各記各的 | 推定休市 1、略過 2、存下 268 | `k_candle_symbol_ingestion_report_domain.go` | svc `TestSyncingHistoryCountsClosedDaysApartFromSkippedKCandles` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 上一趟推定休市的那一天，下一趟會再問一次 | 第二趟再問那天；仍沒資料則第二趟也推定 1 | 無長期記錄（設計使然） | svc `TestSyncingHistoryAsksAgainAboutADayItPresumedClosedLastTime` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 已齊全的一天不問、也不算推定休市 | 那天不問；不算進推定休市 | `k_candle_ingestion_service.go:227-239` | svc `TestSyncingHistoryDoesNotCountADayItAlreadyHoldsAsClosed`（齊全那天若被算進去會是 3 不是 2） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 推定休市之後存不進去 | 失敗、寫系統失敗原因；推定休市 1、已存根數保留 | `k_candle_history_sync_runner.go:86`（ending 經 `progressedRun`） | svc `TestSyncingHistoryEndsAsFailedWhenStorageBreaksAfterAClosedDay` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 三種回答、三種下場 | 有回答→照判定存；沒資料→跳過繼續；其他→放棄 | `k_candle_ingestion_service.go:240-256` | AC-1～AC-8 的測試合起來 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 只有明確的「沒有這份資料」才算推定休市 | 429、500、看不懂、連不上都不是沒資料 | `fugle_market_data_proxy.go:131-140` | fugle 上述三支（`NotErrorIs`） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 推定休市天數：每天至多一次、從零開始、進行中隨走過的天數增加 | 進行中寫入的天數依序 0,0,1,1,2 | `k_candle_history_sync_runner.go:75-94` | svc `TestSyncingHistorySaysHowManyDaysItPresumedClosedWhileStillGoing` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 不改變：已齊全不問、週末/時段外不問、一趟一標的、同時進行上限、先回輪次、重啟收尾 | 既有行為不變 | 未動 | 既有測試全綠（含 Postgres 持久化測試） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 跳過一天不額外向來源發請求 | 問的次數＝走過的段數 | `k_candle_ingestion_service.go:243-248` | svc `…CarriesOnPastADay…`（`source.asked() == 5`） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 既有輪次推定休市天數讀作 0 | 舊列讀作 0；新欄位會存會讀 | `k_candle_history_sync_run.go:24`（`not null;default:0`） | repo `TestSavingAHistorySyncAgainMovesItAlongInsteadOfMakingASecondOne`（存讀）；舊列補 0 是 ORM 的 schema 同步行為，依 testing 規則不另測 | asserts-oracle（存讀部分） | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `fugle_market_data_proxy.go:133` | 每分鐘例行抓取遇到同一個 404 時，`fetchFailureReason` 文字多了 `market data not held by source:` 前綴；行為（仍算來源不答話、不推定休市）不變 | undocumented，無害：只是錯誤訊息更明確；例行抓取在範圍外、規則未改 |
| `dto/k_candle_ingestion_report_dto.go` `PresumedClosedDayCount json:"-"` | 這個數字只在歷史同步輪次上對外呈現；例行抓取與手動回補的回覆不出現它 | 符合 PRD「其他路徑不在這一刀裡」 |

## Summary

- Conforms: 18/18 clauses ✅ (100%)（修正前 17/18：AC-1 🟠 mis-asserted）
- Violations: —
- Mis-asserted: —（AC-1 已補台股真實日曆的測試）
- Partial: —
- Gaps: —
- Unclear: —
- Orphans: 2（皆無害，見上）

Note: static conformance audit against the Acceptance Criteria — it judges test assertions and code paths against the spec's expected outcome, not by running the full suite.
