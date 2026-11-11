# Contract Traceability Matrix — strategy-script-usage（策略腳本的用途）

Contract: PRD.md (v1.0, Finalized)
Design map: ARCH.md
Implementation: `git diff main...HEAD` on `feature/bot-trading-direction-and-script-usage` — `internal/`, `cmd/`
Oracle: Acceptance Criteria (30 clauses: 26 AC + 3 BR + 1 NFR)
Glossary: `.sdd/UL-MAP.md` (用途、切換用途、市集策略腳本)

Out-of-scope negative checklist (PRD §1): auto-inferring usage from the indicator value kind; new usage kinds; frontend screens and the plugin.

Ceiling: this is a **static conformance audit**. It judges test assertions and code paths against the spec's expected outcome. It does not execute invented scenarios. As corroboration, only the mapped test sets were run (domain / application / controller / persistence usage tests; all green). Pass/fail never decided a verdict.

## Clauses

The `Spec-expected` column holds the business-observable oracle from Phase 2. The audit columns check the concrete artifact that the oracle bridges to through UL-MAP/ARCH. Paths are abbreviated: `D/` = `internal/domain/models/domains/`, `DT` = `D/tests/strategy_script_usage_domain_test.go`, `AT` = `internal/application/tests/strategy_script_usage_application_test.go`, `CT` = `internal/controller/tests/strategy_script_usage_controller_test.go`, `RT` = `internal/infrastructure/persistence/tests/strategy_script_usage_repositories_test.go`, `SVC` = `internal/domain/service/strategy_script_service.go`, `APP` = `internal/application/strategy_script_application.go`.

