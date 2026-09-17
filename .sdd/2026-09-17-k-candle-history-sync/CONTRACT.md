# Contract Traceability Matrix — k-candle-history-sync

Contract: `.sdd/2026-09-17-k-candle-history-sync/PRD.md` (v2.0)
Design map: `.sdd/2026-09-17-k-candle-history-sync/ARCH.md`
Implementation: `internal/domain/models/domains/k_candle_history_lookback_domain.go`,
`internal/domain/models/domains/k_candle_ingestion_domain.go` (`HistoryWindow`,
`HistoryChunks`), `internal/domain/models/domains/market_domain.go`
(`TradingKCandleCountBetween`, `Zone`),
`internal/domain/models/entities/k_candle_history_sync_run.go`,
`internal/domain/models/vo/k_candle_history_sync_run_status_vo.go`,
`internal/domain/models/dto/k_candle_history_sync_dto.go`,
`internal/domain/models/dto/k_candle_history_sync_run_dto.go`,
`internal/domain/interface/i_k_candle_history_sync_run_repository.go`,
`internal/domain/service/k_candle_ingestion_service.go` (`StartHistorySyncFor`,
`GetHistorySyncRun`, `FailInterruptedHistorySyncs`, `syncSymbolHistory`, `judge`,
`noteSkipped`, `reachSymbolOnDemand`),
`internal/domain/service/k_candle_history_sync_runner.go`,
`internal/application/k_candle_ingestion_application.go`,
`internal/controller/k_candle_history_sync_controller.go`,
`internal/controller/models/k_candle_history_sync_request.go`,
`internal/infrastructure/persistence/k_candle_repository.go` (`SaveIfAbsent`,
`SaveAllIfAbsent`, `CountInRange`),
`internal/infrastructure/persistence/k_candle_history_sync_run_repository.go`,
`internal/infrastructure/marketdata/request_pacer.go`,
`internal/infrastructure/marketdata/binance_market_data_proxy.go`,
`internal/infrastructure/marketdata/fugle_market_data_proxy.go`,
`internal/config/application_config.go`, `cmd/server/dependencies.go`, `cmd/server/main.go`
Oracle: Acceptance Criteria (28 clauses) + Core Business Rules (15 clauses)

