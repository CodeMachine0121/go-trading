# Contract Traceability Matrix — contract-strategy-script

Contract: PRD.md (v1.0)
Design map: ARCH.md
Implementation: `git log main..HEAD` on `feat/contract-strategy-scripts` (f2ea7ee..f64a98d)
Oracle: Acceptance Criteria + Core Business Rules + NFR (52 clauses)

Abbreviations used in the Impl/Test columns:
`align` = `internal/domain/models/domains/contract_k_candle_alignment_domain.go` ·
`mdk` = `internal/domain/models/domains/market_data_kind_domain.go` ·
`csvc` = `internal/domain/service/contract_indicator_calculation_service.go` ·
`app` = `internal/application/indicator_calculation_application.go` ·
`runner` = `internal/infrastructure/script/indicator_script_runner.go` ·
`series` = `internal/domain/models/domains/k_candle_contract_series_domain.go` ·
`T-svc` = `internal/domain/service/tests/contract_indicator_calculation_service_test.go` ·
`T-app` = `internal/application/tests/contract_indicator_calculation_application_test.go` ·
`T-mdk` = `internal/application/tests/strategy_script_market_data_kind_application_test.go` ·
`T-yaegi` = `internal/infrastructure/script/tests/yaegi_contract_indicator_script_proxy_test.go` ·
`T-ctrl` = `internal/controller/tests/contract_indicator_calculation_controller_test.go`