`Test audit` / `Code audit` are Claude's shadow verdict. `Jev test` / `Jev code` are Jev's pick with its confidence. `Status` and `Route` come from `jev_audit.py` exactly as returned.

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | 建立圖表用腳本：Given 一支新腳本，用途說「圖表用」 When 擁有者建立它 Then 建立成功，用途為圖表用 | Created; usage = chart | D/strategy_script_usage_domain.go:22-32; D/strategy_script_domain.go:82-85,124; SVC:31-45 | DT:26-46 "a chart script" | asserts-oracle | produces-oracle | asserts-oracle 0.84 | produces-oracle 0.81 | ✅ conforms | recheck |
| AC-2 | 建立時沒說用途：Given 一支新腳本，沒有提到用途 When 擁有者建立它 Then 建立成功，用途為計算用 | Created; usage = calculation | D/strategy_script_usage_domain.go:23-26 | DT:26-46 "nothing said…" | asserts-oracle | produces-oracle | asserts-oracle 0.95 | produces-oracle 0.75 | ✅ conforms | recheck |
| AC-3 | 用途不在兩種之內：Given 用途說「回測用」 When 建立 Then 建立被拒絕，並說明用途只能是圖表用或計算用 | Refused; says usage can only be chart or calculation | D/strategy_script_usage_domain.go:34-41 | DT:48-53 | asserts-oracle | produces-oracle | asserts-oracle 0.83 | produces-oracle 0.88 | ✅ conforms | recheck |
| AC-4 | 沒人在用的腳本切成圖表用 → 切換成功，用途為圖表用 | Switch succeeds; usage = chart | APP:32-56; SVC:160-190; strategy_script_repository.go:173-188 | AT:32-68 case 1; CT:21-65 case 1 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.97 | ✅ conforms | accept |
| AC-5 | 交易策略在用時不能切成圖表用 → 拒絕，並說出交易策略「4H 趨勢」正在用 | Refused; names trading strategy 「4H 趨勢」 | SVC:177-180; D/strategy_script_errors.go:89-100; trading_strategy_repository.go:199-236 | AT:94-97; CT:32-35 (409); RT:60-84 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.89 | ✅ conforms | accept |
| AC-6 | 篩選設定在用時不能切成圖表用 → 拒絕，並說出篩選設定「新幣」 | Refused; names screening profile 「新幣」 | SVC:177-180; D/strategy_script_errors.go:89-100; symbol_screening_profile_repository.go:102-116 | AT:98-101; RT:86-110 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.92 | ✅ conforms | accept |
| AC-7 | 圖表用切回計算用 → 切換成功，用途為計算用 | Switch succeeds; usage = calculation | SVC:177-189 (no reference check for calculation) | AT:41-42 | asserts-oracle | produces-oracle | asserts-oracle 0.83 | produces-oracle 0.93 | ✅ conforms | recheck |
| AC-8 | 切成原本的用途 → 切換成功，用途仍為計算用 | Succeeds; usage stays calculation | SVC:173-175 | AT:70-82 | asserts-oracle | produces-oracle | asserts-oracle 0.48 | produces-oracle 0.68 | ✅ conforms | escalate (low confidence) |
| AC-9 | 市集副本也能切換 → 切換成功，用途為圖表用 And 副本的指標算式仍然看不到也改不了 | Succeeds; usage = chart; **and** the copy's algorithm stays hidden and not modifiable | SVC:160-190; strategy_script_repository.go:173-188 (writes `usage` only); entities/strategy_script.go:39-43 (Script blanked for copies); SVC:113-131 (rewrite of copy still refused) | AT:43-48 (usage only); RT:23-34 (non-copy row) | shallow | produces-oracle | shallow 0.29 | produces-oracle 0.89 | 🟠 mis-asserted | escalate (low confidence) |
| AC-10 | 別人的腳本 → 切換被拒絕，說法與找不到這支腳本相同 | Refused with the same wording as "script not found" | SVC:164-168, 243-252; D/strategy_script_access_domain.go:73-79 | AT:102-109 | asserts-oracle | produces-oracle | asserts-oracle 0.77 | produces-oracle 0.71 | ✅ conforms | recheck |
| AC-11 | 改寫內容時順便送用途 → 改寫被拒絕，並說明請改用切換用途 | Rewrite refused; points to switching usage | SVC:132-136; D/strategy_script_errors.go:80-84; strategy_script_controller.go:153-157 | AT:132-144; CT:67-79 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-12 | 計算用的信號腳本當信號來源 → 存下成功 | Trading strategy saved | D/trading_strategy_signal_sources_domain.go:90-99; trading_strategy_application.go:175 | DT:88 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.71 | ✅ conforms | recheck |
| AC-13 | 圖表用的腳本當信號來源 → 存下被拒絕，並說明這支腳本是圖表用，不能當信號來源 | Refused; says the script is a chart script and cannot be a signal source | D/trading_strategy_signal_sources_domain.go:96-99; D/strategy_script_usage_domain.go:53-60 | DT:82-107 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.75 | ✅ conforms | recheck |
| AC-14 | 篩選設定指名圖表用的腳本 → 存下被拒絕，並說明這支腳本是圖表用，不能用於篩選 | Refused; says the script is a chart script and cannot be used for screening | D/symbol_screening_profile_domain.go:143-149; symbol_screening_profile_service.go:156 | DT:122-132 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.64 | ✅ conforms | recheck |
| AC-15 | 直接回測圖表用的腳本 → 回測被拒絕，並說明圖表用的腳本不能回測 | Refused; says chart scripts cannot be backtested | backtest_application.go:117-123; backtest_controller.go:100-103 | backtest_application_test.go:439-448 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.99 | ✅ conforms | accept |
| AC-16 | 計算用但不是信號種類 → 存下照舊被拒絕，並說明信號來源必須是信號種類 | Refused as before, with the signal-kind reason | D/trading_strategy_signal_sources_domain.go:82-88 (before usage check at 90) | DT:109-120 | asserts-oracle | produces-oracle | asserts-oracle 0.95 | produces-oracle 0.91 | ✅ conforms | accept |
| AC-17 | 圖表用的腳本出現在圖表選單 | A chart, plottable script appears in the chart indicator menu | entities/strategy_script.go:53 (Usage on DTO; menu filtering lives in the frontend per ARCH) | assistant list test (assistantqueries/tests/strategy_script_assistant_queries_test.go:90-105) only | shallow | unclear | asserts-oracle 0.21 | produces-oracle 0.67 | ✅ conforms | escalate (disagree) |
| AC-18 | 計算用的腳本不出現在圖表選單 | A calculation, plottable script does NOT appear in the chart menu | same as AC-17 (frontend) | same as AC-17 | shallow | unclear | mis-asserted 0.29 | unclear 0.35 | ❔ unclear | escalate (disagree) |
| AC-19 | 計算用的腳本照樣能試算 → 計算不因用途被拒絕，並回覆結果 | One-off calculation not refused for usage; result returned | indicator_calculation_application.go:32-95 (no usage check) | — | no-test | produces-oracle | no-test 1.00 | produces-oracle 0.80 | 🟡 partial | recheck |
| AC-20 | 查詢腳本看得到用途 → 查詢它或列出可用的腳本，看到用途是圖表用 | Both get and list show usage = chart | entities/strategy_script.go:53; dto/strategy_script_dto.go:17-19 | assistant list test only (strategy_script_assistant_queries_test.go:90-105); no GET/list HTTP assertion | shallow | produces-oracle | asserts-oracle 0.92 | produces-oracle 0.57 | ✅ conforms | escalate (disagree) |
| AC-21 | 市集顯示用途 → 其他使用者看到它的用途是圖表用 | Marketplace shows usage = chart | entities/published_strategy_script.go:26 | DT:154-160; assistant test :319-377 | asserts-oracle | produces-oracle | asserts-oracle 0.91 | produces-oracle 0.76 | ✅ conforms | recheck |
| AC-22 | 加入時副本帶著用途 → 副本的用途是圖表用 | Adopted copy's usage = chart | D/strategy_script_marketplace_copy_domain.go:73 | DT:134-152 | asserts-oracle | produces-oracle | asserts-oracle 0.92 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-23 | 發佈者之後切換不影響副本 → 副本的用途仍是圖表用 | Copy keeps chart after publisher switches original | copy is an independent row (marketplace_copy_domain.go:52-80); UpdateUsage by id (strategy_script_repository.go:173-188) | — | no-test | produces-oracle | no-test 1.00 | produces-oracle 0.81 | 🟡 partial | recheck |
| AC-24 | 既有畫線用腳本 → 系統升級後用途是計算用 | Pre-existing drawing script becomes calculation | entities/strategy_script.go:21-22 (`default:'calculation'`, AutoMigrate at schema_migrator.go:77+) | RT:12-21 (fresh insert, not a pre-existing row) | shallow | produces-oracle | asserts-oracle 0.77 | produces-oracle 0.64 | ✅ conforms | escalate (disagree) |
| AC-25 | 既有市集副本 → 系統升級後用途是計算用 | Pre-existing marketplace copy becomes calculation | same as AC-24 | RT:12-21 (not a copy, not pre-existing) | shallow | produces-oracle | asserts-oracle 0.36 | produces-oracle 0.65 | ✅ conforms | escalate (disagree) |
| AC-26 | 升級重做不覆蓋擁有者的切換 → 用途仍是圖表用 | Re-running the upgrade keeps chart | AutoMigrate never rewrites existing values | — | no-test | produces-oracle | no-test 1.00 | produces-oracle 0.41 | 🟡 partial | escalate (low confidence) |
| BR-1 | 用途是「多加的一道」檢查：信號來源、篩選設定、直接回測除原有規則外再要求計算用 | All three consumers require calculation, on top of their existing rules | signal_sources_domain.go:90-99; symbol_screening_profile_domain.go:143-149; backtest_application.go:117-123 | DT:82-132; backtest_application_test.go:439-458 | asserts-oracle | produces-oracle | asserts-oracle 0.35 | produces-oracle 0.96 | ✅ conforms | escalate (low confidence) |
| BR-2 | 機器人每一輪讀規則、交易策略回測讀規則時照舊經過信號來源的檢查；正常情況下不會在這裡被擋 | Rounds and strategy backtests still pass the signal-source check; normally not blocked | strategy_bot_run_application.go:212-220 and SVC:271-296 resolve ownership only, **no usage check** (ARCH: "Not touched") | — | no-test | unclear | no-test 1.00 | produces-oracle 0.87 | 🟡 partial | escalate (disagree) |
| BR-3 | 「單純算一次指標」不檢查用途；圖表只列圖表用是選單層面的規則 | One-off calculation never checks usage | indicator_calculation_application.go:32-95 | — | no-test | produces-oracle | no-test 1.00 | produces-oracle 0.72 | 🟡 partial | recheck |
| NFR-1 | 升級可重複執行、結果不變；既有交易策略、篩選設定、機器人升級後全部照常運作 | Upgrade is idempotent; everything existing keeps working (all scripts are calculation) | entities/strategy_script.go:21-22; schema_migrator.go:77+ | — | no-test | produces-oracle | no-test 1.00 | produces-oracle 0.72 | 🟡 partial | recheck |

