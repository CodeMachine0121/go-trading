# Contract Traceability Matrix — history-sync-concurrency-cap

Contract: PRD.md (Section 3 Gherkin AC, Section 4 Core Business Rules, Section 6 NFR)
Design map: ARCH.md (Section 7 Traceability)
Implementation: branch `fix/history-sync-global-cap` vs `main`
Oracle: Acceptance Criteria (25 clauses: 15 AC, 7 BR, 3 NFR)

Ceiling: static conformance audit. I derived the oracles from the PRD before reading any code. I ran the mapped
service / domain / controller / config tests once as a cross-check, and they pass. The persistence tests skip
when `TEST_POSTGRES_DSN` is unset, so they were **not executed** here (CI runs them against Postgres). I judged the
clauses that depend on them by reading the code only.

Path legend:

- `CAP` = `internal/domain/models/domains/k_candle_history_sync_capacity_domain.go`
- `SPOT` = `internal/domain/service/k_candle_ingestion_service.go`
- `CON` = `internal/domain/service/contract_k_candle_ingestion_service.go`
- `SREPO` / `CREPO` = `internal/infrastructure/persistence/k_candle{,_contract}_history_sync_run_repository.go`
- `SCTL` / `CCTL` = `internal/controller/k_candle{,_contract}_history_sync_controller.go`
- `CFG` = `internal/config/application_config.go`
- `T-SPOT` / `T-CON` = `internal/domain/service/tests/{k_candle,contract_k_candle}_ingestion_service_test.go`
- `T-CAP` = `internal/domain/models/domains/tests/k_candle_history_sync_capacity_domain_test.go`
- `T-SCTL` = `internal/controller/tests/k_candle_history_sync_controller_test.go`; `T-CCTL` = `internal/controller/tests/contract_controllers_test.go`
- `T-SREPO` = `internal/infrastructure/persistence/tests/k_candle_history_sync_run_repository_test.go`; `T-CREPO` = `internal/infrastructure/persistence/tests/contract_trading_symbol_repository_test.go`
- `T-CFG` = `internal/config/tests/contract_ingestion_config_test.go`

## Clauses