Persistence tests skip without `TEST_POSTGRES_DSN` (not available locally); repository-level behaviour was judged by reading only.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01.1 | 建立一支合約行情種類的策略腳本 | 建立成功，讀回時行情種類為合約行情 | mdk:39; strategy_script_domain.go:109,151; strategy_script.go:78 | T-mdk:22 (case 1); strategy_script_controller_test.go:495 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.2 | 沒說行情種類就是 K 線 | 建立成功，讀回時為 K 線 | mdk:41-42 | T-mdk:22 (case 2) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.3 | 這一刀之前存下的策略腳本 | 既有那一支讀回時行情種類為 K 線 | strategy_script.go:43 (`default:'kCandle'`, AutoMigrate backfill) | — (persistence tests skip; none for this) | no-test | produces-oracle | 🟡 partial |
| AC-01.4 | 行情種類建立後不得更換 | 修改被拒絕，提示「行情種類建立後不得更換」；仍為 K 線 | mdk:71-88; strategy_script_service.go:145-155; repo writable-column whitelist | T-mdk:71 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.5 | 修改其他內容時不提行情種類 | 修改成功；仍為合約行情 | mdk:72-74 | T-mdk:85 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.6 | 修改時照抄原本的行情種類 | 修改成功 | mdk:81-87 | T-mdk:107 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.7 | 不認得的行情種類 | 建立被拒絕，提示只能是 K 線或合約行情 | mdk:51-57; strategy_script_domain.go:109-112 | T-mdk:57 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.8 | 市集看得出行情種類 | 別人瀏覽市集看得到它，行情種類是合約行情 | published_strategy_script.go:54 | T-mdk:173 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01.9 | 可用策略腳本帶著行情種類 | 兩支都在，各帶自己的種類 | strategy_script.go:78; published_strategy_script.go:54 | T-mdk:152 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.1 | 一格的價量合併 | 09:00 格 開100 收105 筆數12,000；四項量額為 60 根加總 | series:62-100; align:129-142 | T-svc:155 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.2 | 三組價格各自合併 | 標記最高 106、溢價最低 -0.0009 | series:83-90; align:143-160 | T-svc:155 (L182-184) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.3 | 一格裡有指數價格出現之前的舊資料 | 指數與溢價 OHLC 全為零；價量與標記照常 | series:119-146; align:149-160 | T-svc:189 | shallow — asserts Mark only; price/volume "照常" not asserted (fixture all 100, a bug zeroing volumes would pass) | produces-oracle | 🟠 mis-asserted |
| AC-02.4 | 沒有合約 K 線的一格不產出 | 無 09:00 格，可用根數不算它 | series:34-58 | T-svc:221 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02.5 | 現貨算式的讀法照舊有效 | 讀收盤價的行不改即算得出平均值 | contract_k_candle_vo.go:18 (embed) | T-yaegi:48 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.1 | 結算發生在這一格 | 08:00 格 費率 +0.0001，有結算 | align:163-167 | T-svc:242 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.2 | 兩次結算之間延續上一次的費率 | 09:00~15:00 **每一格** 費率 +0.0001、無結算 | align:163-167 | T-svc:249 | shallow — checks only the 15:00 bar; the 09:00 bar (first after settlement, where an off-by-one in `FundingSettledInBar` would show) is never asserted | produces-oracle | 🟠 mis-asserted |
| AC-03.3 | 帶著毫秒尾數的結算屬於它所落入的那一格 | 07:00 格 +0.0001 無結算；08:00 格 +0.0002 有結算 | align:120-122,166 | T-svc:256,263 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.4 | 一格內有多次結算取最後一次 | 日格費率 +0.0003，有結算 | align:163-167 | T-svc:307 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.5 | 負的費率照實給 | 費率 -0.00003 | align:165 | T-svc:270 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.6 | 收盤前還沒有任何一次結算 | 第一格費率零、無結算 | align:163 | T-svc:276 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03.7 | 觀察區間之前的最後一次結算延續進第一格 | 10:00 格 +0.0001，無結算 | align:55-57,163-167 | T-svc:281 + T-svc:439 (read range 02:00–12:00) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.1 | 一格內有多筆取最近一筆 | 09:00 格為 09:55 那筆 | align:124-127,173-184 | T-svc:355 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.2 | 比五分鐘細的一格延續收盤前五分鐘內的那一筆 | 1m 09:03 格持倉量 5,000 | align:169-172 | T-svc:360 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.3 | 比五分鐘細的一格,最近一筆已經太舊 | 1m 09:05 格持倉統計**全為零** | align:169-185 | T-svc:366 | shallow — only `OpenInterest` asserted and the fixture statistic has the other seven figures at zero anyway, so a leak of the other seven would still pass | produces-oracle | 🟠 mis-asserted |
| AC-04.4 | 不比五分鐘細的一格,那一筆必須落在這一格之內 | 1h 09:00 格持倉統計**全為零** | align:169-185 | T-svc:372 | shallow — same as AC-04.3 | produces-oracle | 🟠 mis-asserted |
| AC-04.5 | 收盤那一刻的統計屬於下一格 | 09:00 格 5,000；10:00 格 6,000 | align:124-127 | T-svc:378,385 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04.6 | 還沒開始錄持倉統計的那段時間 | 算得出結果；持倉統計全為零，**價量與資金費率照常** | align:173 | T-svc:399 | shallow — no settlements supplied and only `OpenInterest` asserted; "資金費率照常" never exercised | produces-oracle | 🟠 mis-asserted |
| AC-04.7 | 持倉統計的每一項都帶到格子上 | 八個數字各在自己名下 | align:176-183 | T-svc:415 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.1 | 指名一支合約行情種類的信號策略腳本 | 回一個信號，並說出計算根數與實際採用根數 | app:64-78; csvc:61-119 | T-app:106; T-svc:540 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.2 | 自帶一段吃合約行情的算式 | 回指標結果 | app:96,116-118 | T-app:124; T-ctrl:108 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.3 | 湊不出最少可算根數 | 整次拒絕，可用 12、最少 20 | indicator_calculation_domain.go `usableBucketCount` | T-svc:507 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.4 | 還沒走完的一格不交給算式 | 截止 09:30 → 最後一格 08:00 | csvc:74-80 (`ReadCutoff`) | T-svc:470 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.5 | 從來沒有合約 K 線的代號 | 整次拒絕，可用根數為零 | usableBucketCount | T-svc:514; T-ctrl (thin case) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.6 | 取用沒有宣告的參數 | 計算失敗，指出「週期」 | runner:236-238 | T-yaegi:117 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.7 | 自帶的算式照現貨的寫法收一串 K 線 | 計算失敗，說入口應收一串合約行情格 | runner:190-195 | T-yaegi:107 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.8 | 算式逾時 | 計算中止，說出逾時 | runner:240-244 | T-yaegi:137 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05.9 | 沒指定彙總刻度就以一分鐘計算 | 以一分鐘計算並說出一分鐘 | IndicatorCalculationDomain (shared) | T-svc:567 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.1 | K 線種類的策略腳本做合約指標計算 | 整次拒絕，「這支策略腳本吃的是 K 線」 | app:105-111; mdk:95-102; controller 400 | T-app:143; T-ctrl:124 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.2 | 合約行情種類的策略腳本做現貨指標計算 | 整次拒絕，「這支策略腳本吃的是合約行情」 | app:51-52,105-111 | T-app:153; market_data_kind_domain_test.go:12 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.3 | K 線種類的策略腳本做現貨指標計算 | 結果與這一刀之前完全相同 | app:45-59; indicator_calculation_service.go (ToResultDto refactor) | T-app:164 + unchanged spot suites | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06.4 | 別人的、沒發佈的合約行情種類策略腳本 | 整次拒絕，找不到這支策略腳本（同現貨） | app:99-103 (gates before kind check) | T-app:180 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 行情種類：二選一；未說即 K 線；建立後不得更換；既有一律 K 線 | 同左 | mdk; strategy_script.go:43 | T-mdk (all) | no-test for "既有一律 K 線" | produces-oracle | 🟡 partial |
| BR-2 | 合約行情格：由格內合約 K 線決定存在；價量/三組價格照規則合併；一般數字同名 | 同左 | series; align:129-160; vo embed | T-svc:155,189,221; T-yaegi:48 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 資金費率＝結算時間早於收盤的**最近一次**結算；收盤前**沒有任何**結算時為零 | 無論上一次結算多久以前，都延續它 | align:17,55-57 (reads only 8h before the first bar) | T-svc:439 pins the 8h reach | asserts a different rule (8h cap) | diverges — if stored settlements have a gap > 8h before the first bar (fetch outage, or a venue interval change), the first bars carry 0 although an earlier settlement exists; PRD has no such cap (ARCH §8 accepts it as a risk, PRD does not) | 🔴 violation |
| BR-4 | 持倉統計：收盤前最近一筆，落在格內或收盤前 5 分鐘內較寬者；否則全為零 | 同左 | align:169-185 | T-svc:326 | asserts-oracle (OI dimension) | produces-oracle | ✅ conforms |
| BR-5 | 缺值給零（指數/溢價、無結算、無夠新持倉統計） | 每項缺值都是 0 | align:149-185; optional_figure_domain.go:65 | T-svc:189,276,366,372,399 | shallow for position statistics (only OI ever asserted as zero) | produces-oracle | 🟠 mis-asserted |
| BR-6 | 不偷看未來：收盤那一刻的結算與統計屬於下一格；還在走的一格不算 | 收盤時刻的結算不出現在前一格 | align:120-127; ReadCutoff | T-svc:378 (statistic), 470 (running) | shallow — no test asserts a settlement stamped exactly at a bar's close is excluded from that bar (case T-svc:242 only inspects the 08:00 bar, never 07:00) | produces-oracle | 🟠 mis-asserted |
| BR-7 | 不得混用：種類與計算種類不符即整次拒絕 | 兩個方向都拒絕 | app:105-111 | T-app:140 (both directions) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 其餘與現貨指標計算相同 | 觀察區間/刻度/根數/參數/逾時/三道關卡同現貨 | shared IndicatorCalculationDomain, runner, ResolveRunnableStrategyScript | T-svc:591; T-ctrl:133 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 讀資料次數固定（K 線、結算、持倉統計各一次） | 恰三次讀取，不隨格數增加 | csvc:74-100 | T-svc:111-140 (gomock Times(1) on each) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 必須登入；擁有權與市集規則同現貨 | 未登入被拒 | dependencies.go:397 (`requiresSignIn`) | routes_test.go:97 only asserts mounting | no-test (sign-in not asserted for this route) | produces-oracle | 🟡 partial |
| NFR-3 | 既有呼叫不帶行情種類照舊有效；現貨回應形狀不變 | 同左 | mdk:41; ToResultDto | T-mdk:22 case 2, :85; unchanged spot suites | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| mdk:39-49 | Kind spelling tolerates surrounding blanks and any letter case (` ContractKCandle ` accepted) | undocumented |
| strategy_script_service.go:145-149; app:105-108 | A stored kind nobody recognises refuses updates and runs; on the calculation route it is a plain error (no sentinel) and falls through to 502 | undocumented |
| yaegi_contract_indicator_script_proxy.go:52 | `ExecuteForEachCandle` for contract bars exists (prep for contract backtest) but no route uses it | undocumented (not scope creep: no backtest behaviour ships) |
| indicator_calculation_controller.go (default branch) | Read failure answered 502 with the raw storage error text rather than a business "讀不到行情" sentence (PRD §4 Edge Case) — same as spot | undocumented / edge case not asserted |