### Recheck notes
- AC-1/2/3/7/10/12/13/14/21: re-read the paths. The mapped tests ran green. AC-12/13 rely on `trading_strategy_application.go:175` copying `DeclaredUsage`. No application-level test proves that wiring; only the domain test does.
- AC-19/23/BR-3/NFR-1: the code path is correct by reading, but no test pins it.

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| D/strategy_script_usage_domain.go:23-29 | Usage parsing trims whitespace and ignores letter case (`"  Chart "` is accepted, DT:35) | undocumented |
| D/trading_strategy_signal_sources_domain.go:90-94; D/symbol_screening_profile_domain.go:143-146; backtest_application.go:117-120 | A stored usage that is neither value is refused at every consumer (DT:162-182, backtest test :450-458) | undocumented (defensive) |
| APP:32-56 + SVC:160-190 | The switch reads referencing names, then writes `usage` with no transaction or lock. A trading strategy saved between the read and the write can end up naming a chart script. PRD §4 Edge Case says "兩邊都不會留下不一致的引用", and this path does not guarantee it | undocumented (race against PRD edge case) |
| trading_strategy_repository.go:199-236; symbol_screening_profile_repository.go:102-116 | ARCH names `FindNamesByOwnerAndStrategyScript`. The code is not owner-scoped. This is harmless today because both consumers only accept owned scripts | undocumented (ARCH deviation) |

