# Contract Traceability Matrix — contract-position-statistic-history-sync

Contract: PRD.md (Section 3 Gherkin AC, Section 4 Core Business Rules, Section 6 NFR)
Design map: ARCH.md (Section 7 Traceability)
Implementation: branch `feat/contract-position-statistic-history-sync` vs `main`
Oracle: Acceptance Criteria (44 clauses: 29 AC, 11 BR, 4 NFR)

Ceiling: static conformance audit. Oracles were derived from the PRD before reading code. The mapped
tests (service / domains / marketdata proxy / entity) were run once as corroboration and pass; the
persistence tests skip without `TEST_POSTGRES_DSN` and were **not executed** — clauses that rest on
them are judged by reading only.

Path legend:

- `SVC` = `internal/domain/service/contract_position_statistic_service.go`
- `RUN` = `internal/domain/service/contract_k_candle_history_sync_runner.go`
- `ING` = `internal/domain/service/contract_k_candle_ingestion_service.go`
- `ARD` = `internal/domain/models/domains/contract_position_statistic_archive_domain.go`
- `HSD` = `internal/domain/models/domains/contract_position_statistic_history_domain.go`
- `PSD` = `internal/domain/models/domains/contract_position_statistic_domain.go` (pre-existing live rules)
- `PRX` = `internal/infrastructure/marketdata/binance_contract_position_statistic_archive_proxy.go`
- `REPO` = `internal/infrastructure/persistence/contract_position_statistic_repository.go`
- `ENT` = `internal/domain/models/entities/k_candle_contract_history_sync_run.go`
- `T-SYNC` = `internal/domain/service/tests/contract_position_statistic_history_sync_test.go`
- `T-ARD` / `T-HSD` = `internal/domain/models/domains/tests/contract_position_statistic_{archive,history}_domain_test.go`
- `T-PRX` = `internal/infrastructure/marketdata/tests/binance_contract_position_statistic_archive_proxy_test.go`
- `T-ENT` = `internal/domain/models/entities/tests/k_candle_contract_history_sync_run_test.go`
- `T-REPO` = `internal/infrastructure/persistence/tests/contract_position_statistic_repository_test.go` (Postgres-only, skipped locally)

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01-1 | 回溯 180 天,持倉統計的前一百五十天從歷史資料庫補進來 — Given 只有最近三十天 When 同步回溯 180 天 Then K 線照既有規則補齊 And 那 180 天裡歷史資料庫有檔案的每一天都補進來 | 同一趟同步後 K 線照舊補齊；範圍內每個有檔案的日子的持倉統計都已存入 | RUN:83-94, SVC:156-222 | T-SYNC:166 (FillsADay…); candle behaviour held by pre-existing candle tests now running with the statistics half attached (`newContractIngestionUnderTest`) | asserts-oracle (by composition: 2-day stretch, day with file stored 288, candle suite unchanged) | produces-oracle | ✅ conforms |
| AC-01-2 | 持倉統計涵蓋的天數跟著回溯天數走 — 今天 2026-09-25,回溯 180 天 → 共 181 天(2026-03-29~2026-09-25) | 輪次顯示持倉統計 181 天，首日 03-29、末日 09-25 | HSD:31-47, ING:161-173 | T-HSD:12; T-SYNC:139 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-3 | 回溯 1 天,持倉統計只涵蓋昨天與今天 → 共 2 天 | 輪次顯示持倉統計 2 天（09-24、09-25） | HSD:31-47 | T-HSD:12; T-SYNC:139 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-4 | 先補合約 K 線,再補持倉統計 — K 線段數還沒走完時,持倉統計走到第 0 天 | K 線未走完前持倉統計進度為 0；持倉統計只在 K 線走完後才開始 | RUN:67-94 | T-SYNC:402 (archive only asked once chunks==total) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-5 | 回溯天數超過上限,整次拒絕 — 3651 天 → 拒絕並說上限 3650,持倉統計也不補,沒有輪次 | 請求被拒且提到 3650；不留輪次；不向歷史資料庫問任何一天 | ING:147-151 | T-SYNC:454 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-1 | 比值 3 換算成多方 0.75、空方 0.25 | 存下多方 0.75、空方 0.25、比值 3 | ARD:70-86 | T-ARD:27; T-SYNC:190-191 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-2 | 比值 1 換算成各佔一半（大戶） | 存下多方 0.5、空方 0.5 | ARD:70-86 | T-ARD:27 (both splits) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-3 | 比值 0 換算成一面倒做空 | 存下多方 0、空方 1 | ARD:82-84 | T-ARD:27 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-4 | 比值為負,那一筆不存,跳過數加一;同一天其他合法的照常存入 | 該筆不存、跳過數 +1、其他合法筆存入 | ARD:77-80, SVC:204-211 | T-ARD:81; T-SYNC:233 (Len(1) saved, skipped 4, stored 1) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-5 | 任何一項缺了(大戶比值空白),那一筆不存,跳過數加一 | 該筆不存、跳過數 +1 | PRX:166-171, PSD:66-69 | T-PRX:131; T-ARD:81; T-SYNC:233 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-6 | 持倉量為負,那一筆不存,跳過數加一 | 該筆不存、跳過數 +1 | PSD:44-47 | T-ARD:81; T-SYNC:233 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-7 | 統計時間不在五分鐘刻度(09:03),那一筆不存,跳過數加一 | 該筆不存、跳過數 +1 | PSD:35-38 | T-ARD:81 (09:03); T-SYNC:233 (off-grid :18) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-1 | 一筆都沒有的那一天,整天補進來(288 筆) | 那一天存了 288 筆 | SVC:185-219 | T-SYNC:166 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-2 | 已存滿一整天,連問都不問;那一天仍算走過一天 | 不問歷史資料庫那一天；走過天數仍計入 | SVC:180-187, HSD:57-59 | T-SYNC:199 (Times(0), completed 2); T-HSD:53 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-3 | 只缺一筆,只補那一筆;其他 287 筆的每一個數字都不變 | 只新增 09:05；既有 287 筆數值不變 | SVC:214-219, REPO:33-51 (ON CONFLICT DO NOTHING) | T-SYNC:216 (mock returns 1 — tautological at service level); T-REPO:31 KeepsTheFirstAnswerForOneMoment (existing row kept, new row added) | asserts-oracle (via T-REPO; not executed here) | produces-oracle | ✅ conforms |
| AC-03-4 | 即時錄下的那一筆不被覆蓋(0.7353 vs 0.73531) | 09:05 仍是 0.7353 | REPO:40-45 | T-REPO:31 (first answer kept on conflict) | asserts-oracle (row-level; asserts OpenInterest, not the share; not executed here) | produces-oracle | ✅ conforms |
| AC-03-5 | 整段本來就齊全,存了 0 筆,輪次成功 | 每一天都已滿 → 不問任何一天、存 0 筆、輪次成功 | SVC:185-187, 222 | T-SYNC:501 (StoresNothingWhenEveryDayIsAlreadyWhole: both days hold 288 → archive Times(0), SaveAllIfAbsent Times(0), stored 0, completed 2, succeeded) — added after the first audit | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-1 | 今天的檔案還沒出現 — 今天什麼都不存,持倉統計沒有來源原因,輪次成功 | 今天存 0、無來源原因、輪次成功 | PRX:90-92, SVC:199-201 | T-PRX:120; T-SYNC:261 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-2 | 回溯到合約上市之前 — 前 100 天沒有檔案,什麼都不存、繼續;上市之後有檔案的照常補 | 無檔案的日子跳過且不停，之後有檔案的日子照常存入 | SVC:199-201 | T-SYNC:261 (next day still asked after a no-file day) + T-SYNC:166 (day with file stored) | asserts-oracle (by composition; no single no-file-then-file sequence) | produces-oracle | ✅ conforms |
| AC-04-3 | 某標的在歷史資料庫完全沒有檔案 — 存 0 筆、走完全部天數,輪次成功,無來源原因 | 存 0、完成天數 = 總天數、成功、無原因 | SVC:199-201, 222 | T-SYNC:261 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-1 | 補到第 40 天時歷史資料庫連不上 — 之後不再問;走到第 39 天,來源原因寫著連不上;輪次成功,前 39 天存下的留著 | 之後的日子不問；進度停在 39；原因在持倉統計那組；輪次成功；已存資料保留 | SVC:189-197 | T-SYNC:281 (failure on day 3 → completed 2, days 4-5 Times(0), success) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-2 | 某一天的檔案內容讀不懂 — 視為不答話:之後不再問,原因寫著讀不懂;輪次成功 | 同 AC-05-1，原因是內容讀不懂 | PRX:98-106, 115-183; SVC:189-197 | T-PRX:164, 223 (unreadable → error); T-SYNC:281 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-3 | 合約 K 線的來源不答話,持倉統計照常補 | K 線原因記在 K 線那組；持倉統計從第一天走到最後一天 | RUN:74-94 | T-SYNC:309 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-4 | 兩個來源的原因分開記 | K 線與持倉統計各自只寫自己的來源原因 | RUN:125-145, ENT:37-44 | T-SYNC:331 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-5 | 系統自己存不進去,輪次失敗,寫著原因;已經存下的留著 | 輪次失敗且有原因；已存資料保留 | SVC:214-218, RUN:90-94 | T-SYNC:352 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-6 | 補到一半服務重啟 — 收在失敗,寫著被重啟中斷;再跑一次時已存滿的那幾天連問都不問 | 重啟後輪次為失敗、原因為被重啟中斷；重跑略過已滿的日子 | ING:212-218 (existing sweep), SVC:185-187 | `internal/infrastructure/persistence/tests/contract_trading_symbol_repository_test.go:157`, `internal/application/tests/contract_k_candle_application_test.go:287`; T-SYNC:199 | asserts-oracle (by composition; sweep test Postgres-only) | produces-oracle | ✅ conforms |
| AC-06-1 | 補持倉統計到第 20 天時查輪次 — K 線段數已走完;持倉統計說走到第 20 天、共幾天 | 查到 K 線已完成、持倉統計 20/總天數 | RUN:113-145, ENT:58-78 | T-SYNC:426 (mid-run progress written), T-SYNC:402 (chunks done), T-SYNC:468 (read-back 20/181) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-2 | 兩種數字不加總 — K 線 1,000 根、持倉統計 500 筆 | 各組各報自己的數字 | ENT:58-78, dto/k_candle_contract_history_sync_run_dto.go | T-ENT:13, 37; T-SYNC:468 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-3 | 這一刀之前結束的輪次 — 持倉統計天數、走到第幾天、筆數、跳過數都是零,沒有來源原因 | 舊輪次的持倉統計那組全為零、無原因 | ENT:37-44 (`not null default`) | T-ENT:30 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 同一個回溯天數:從現在往回推回溯天數那一刻所在的那一天到今天,UTC 日曆日 | 範圍 = UTC(now−lookback) 的日到今天（含） | HSD:34-44, ING:161-163 | T-HSD:12, 43 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 一天一份:以日曆日為單位要;一天最多 288 筆 | 每次請求一天；滿額 288 | PRX:76-77, HSD:57-59 | T-PRX:111; T-HSD:53 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 已齊全就不問:只決定要不要問,不決定從哪天開始 | 滿 288 不問；起點不受已存資料影響 | SVC:180-187 | T-SYNC:199 (day 1 whole, day 2 still walked) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 只存沒有的:已經有的(包括即時錄下的)原封不動 | 不覆蓋既有資料 | REPO:40-45 | T-REPO:31 | asserts-oracle (not executed here) | produces-oracle | ✅ conforms |
| BR-5 | 佔比由比值換算:多方 = r÷(1+r)、空方 = 1÷(1+r);兩組各換一次 | 兩組都按公式換算 | ARD:70-86 | T-ARD:27 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 同一套合法性:換算後適用即時錄製的規則;缺或不合法即跳過並算進跳過數 | 與即時同規則；違規跳過並計數 | ARD:88-91 → PSD:25-98; SVC:204-211 | T-ARD:81 (incl. future time); T-SYNC:233 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 沒有那一天的檔案不是失敗:繼續下一天,不留原因 | 繼續、無原因 | PRX:90-92, SVC:199-201 | T-PRX:120; T-SYNC:261 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 不答話只停這一份:連不上、回錯誤、讀不懂 → 放棄剩下的日子,原因在持倉統計那組;輪次成功 | 同 AC-05-1/2 | PRX:84-106; SVC:189-197 | T-PRX:164, 223, 261; T-SYNC:281 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 兩份互不牽連:K 線來源不答話不影響持倉統計,反之亦然 | 一方的來源原因不影響另一方的進行與原因 | RUN:74-94, 125-145 | T-SYNC:309, 331 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 系統自己壞掉才算失敗:存不進去、讀不到自己的資料 → 輪次失敗 | 儲存寫入或讀取失敗 → 輪次失敗 | SVC:182-184, 216-218; RUN:90-94 | T-SYNC:352 (both sub-cases) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 既有規則照舊:回溯上下限、一標的同時一趟、先回輪次、重啟掃成失敗 | 既有行為不變 | ING:147-190 (unchanged checks, `go` runner), `k_candle_contract_history_sync_run_repository.go:58` | pre-existing contract ingestion / repository tests; T-SYNC:454 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | Performance:一天一份檔案,照歷史資料庫自己的節奏打,不影響即時來源的額度 | 歷史資料庫有自己的節奏，與即時來源額度分開 | `cmd/server/dependencies.go` `cryptoContractArchive` pacer; PRX:72-74 | `cmd/server/venue_pacers_test.go` (NotSame limiters, own setting) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | Memory:一次只處理一天(最多 288 筆),不把整段候在記憶體裡 | 每天處理完即存，不累積整段 | SVC:176-219 (per-day slice, saved per day) | T-SYNC:520 (StoresEachDayOnItsOwnBeforeReadingTheNext: read 09-24 → store 09-24 → read 09-25 → store 09-25) — added after the first audit | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | Security:歷史資料庫是公開的,不需要身分 | 請求不帶任何身分 | PRX:79-84 (plain GET, no headers) | T-PRX:122 (AsksWithoutAnyAccount: no Authorization / X-Mbx-Apikey / Cookie) — added after the first audit | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | Compatibility:既有輪次照查得到;持倉統計那組為零 | 舊輪次可查且持倉統計為零 | ENT:37-44, 58-78 | T-ENT:30 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| RUN:74-79 (T-SYNC:386) | When the candle half fails the run (this system broke on the candles), the statistics half is not run at all; statistics progress stays at 0. PRD never says what happens to the statistics when the candle half fails the run. | undocumented — reconcile with PRD (add a BR) |
| PRX:173-176 | A non-numeric (garbage) figure in any row makes the **whole day** a read error, which then abandons **all remaining days** (SVC:189-197). PRD BR-6 says 「不合法即跳過」 (per row) while BR-8 says 「內容讀不懂」 stops the rest; which one a single bad cell is, is not decided by the spec. | undocumented — ambiguity, reconcile |
| PRX:142-149 vs ARCH §8 | A missing required header column is treated as unreadable (abandon remaining days). This matches PRD §7 risk 「改了就讀不懂——照不答話處理」, but contradicts ARCH §8, which says a renamed column 「會讀成缺值、整天跳過」. | undocumented design drift (ARCH vs code); PRD-conformant |