Out-of-scope check: trading strategies, bots, assistant, backtest, chart overlay and fetching-to-fill are untouched. No scope-creep violations.

## Summary

- Conforms: 41/52 clauses ✅ (78.8%)
- Violations: BR-3
- Mis-asserted: AC-02.3, AC-03.2, AC-04.3, AC-04.4, AC-04.6, BR-5, BR-6
- Partial: AC-01.3, BR-1, NFR-2
- Gaps: none
- Unclear: none
- Orphans: 4

Ceiling: static conformance audit. Mapped tests were run as corroboration only (`TestContractCalculation*`, `TestContractExecute*`, application contract/kind tests: all green); verdicts come from oracle comparison. Persistence tests could not run (no `TEST_POSTGRES_DSN`).

## Re-audit after fixes

Every non-conforming clause above was resolved on the same branch:

| Clause | Was | Fix | Now |
|---|---|---|---|
| BR-3 | 🔴 violation (8h reach) | The first bar's rate now comes from a dedicated read of the latest settlement strictly before it (`IContractFundingRateSettlementRepository.FindLatestBefore`), however long ago; the settlement range covers only the bars. New test: a two-day gap still carries the old rate (`TestContractCalculationCarriesTheRateInForceHoweverLongAgoItWasSettled`). Falsified by dropping the lead-in, shifting its cut-off, and widening the range. | ✅ conforms |
| AC-03.2 | 🟠 | Every bar 09:00–15:00 asserted (`TestContractCalculationCarriesTheEarlierRateOnEveryBarBetweenTwoSettlements`) | ✅ |
| AC-04.3, AC-04.4, BR-5 | 🟠 | All eight statistic figures seeded non-zero and asserted zero (`TestContractCalculationCarriesAllZerosForAStatisticThatIsNotRecentEnough`) | ✅ |
| AC-04.6 | 🟠 | Funding and prices asserted alongside the zero statistics (`TestContractCalculationAnswersAStretchBeforeStatisticsWereRecordedWithFundingAndPricesIntact`) | ✅ |
| AC-02.3 | 🟠 | Open, close, volumes and trade count asserted beside the zeroed lines (`TestContractCalculationMergesPricesAndVolumesAsUsualBesideAnOldCandle`) | ✅ |
| BR-6 | 🟠 | Settlement stamped exactly at the 07:00 bar's close asserted absent from it (`TestContractCalculationLeavesASettlementAtTheCloseToTheNextBar`) | ✅ |
| AC-01.3, BR-1 | 🟡 | Storage tests: a row written without the kind reads back as `kCandle`; a rewrite never changes it (`strategy_script_repository_test.go`) — **skipped locally without `TEST_POSTGRES_DSN`** | ✅ (pending a Postgres run) |
| NFR-2 | 🟡 | Route turns away a request with no proof of identity (`TestContractIndicatorRouteTurnsAwayARequestCarryingNoProofOfIdentity`) | ✅ |
| Orphans (4) | ⚠️ | Reconciled into PRD §4 Edge Cases (spelling tolerance, unrecognised stored kind, read failure, per-bar replay reserved for the next slice) | documented |

Conformance after fixes: 52/52 clauses (two of them rest on storage tests that need a Postgres run).