| ID | Clause | Oracle (from spec) | Implementation | Test | Test audit | Code audit | Status |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01-1 | 沒有任何進行中的同步時照常開跑 | 上限 2、進行中 0 → 開跑，回一筆進行中輪次 | CAP:22, SPOT:161 | T-SPOT `TestSyncingHistoryStartsWhileAPlaceIsFree/nothing else is running` | asserts-oracle | produces-oracle | ✅ |
| AC-01-2 | 最後一個空位照常開跑 | 上限 2、進行中 1 → 開跑 | CAP:22 | T-SPOT same `/the last place is free`; T-CAP admits | asserts-oracle | produces-oracle | ✅ |
| AC-02-1 | 剛好滿了就拒絕 | 進行中 2 → 拒絕為「太忙」，說明含「同時最多 2 趟，等其中一趟結束再開」；不記輪次、不問交易所 | CAP:22-26, SPOT:171, SCTL:59 | T-SPOT `TestSyncingHistoryIsRefusedWhileEveryPlaceIsTaken/exactly at the limit` (Save/Fetch `Times(0)`, message); T-SCTL `every place for a running sync is taken` (429) | asserts-oracle | produces-oracle | ✅ |
| AC-02-2 | 超過上限一樣拒絕 | 進行中 3 → 拒絕、不記輪次 | CAP:22 | T-SPOT same `/above a limit lowered since`; T-CAP | asserts-oracle | produces-oracle | ✅ |
| AC-02-3 | 其中一趟結束後就有空位 | 原 2 趟其中一趟已結束 → 開跑 | SREPO:72 (counts only `running`) + CAP | T-SREPO `TestCountingRunningHistorySyncsLeavesTheEndedOnesOut` + T-SPOT `/the last place is free` | asserts-oracle | produces-oracle | ✅ (persistence part judged by reading) |
| AC-03-1 | 合約滿了，現貨照常開跑 | 合約 2 趟進行中、現貨 0 → 現貨開跑 | separate tables/repositories; SREPO:72 | T-SREPO count test — **first pass: no contract row present**, so a spot count that included contract rows would still pass | shallow → resolved | produces-oracle | 🟠 → ✅ |
| AC-03-2 | 合約滿了就拒絕合約 | 合約 2 趟 → 拒絕為「太忙」、說明含「同時最多 2 趟」、不記合約輪次 | CON:161, CCTL:58 | T-CON `TestContractHistorySyncIsRefusedWhileEveryPlaceIsTaken`; T-CCTL `everyPlaceTakenRecorder` (429) | asserts-oracle | produces-oracle | ✅ |
| AC-04-1 | 沒設定時用預設值 | 未設 → 兩個上限各 2 | CFG:254, CFG:274 | T-CFG `TestTheHistorySyncConcurrencyLimitsAreSettledApart/nothing set` | asserts-oracle | produces-oracle | ✅ |
| AC-04-2 | 設定不成立時用預設值 | 0 / -1 / abc → 2 | CFG (`positiveIntWithDefault`) | T-CFG `/zero falls back`, `/negative or unreadable falls back` | asserts-oracle | produces-oracle | ✅ |
| AC-04-3 | 上限 1 時只容一趟 | 上限 1、進行中 1 → 拒絕 | CAP:22 | T-CAP `a limit of one` | asserts-oracle | produces-oracle | ✅ |
| AC-05-1 | 回溯天數不合法先判 | 已滿 + 回溯 0 → 拒絕為回溯天數不合法，不是「太忙」 | SPOT:120 (before count) | T-SPOT `TestSyncingHistoryJudgesTheRequestBeforeTheLimit/no days at all` | asserts-oracle | produces-oracle | ✅ |
| AC-05-2 | 沒登錄過的代號先判 | 已滿 + 沒登錄 → 拒絕為沒登錄過 | SPOT:126 (`reachSymbolOnDemand` before count) | same `/a symbol nobody registered` | asserts-oracle | produces-oracle | ✅ |
| AC-05-3 | 同一標的已在跑且上限已滿時回答已滿 | → 「太忙」 | SPOT:161 (count before Save's unique index) | T-SPOT `TestSyncingHistoryOnASymbolAlreadyRunningAnswersBusyWhenEveryPlaceIsTaken` | asserts-oracle | produces-oracle | ✅ |
| AC-06-1 | 兩個要求搶最後一個空位 | 上限 2、進行中 1、兩個同時 → 恰一個開跑、一個「太忙」 | SPOT:164, CON:164 (`historySyncStartMutex`) | T-SPOT / T-CON `…LetsOnlyOneOfTwoSimultaneousStartsTakeTheLastPlace` (mutation: removing the lock turns each red) | asserts-oracle | produces-oracle | ✅ |
| AC-07-1 | 讀不到進行中趟數 | 拒絕為系統端問題、不記輪次 | SPOT:167-170, CON:167-171; controller default branch → 502 | T-SPOT / T-CON `…StartsNothingWhenTheRunningSyncsCannotBeCounted` (Save `Times(0)`, not "busy") | asserts-oracle | produces-oracle | ✅ |
| BR-1 | 進行中輪次數以記下的輪次為準 | 數的是記錄中 `running` 的輪次 | SREPO:72, CREPO:75 | T-SREPO, T-CREPO count tests | asserts-oracle | produces-oracle | ✅ (judged by reading) |
| BR-2 | 現貨、合約各一份，預設 2，可調，必須大於零 | 兩個獨立設定 | CFG, `cmd/server/dependencies.go` | T-CFG `/each set on its own` | asserts-oracle | produces-oracle | ✅ |
| BR-3 | 現貨那一份涵蓋加密貨幣現貨與台股 | 一份計數不分市場 | SREPO:72 (no market filter) | T-SREPO count test (all rows counted regardless of symbol) | asserts-oracle | produces-oracle | ✅ |
| BR-4 | 「太忙」與「同一標的已在跑」是兩種拒絕 | 兩種不同的拒絕類別 | `ErrKCandleHistorySyncCapacityReached` vs `ErrKCandleHistorySyncInProgress`; SCTL 429 vs 409 | T-SCTL both cases; T-CCTL both recorders | asserts-oracle | produces-oracle | ✅ |
| BR-5 | 標的登錄的評估結論：同步從不登錄標的 | 沒登錄的代號被拒絕，且沒有東西被登錄 | SPOT `reachSymbolOnDemand`, CON `reachSymbolOnDemand` (no `Save` on the symbol repository) | T-SPOT / T-SCTL unregistered cases (strict gomock fails on any unexpected symbol `Save`) | asserts-oracle | produces-oracle | ✅ |
| BR-6 | 上限調低後不中斷進行中的，只拒絕新的 | 進行中 3、上限 2 → 新的拒絕；沒有東西被停掉 | CAP (only refuses); nothing cancels runners | T-SPOT `/above a limit lowered since` | asserts-oracle | produces-oracle | ✅ |
| BR-7 | 數、判斷、記下在服務內一個一個來 | 同一種同步的起跑是循序的 | SPOT:164, CON:164 | AC-06-1 tests | asserts-oracle | produces-oracle | ✅ |
| NFR-1 | 多一次數一數，成本可忽略 | 每次起跑一次計數查詢 | SREPO:72 (single `COUNT`) | — (non-behavioral) | n/a | produces-oracle | ✅ |
| NFR-2 | 既有的回覆形狀與其他拒絕不變 | 202 / 400 / 404 / 409 / 502 照舊 | SCTL, CCTL | pre-existing controller cases, all still green | asserts-oracle | produces-oracle | ✅ |
| NFR-3 | 誰能發起由登錄保護另外處理 | 本切片不動路由保護 | `cmd/server/dependencies.go` route lines untouched | — (scope boundary) | n/a | produces-oracle | ✅ |

## Orphans

| Behaviour | Where | Classification |
| :--- | :--- | :--- |
| A domain limit below one is raised to one | CAP:17-19 | Defensive; config already replaces non-positive values with the default, so this is reachable only through direct construction. ARCH §3 documents it. Kept — not an Out-of-Scope item. |
| The refusal message says 「同時最多 N 趟**歷史同步**，…」 (PRD example omits 「歷史同步」) | CAP:23 | Wording superset of the oracle; kept. |

## Summary

- Conforms: 25/25 clauses ✅ (100%) after resolution — first pass 24/25 (96%)
- Violations: none
- Mis-asserted (first pass, resolved): AC-03-1 — the spot count test had no contract row, so it could not tell a shared count apart from a separate one
- Partial: none
- Gaps: none
- Unclear: none
- Orphans: 2 (both kept, documented)
- Caveat: AC-02-3, AC-03-1, BR-1, BR-3 depend on Postgres-only persistence tests that skip without `TEST_POSTGRES_DSN`. I judged them by reading the code here; CI executes them.

## Resolution (after the first audit)

| Finding | Decision | Change |
|---|---|---|
| AC-03-1 shallow | fix the test | `TestCountingRunningHistorySyncsLeavesTheEndedOnesOut` now also records a running contract run and still expects the spot count to be 2 |

No clause remains non-conforming.