Out-of-Scope check: no dedicated statistics-only route, no change to the 5-minute round / 30-day catch-up (`RunRound*` untouched), no extra archive columns stored, no funding-rate history, no overwrite, no spot changes — no out-of-scope violations found.

## Summary

- Conforms: 44/44 clauses ✅ (100%) after resolution — first pass 41/44 (93.2%)
- Violations: none
- Mis-asserted (first pass, resolved): AC-03-5 (no test with every day already whole; nearest test doesn't assert run succeeded)
- Partial (first pass, resolved): NFR-2, NFR-3
- Gaps: none
- Unclear: none
- Orphans: 3
- Caveat: AC-03-3, AC-03-4, AC-05-6, BR-4 rest on Postgres-only persistence tests that skip without `TEST_POSTGRES_DSN`; they were judged by reading, not executed. At the service level AC-03-3's test (T-SYNC:216) only proves the mock's return value passes through.


## Resolution (after the first audit)

| Finding | Decision | Change |
|---|---|---|
| AC-03-5 mis-asserted (no whole-stretch test) | fix the test | `TestContractHistorySyncStoresNothingWhenEveryDayIsAlreadyWhole` |
| NFR-2 partial | fix the test | `TestContractHistorySyncStoresEachDayOnItsOwnBeforeReadingTheNext` (also shows today's statistics after the sync time are refused as future: 288 + 121 stored, 167 skipped) |
| NFR-3 partial | fix the test | `TestArchiveProxyAsksWithoutAnyAccount` |
| Orphan: statistics never start when the candle half broke the system | keep — deliberate (ARCH §4 runner row) | PRD §4 now states it as a business rule |
| Orphan: one unreadable figure makes the whole day unreadable | keep — an unreadable cell is a broken file, not a blank one; treating it as "missing" would name the wrong reason | PRD §4 now states it as a business rule (same for a missing column) |
| Orphan: missing header column vs ARCH §8 | code is right, ARCH was wrong | ARCH §8 risk note corrected |

No clause remains non-conforming; no orphan remains unreconciled.