## Summary

- Conforms: 22/30 clauses ✅ (73%)
- Violations: none
- Mis-asserted: AC-9
- Partial: AC-19, AC-23, AC-26, BR-2, BR-3, NFR-1
- Gaps: none
- Unclear: AC-18
- Orphans: 4
- Jev: ran on 30 clauses
- Needs James: AC-8 (low confidence), AC-9 (low confidence), AC-17 (disagree), AC-18 (disagree), AC-20 (disagree), AC-24 (disagree), AC-25 (disagree), AC-26 (low confidence), BR-1 (low confidence), BR-2 (disagree)

## Resolution (after the audit)

| Clause | Action |
| :--- | :--- |
| BR-2 + race orphan | Implemented: every path reading a trading strategy's rules (bot round, strategy replay, live comparison) now resolves sources through `ResolveOwnedCalculationScript`; a chart source halts the bot (strategy script unavailable) or refuses the replay. Tests: bot-round halt case, strategy replay refusal. The remaining save-vs-switch window is recorded as known debt in ARCH; PRD edge case reworded to match. |
| AC-9 | Strengthened: switched copy returns no algorithm; added `TestStrategyScriptApplicationKeepsASwitchedCopyUnrewritable`. |
| AC-19, BR-3 | Added `TestIndicatorCalculationApplicationComputesEitherUsage`. |
| AC-24, AC-25, AC-26, NFR-1 | Added storage test `TestSchemaMigratorMakesScriptsFromBeforeUsageCalculationScriptsOnce` (rows without the column, incl. a copy; re-migration keeps an owner's switch). |
| AC-20 | Added `TestStrategyScriptRouterAnswersWithTheUsage` (read and list). |
| BR-1 | Added `TestTradingStrategyApplicationCreateRefusesAChartScriptAsASource`. |
| AC-17, AC-18 | Chart menu filtering is the front end's (verified in the frontend repo's own contract). |
| AC-23 | Not added: a copy is its own row with no link to the original (ARCH); nothing to pin beyond the copy test. |
| Lookups not owner-scoped | Intended; ARCH updated. |