Test files below are abbreviated:
`look` = `internal/domain/models/domains/tests/k_candle_history_lookback_domain_test.go`,
`chunk` = `internal/domain/models/domains/tests/k_candle_history_chunks_test.go`,
`svc` = `internal/domain/service/tests/k_candle_ingestion_service_test.go`,
`ctrl` = `internal/controller/tests/k_candle_history_sync_controller_test.go`,
`cfg` = `internal/config/tests/ingestion_config_test.go`,
`repo` = `internal/infrastructure/persistence/tests/k_candle_repository_test.go`,
`runrepo` = `internal/infrastructure/persistence/tests/k_candle_history_sync_run_repository_test.go`,
`fugle` = `internal/infrastructure/marketdata/tests/fugle_market_data_proxy_test.go`,
`binance` = `internal/infrastructure/marketdata/tests/binance_market_data_proxy_test.go`,
`routes` = `cmd/server/routes_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 抓回指定的那一段 | 那一段的一分鐘 K 線被抓回來並存下 | `HistoryChunks` + `syncSymbolHistory` | `svc:1231`（每一段接起來剛好是那一整段）, `ctrl:156` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 已經有的那幾根原封不動 | 缺的補進去，既有的一根都沒被寫過 | `HistoryWindow` 不讀既有資料 + `SaveAllIfAbsent` | `svc:1533`, `repo:536`, `repo:637` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2b | 中間的破洞補得起來 | 中間那幾天補起來 | 起點不受既有資料影響 | `repo:570`（洞被補上，兩側原封不動）, `svc:1339` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2c | 本來就齊全的那一段連問都不問 | 那幾段沒向來源問過，存 0 根 | `CountInRange` vs `TradingKCandleCountBetween` | `svc:1364`, `chunk:97` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2d | 其餘三條路照樣覆蓋 | 定時那一輪仍然蓋掉自己先前收的 | 歷史同步不走 `ingestSymbols`（結構上分開） | `svc:1578`（斷言它走的是 `Save` 而不是 `SaveAllIfAbsent`） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 粒度固定一分鐘 | 沒有任何地方可以指定粗細 | `KCandleHistorySyncRequest` 無該欄位 | `ctrl:360` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 沒登錄過的代號 | 被拒絕，並說這個代號沒有登錄 | `reachSymbolOnDemand` | `svc:1623`, `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 代號空白 | 被拒絕，說那不是一個代號 | 同上 | `svc:1637`（並斷言不是「沒登錄」那一種）, `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 回溯天數超過上限 | 被拒絕，句子裡看得到上限 | `NewKCandleHistoryLookbackDomain` | `look:60`, `svc:1595`, `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 回溯天數不是正數 | 被拒絕 | 同上 | `look:39`, `svc:1595`, `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 沒給回溯天數 | 被拒絕 | 請求物件無預設值，缺漏即 0 | `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 剛好一天 | 照做 | `NewKCandleHistoryLookbackDomain` | `look:16` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 剛好是上限 | 照做 | 同上 | `look:16` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 抓得動四年 | 切成一天一段，抓一段存一段 | `HistoryChunks` + 逐段 `SaveAllIfAbsent` | `chunk:41`, `chunk:67`, `svc:1231`, `svc:1407`（前半段已存好才問下一段） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 市場那幾天沒開不算失敗 | 不算失敗，且與「來源不答話」分得開 | `ClampToTradingSession` 把那幾段清空 | `svc:1738`（整段落在週末：來源根本沒被問，`fetchFailureReason` 是空的） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 來源連不上寫進輪次，不是這一趟失敗 | 輪次仍 `succeeded`，帶著 `fetchFailureReason` | `syncSymbolHistory` 記下後 `break` | `svc:1665`, `svc:1407`, `ctrl:253`（「來源不答話還是被收下」那一條） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13b | 這個系統自己壞掉才是這一趟失敗 | 輪次 `failed`，`failureReason` 有話、`fetchFailureReason` 沒有 | `recordEnding` | `svc:1487` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13c | 連輪次都記不下來就整個拒絕 | 502，什麼都沒開始 | `StartHistorySyncFor` 的 `Save` 失敗分支 | `svc:1515`（並斷言來源一次都沒被問）, `ctrl:253` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 不在觀察清單上的也同步得動 | 照做 | `reachSymbolOnDemand` 走 `FindBySymbol` | `svc:1648` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 上限由環境變數決定，非正整數退回預設 | 預設 3650；0／負／讀不出來退回 | `application_config.go` | `cfg:169` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 送出就收到一筆輪次 | 202 + `running` + 總段數 | `StartHistorySyncFor` → `202` | `svc:1283`（在來源還被擋在門外時就回來了）, `ctrl:156` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 回來看它走到哪 | 第幾段／共幾段、存了幾根、跳過幾根 | `GetHistorySyncRun` | `svc:1765`, `ctrl:172` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 收尾 | `succeeded` 且有收尾時間 | `recordEnding` | `svc:1283`（斷言收尾時間有被寫上）, `ctrl:172` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 沒有那筆輪次 | 404 | `ErrKCandleHistorySyncRunNotFound` | `svc:1783`, `ctrl:192` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 抓到一半服務重啟 | `failed`＋`interrupted by restart` | `FailAllRunning` + `main.go` 啟動那一段 | `svc:1793`, `runrepo:93`（打真的 PostgreSQL：只掃 `running`，已收尾的不動） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 交代它做了什麼 | 跳過的那幾根算進 `skippedCount` | `judge` + `noteSkipped` | `svc:1457` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 進度一段一段往前 | 不是只在收尾才出現 | `recordProgress` 每段一次 | `svc:1316`（逐次寫下的段數依序遞增） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 抓到一半來源不答話 | 前面的已存，剩下的不再問 | `syncSymbolHistory` 的 `break` | `svc:1407` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 粒度固定，請求裡沒有地方指定 | 同 AC-3 | 請求物件 | `ctrl:360` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 窗口起點只由回溯天數決定 | 同 AC-2／AC-2b | `HistoryWindow` | `svc:1231`, `svc:1339`（並斷言 `FindLatest` 一次都沒被呼叫）, `ctrl:239` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2b | 寫的時候不覆蓋，判斷交給資料庫 | 已經有的原封不動 | `ON CONFLICT DO NOTHING` | `repo:536`, `repo:556`, `repo:570`, `repo:637`, `repo:662`（五條都打真的 PostgreSQL） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2c | 「存了幾根」是這一次新增的 | 齊全時為 0 | `SaveAllIfAbsent` 回報寫了幾筆 | `svc:1533` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 1..上限，超過即說出上限 | 同 AC-6／AC-7 | `NewKCandleHistoryLookbackDomain` | `look:16`, `look:39`, `look:60` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 沒登錄的拒絕，不猜市場 | 同 AC-4 | `reachSymbolOnDemand` | `svc:1623` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 不推定休市，也不因推定過而略過 | 已經判過休市，手動同步照樣去問來源 | `reachSymbolOnDemand` 的 `reconsider` | `svc:1690`（先讓一輪把市場判成休市，再從歷史同步進去，斷言來源真的被問到） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 不碰觀察清單 | 同 AC-14 | `reachSymbolOnDemand` | `svc:1648` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 這一趟失敗只留給系統自己壞掉 | 同 AC-13／AC-13b | `recordEnding` | `svc:1487`, `svc:1665` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 一天一段，抓一段存一段 | 同 AC-11 | `HistoryChunks` | `chunk:41`, `chunk:67`, `svc:1407` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 已齊全就不問，且只決定「要不要問」 | 同 AC-2c | `CountInRange` | `svc:1364`, `svc:1339`（起點仍不受既有資料影響） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 一段問不到就停掉剩下的 | 同 AC-23 | `break` | `svc:1407`（斷言只問了兩次，沒有把剩下的段都撞上去） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | 先記輪次再回答；記不下來就整個拒絕 | 同 AC-16／AC-13c | `StartHistorySyncFor` | `svc:1283`, `svc:1515` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | 進度以段數表示，每段更新一次 | 同 AC-22 | `recordProgress` | `svc:1316`, `runrepo:44`（同一筆被改寫，不是長出第二筆）, `runrepo:63`（歸零也寫得回去） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-14 | 啟動時掃殘留 `running` | 同 AC-20 | `FailAllRunning` | `svc:1793`, `runrepo:93` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-15 | 每個來源都有每分鐘請求數上限 | 兩次請求之間不快於來源允許的節奏 | `requestPacer`，兩個 proxy 各一個 | `binance:125`（實際量兩次請求之間的間隔） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `fugle_market_data_proxy.go` 跳過休市日 | 這個來源一天打一次，四分之一的日子是一定空的週末 | 這個切片**造成**的成本，不修就會在正式環境被來源擋下。已補測試 `fugle:459` |
| `noteSkipped` 的 200 筆上限 | 一個答了四年垃圾的來源會把報告變成沒人打得開的東西 | 規格已載明（PRD Edge Cases）；`skippedCount` 照實算，由 `svc:1457` 釘住 |
| `reachSymbolOnDemand` | 兩個手動用例共用的前幾步 | 重構產物而非新行為；「兩個以上才留私有 helper」門檻滿足 |
| `judge` | 歷史同步與其餘三條路共用的「哪幾根可以存」 | 同上：什麼樣的 K 線可以存，與它為什麼被抓來無關 |
| `MarketDomain.Zone()` | Fugle proxy 要說得出「本地的哪一天」 | 為了讓 proxy 不再自己複製一份市場規則；由 `fugle:459` 連帶覆蓋 |
| `routes_test.go` 多兩行 | 掛上去的路由就是清單上那些 | 既有的守門測試，非孤兒 |

## Summary

- Conforms: 43/43 clauses ✅ (100%)
- Violations: 無
- Mis-asserted: 無
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 0

**v2.0 的行為改動**（原本是「同步回答、整段一次抓、上限九十天」）：
新增 AC-11（抓得動四年）、AC-13c（輪次記不下來）、AC-16 到 AC-23（不等抓完、
可查進度、重啟掃殘留、逐段收尾），以及 BR-9 到 BR-15。AC-2c 從「齊全時存 0 根」
改寫成更強的「齊全時連問都不問」。AC-13 的期望從 `200 + 報告` 改成
`202 + 輪次仍 succeeded`——來源不答話是這一趟查到的事，那件事沒變，
變的是它寫在哪裡。

AC-2d 現在是**結構性**的：歷史同步不走 `ingestSymbols`，所以沒有旗標可以被傳錯；
那條測試從「斷言旗標值」變成「斷言定時那一輪走的是 `Save`」。

> 稽核性質：靜態一致性稽核。它比對測試斷言與程式路徑對上規格的預期結果，
> 不執行自行發明的情境，也不以整份測試套件的綠燈作為判準。
