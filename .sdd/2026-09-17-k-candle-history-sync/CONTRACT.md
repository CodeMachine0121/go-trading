# Contract Traceability Matrix — k-candle-history-sync

Contract: `.sdd/2026-09-17-k-candle-history-sync/PRD.md`
Design map: `.sdd/2026-09-17-k-candle-history-sync/ARCH.md`
Implementation: `internal/domain/models/domains/k_candle_history_lookback_domain.go`,
`internal/domain/models/domains/k_candle_ingestion_domain.go` (`HistoryWindow`),
`internal/domain/models/dto/k_candle_history_sync_dto.go`,
`internal/domain/service/k_candle_ingestion_service.go` (`SyncHistoryFor`,
`reachSymbolOnDemand`), `internal/application/k_candle_ingestion_application.go`,
`internal/controller/k_candle_history_sync_controller.go`,
`internal/controller/models/k_candle_history_sync_request.go`,
`internal/config/application_config.go`, `cmd/server/dependencies.go`,
`internal/infrastructure/persistence/k_candle_repository.go` (`SaveIfAbsent`),
`internal/infrastructure/marketdata/fugle_market_data_proxy.go`
Oracle: Acceptance Criteria (18 clauses) + Core Business Rules (9 clauses)

Test files below are abbreviated:
`look` = `internal/domain/models/domains/tests/k_candle_history_lookback_domain_test.go`,
`svc` = `internal/domain/service/tests/k_candle_ingestion_service_test.go`,
`ctrl` = `internal/controller/tests/k_candle_history_sync_controller_test.go`,
`cfg` = `internal/config/tests/ingestion_config_test.go`,
`repo` = `internal/infrastructure/persistence/tests/k_candle_repository_test.go`,
`fugle` = `internal/infrastructure/marketdata/tests/fugle_market_data_proxy_test.go`,
`routes` = `cmd/server/routes_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 抓回指定的那一段 | 那 30 天的一分鐘 K 線被抓回來並存下，報告說出存了幾根 | `SyncHistoryFor` + `HistoryWindow` | `svc:1145`, `ctrl:84` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 已經有的那幾根原封不動 | 整段都向來源問過，缺的補進去，既有的一根都沒被寫過 | `HistoryWindow` 不讀既有資料 + `keepStoredKCandles`／`SaveIfAbsent` | `svc:1169`（`FindLatest` 一次都沒被呼叫）, `svc:1197`（只有缺的那一根被寫）, `repo:534` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2b | 中間的破洞補得起來 | 中間那幾天補起來 | 同上——起點不受既有資料影響，所以問得到中間 | `repo:568`（洞被補上，兩側原封不動）, `svc:1169` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2c | 本來就齊全時什麼都不做 | 報告說存了 0 根，既有的沒被動到 | `SaveIfAbsent` 回報沒寫，`StoredCount` 不加 | `svc:1197`, `repo:534` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2d | 其餘三條路照樣覆蓋 | 定時那一輪仍然蓋掉自己先前收的 | `replaceStoredKCandles` 明寫在三個呼叫處 | `svc:1232`（斷言它走的是 `Save` 而不是 `SaveIfAbsent`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 粒度固定一分鐘 | 沒有任何地方可以指定粗細 | `KCandleHistorySyncRequest` 無該欄位 | `ctrl:221` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 沒登錄過的代號 | 被拒絕，並說這個代號沒有登錄 | `reachSymbolOnDemand` | `svc:1206`, `ctrl:115` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 代號空白 | 被拒絕，說那不是一個代號 | 同上 | `svc:1219`（並斷言不是「沒登錄」那一種）, `ctrl:115` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 回溯天數超過上限 | 被拒絕，句子裡看得到上限 | `NewKCandleHistoryLookbackDomain` | `look:60`, `svc:1180`, `ctrl:115` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 回溯天數不是正數 | 被拒絕 | 同上 | `look:39`, `svc:1180`, `ctrl:115` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 沒給回溯天數 | 被拒絕 | 請求物件無預設值，缺漏即 0，由上面那條擋下 | `ctrl:115` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 剛好一天 | 照做 | `NewKCandleHistoryLookbackDomain` | `look:16` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 剛好是上限 | 照做 | 同上 | `look:16` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 報告與既有回補一字不差 | 列出壞掉的那一根與原因，其餘照存 | 共用 `ingestSymbols`，沒有第二種報告 | `svc:1245` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 市場那幾天沒開不算失敗 | 不算失敗，且與「來源不答話」分得開 | 既有 `ingestSymbol` 路徑 | `svc:1333`（整段落在週末：來源根本沒被問，`fetchFailureReason` 是空的）, `ctrl:84` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 來源連不上 | 回「稍後再試」那一類 | `ingestSymbol` 寫進報告；controller 只對映系統自己的故障 | `svc:1271`, `ctrl:115`（兩條：報告裡的來源失敗，與 502 的儲存故障） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 不在觀察清單上的也同步得動 | 照做 | `reachSymbolOnDemand` 走 `FindBySymbol` 而非觀察清單 | `svc:1229` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 上限由環境變數決定，非正整數退回預設 | 預設 90；0／負／讀不出來退回 | `application_config.go` | `cfg:169` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 粒度固定，請求裡沒有地方指定 | 同 AC-3 | 請求物件 | `ctrl:221` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 窗口起點只由回溯天數決定 | 同 AC-2／AC-2b | `HistoryWindow` | `svc:1145`, `svc:1169`, `ctrl:96` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2b | 寫的時候不覆蓋，判斷交給資料庫 | 已經有的原封不動 | `SaveIfAbsent` 走 `ON CONFLICT DO NOTHING` | `repo:534`, `repo:554`, `repo:568`（三條都打真的 PostgreSQL） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2c | 「存了幾根」是這一次新增的 | 齊全時為 0 | `store` 回報有沒有寫，`StoredCount` 才加 | `svc:1197` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 1..上限，超過即說出上限 | 同 AC-6／AC-7 | `NewKCandleHistoryLookbackDomain` | `look:16`, `look:39`, `look:60` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 沒登錄的拒絕，不猜市場 | 同 AC-4 | `reachSymbolOnDemand` | `svc:1206` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 不推定休市，也不因推定過而略過 | 已經判過休市，手動同步照樣去問來源 | `reachSymbolOnDemand` 的 `reconsider`（與 `RunBackfillFor` 共用） | `svc:1288`（先讓一輪把市場判成休市，再從歷史同步進去，斷言來源真的被問到）, `svc:1333` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 不碰觀察清單 | 同 AC-14 | `reachSymbolOnDemand` | `svc:1229` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 報告形狀與既有回補完全相同 | 同 AC-11 | 共用 `ingestSymbols` | `svc:1245`, `ctrl:84` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `fugle_market_data_proxy.go` 跳過休市日 | 這個來源一天打一次，而把窗口收進交易時段只動得到頭尾；九十天等於九十次帶金鑰的循序請求，四分之一是一定空的週末 | 這個切片**造成**的成本，不修就會在正式環境被來源擋下。ARCH 未載明；已補測試 `fugle:454` |
| `k_candle_ingestion_service.go` `reachSymbolOnDemand` | 兩個手動用例共用的前四步 | 重構產物而非新行為；`architecture.md` 的「兩個以上才留私有 helper」門檻剛好滿足 |
| `HistoryWindow` 起點往下對齊到刻度邊界 | 起點落在格子中間會讓最舊那一格只有半格的資料，卻被當成一整格交出去 | 與 `BackfillWindow` 同一條既有規則，程式碼內指向它；PRD「往下對齊」已載明 |
| `routes_test.go` 多一行 | 掛上去的路由就是清單上那些 | 既有的守門測試，非孤兒 |

## Summary

- Conforms: 27/27 clauses ✅ (100%)
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 0

本次稽核修補的缺口：AC-12 原本只走完「來源答了空的、回 200」，沒有把「市場休市」
與「來源就是沒東西」分開，而規格想講的正是前者；BR-5 則完全靠「它與手動回補是同一段
程式碼」推論，沒有一條測試從歷史同步這條路進去。已補上兩條——一條讓整段落在週末、
斷言來源根本沒被問而且不算失敗，一條先讓一輪把市場判成休市、再從歷史同步進去，
斷言它照樣去問了來源。

**行為改動之後的重新稽核**（原本是「整段重抓並覆蓋」，改成「問整段、只寫沒有的」）：
AC-2 整條重寫，並拆出 AC-2b（破洞補得起來）、AC-2c（齊全時什麼都不做）、
AC-2d（其餘三條路照樣覆蓋——那一條是防止這條寫入規則被誤用到自動輪次的守門）。
不覆蓋那一條由三條打真的 PostgreSQL 的測試釘住，因為「同一個開盤時刻不寫」
是資料庫在保證的，不是程式碼在保證的。

> 稽核性質：靜態一致性稽核。它比對測試斷言與程式路徑對上規格的預期結果，
> 不執行自行發明的情境，也不以整份測試套件的綠燈作為判準。
