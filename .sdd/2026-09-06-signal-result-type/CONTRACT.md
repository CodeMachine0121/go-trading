# Contract Traceability Matrix — 信號指標值種類

Contract: `PRD.md`
Design map: `ARCH.md`
Implementation: `internal/domain/`, `internal/infrastructure/script/`
Oracle: Acceptance Criteria (25 scenarios + 11 business rules)
Verification: **static conformance audit** — judges test assertions and code paths
against the spec's expected outcome; does not execute invented scenarios.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | US-01 宣告信號並算出買入 | 結果的信號是買入；結果說明種類是「信號」 | `indicator_calculation_service.go:108` · `indicator_script_shape.go:82` | `indicator_calculation_service_test.go` "reports the signal itself, with no indicator name" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | US-01 完全沒有宣告種類時信號不是預設 | 視為「一個數字」；均價 110 | `indicator_result_type_domain.go:41-43` | `indicator_result_type_domain_test.go` "nothing declared falls back to one number" + service test "reports one number per indicator when nothing was declared" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | US-01 宣告的種類不在五種之內 | 拒絕，列出五種（一個數字、一串數字、一個是非、一串是非、信號）；不回傳結果 | `indicator_result_type_domain.go:46-58` | `indicator_result_type_domain_test.go` "RefusesAnythingElse" (asserts `float、floatList、bool、boolList、signal`) + `indicator_calculation_controller_test.go` "reports a kind that is not on offer" (asserts `signal` in body) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | US-02 算式選出買入 | 這次的信號是買入 | `yaegi_indicator_script_proxy.go:150` · `indicator_script_shape.go:88` | `yaegi_indicator_script_proxy_test.go` "ReadsASignalUnderTheSignalKind/Buy" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | US-02 算式選出賣出 | 這次的信號是賣出 | `yaegi_indicator_script_proxy.go:151` | `yaegi_indicator_script_proxy_test.go` ".../Sell" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | US-02 算式選出持有 | 這次的信號是持有 | `yaegi_indicator_script_proxy.go:152` | `yaegi_indicator_script_proxy_test.go` ".../Hold" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | US-02 算式想表達三者以外的意見 | 系統沒有這樣的入口，表達不出第四種 | `signal_vo.go` (only buy/sell/hold consts) · `yaegi_indicator_script_proxy.go:149-152` (only Buy/Sell/Hold injected) | structural — `TestExecuteRefusesABadSignal/...not buy, sell or hold` covers the escape hatch (`indicator.Signal("long")` → reject) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | US-02 算式自己拼一個像信號的東西回傳 | 拒絕；說明信號必須用系統提供的方式產生（ARCH bridge: 形式必須是 `indicator.Signal`）；不回傳結果 | `yaegi_indicator_script_proxy.go:175-178` (shape check) | `yaegi_indicator_script_proxy_test.go` "RefusesABadSignal/a signal script that hands back a set of numbers" + ".../the shape message names the signal form" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | US-03 算式明確給出一個方向 | 這次的信號是賣出 | `indicator_script_shape.go:88` | covered by AC-05 test | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | US-03 算式建立了信號卻沒設方向 | 拒絕整次；「算式的問題：方向沒有設定」；不回傳結果 | `indicator_script_shape.go:90-93` | `yaegi_indicator_script_proxy_test.go` "RefusesABadSignal/a signal built but never given a direction" (asserts `沒有設定方向` + `ErrIndicatorScriptFailed` + nil) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | US-03 算式這一次完全沒有給出信號 | 拒絕整次；「信號種類必須產出一個信號」；不回傳結果 | `indicator_script_shape.go:29-33` (entry point must return `indicator.Signal`) + `:90-93` (zero value → reject) | `yaegi_indicator_script_proxy_test.go` "RefusesABadSignal/a signal built but never given a direction" — ARCH §7 maps "no signal" to the zero-value path; there is no `return nil` path under this kind, so the two collapse by design | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | US-03 信號的產出沒有指標名稱 | 結果就是這一個信號本身，不是帶名稱的一組值 | `indicator_calculation_service.go:107-111` (`Signal` set, `Values` left nil) · `indicator_calculation_result_dto.go` | `indicator_calculation_service_test.go` "reports the signal itself…" (asserts `resultDto.Signal == "buy"` **and** `resultDto.Values` empty) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | US-04 算式產出信號，回測正常重演 | 逐棒模擬進出場；交出成績單、交易明細、資金曲線 | `backtest_service.go:66-84` · `backtest_domain.go:130` | `backtest_application_test.go` "hands back the report card, the trades and the curve together" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | US-04 算式產出的是一個數字（舊寫法） | 回測拒絕；說明只接受信號種類、要改寫（ARCH bridge: 形式必須是 `indicator.Signal`）；不回傳結果 | shared `prepare()` shape check via `ExecuteForEachCandle` → `yaegi_indicator_script_proxy.go:175` | `yaegi_indicator_script_per_candle_test.go` "a number-emitting script is refused under the signal kind" (asserts `ErrIndicatorScriptFailed` + `indicator.Signal` + nil) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | US-04 算式從來沒有產出任何信號 | 回測拒絕；不默默當成「全程持有、零交易」 | `indicator_script_shape.go:90-93` via `ExecuteForEachCandle` | `yaegi_indicator_script_per_candle_test.go` "a signal left unset on one candle…" (unset ≡ "no signal", ARCH §7) + "a number-emitting script is refused…"; `backtest_application_test.go` "a strategy that only ever holds…" confirms hold ≠ absent | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | US-04 某一棒沒產出信號 | 整次回測失敗；不回傳任何部分結果 | `yaegi_indicator_script_proxy.go:97-108` (first error ends the loop, returns nil) | `yaegi_indicator_script_per_candle_test.go` "a signal left unset on one candle brings the whole run down" (asserts `ErrIndicatorScriptFailed` + nil) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | US-04 回測請求不帶指標值種類 | 一律以信號種類執行；不需宣告 | `backtest_domain.go:130` (hardcoded) · `dto/backtest_request_dto.go` (no kind field) | `backtest_application_test.go` "replays the script over the finished hours…" (asserts `resultType.IsSignal()` in the proxy call) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | US-05 單純算指標宣告信號種類 | 回傳這次的信號是買入；不帶指標名稱 | `indicator_calculation_service.go:108-111` | `indicator_calculation_service_test.go` "reports the signal itself…" + `indicator_calculation_controller_test.go` "writes a signal out as the result itself" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | US-05 建立策略時把種類設成信號 | 策略正常建立；不因「不在四種之內」被拒 | `strategy_domain.go:69` (delegates `NewIndicatorResultTypeDomain`) · `indicator_result_type_domain.go:19` | `strategy_domain_test.go` "AcceptsTheSignalKind" (asserts created + `IsSignal()`) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | US-05 修改策略時把種類改成信號 | 策略正常更新 | `strategy_domain.go:69` (same path for create & rewrite) | `strategy_domain_test.go` "AcceptsTheSignalKind/rewriting an existing strategy to emit signals" (ID set → updated, `IsSignal()` true) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | US-06 回測逐棒執行的規則不變 | 第 N 棒看得到第一到第 N 根；第五棒看五根 | `backtest_domain.go` `SelectInputCandles` + `ExecuteForEachCandle` (untouched) | `yaegi_indicator_script_per_candle_test.go` "each run sees everything up to the candle it stands on" (unchanged, still green) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | US-06 排除還沒走完的那一格 | 還在走的刻度區間不被取用 | `backtest_domain.go` `readCutoff` / `SelectInputCandles` (untouched) | `backtest_domain_test.go` "the bucket the stretch ends in is left out" (unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | US-06 根數超過單次可用上限 | 拒絕；說出要用到幾根、上限幾根 | `backtest_domain.go` `NewBacktestDomain` bucket-count check (untouched) | `backtest_domain_test.go` "a stretch needing more buckets than one read allows is refused" (unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | US-06 算式逾時 | 拒絕；說明算式未能在允許時間內算完 | `yaegi_indicator_script_proxy.go` `runOver` allowance (untouched; shared by both entry points) | `yaegi_indicator_script_proxy_test.go` "GivesUpOnAScriptThatNeverFinishes" (unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | US-06 持有那一棒的模擬行為不變 | 空手時什麼都不發生；與原「持平」一字不差 | `signal_domain.go:34-41` (`hold` → `("", false)`) · `backtest_account_domain.go:47-50` (early return) | `backtest_simulation_domain_test.go` "holding does nothing at all" + `backtest_account_domain_test.go` "a hold opinion moves nothing" | asserts-oracle | produces-oracle | ✅ conforms |
| BR-01 | 五選一，宣告其他一律拒絕 | 見 AC-03 | `indicator_result_type_domain.go:14-20,46-58` | AC-03 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 沒有宣告仍視為「一個數字」 | 見 AC-02 | `indicator_result_type_domain.go:41-43` | AC-02 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 信號只有買入、賣出、持有三者之一 | 見 AC-04/05/06/07 | `signal_vo.go` · `indicator_script_shape.go:88` | AC-04..07 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 信號由系統提供的方式產生，不能自建 | 見 AC-08；自行拼湊被拒 | `yaegi_indicator_script_proxy.go:149` (bare type, no constructor) · `:175` | AC-08 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 每次執行剛好一個信號；沒設方向/沒產出即算式失敗 | 見 AC-10/11；ErrIndicatorScriptFailed，不回部分結果 | `indicator_script_shape.go:90-93` | AC-10 + AC-11 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 信號種類的產出不是一組「名稱對應值」 | 見 AC-12 | `indicator_calculation_service.go:107-111` | AC-12 test | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 回測一律以信號種類執行；非信號即拒絕整次 | 見 AC-13/14/17 | `backtest_domain.go:130` · `backtest_service.go:66-84` | AC-13/14/17 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 移除舊讀法與「無信號→零交易」相容路徑 | 數字型/無信號算式不再默默零交易 | `signal_domain.go` (no sign/NaN logic; `SignalIndicatorName` deleted) · `backtest_simulation_domain.go` (no "missing → flat") | `signal_domain_test.go` (rewritten; no sign cases) — absence-of-behavior; `backtest_simulation_domain_test.go` no longer has "a candle with no result is read as flat" | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 信號種類到處可用（單純算指標、策略） | 見 AC-18/19/20 | `indicator_calculation_service.go:108` · `strategy_domain.go:69` | AC-18/19 tests; AC-20 partial | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 「持平」改名「持有」，模擬行為不變 | hold 那一棒倉位不動、不記交易 | `signal_vo.go:15` (`SignalHold = "hold"`) · `backtest_account_domain.go:47` | `backtest_account_domain_test.go` "a hold opinion moves nothing" + `backtest_simulation_domain_test.go` "holding does nothing at all" | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 其餘既有規則不變 | 取 K 線/上限/逾時/逐棒 — 見 AC-21..24 | untouched code paths | AC-21..24 tests (unchanged, green) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-perf | 算式只解讀一次、逐棒執行 | 一次重演只 parse 一次 | `yaegi_indicator_script_proxy.go` `prepare()` once, `runOver` per candle (untouched) | `yaegi_indicator_script_per_candle_test.go` "the script is read once and then run" (unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-sec | 沙箱不因信號種類放寬 | 信號值由沙箱注入常數提供，算式碰不到 K 線以外的東西 | `yaegi_indicator_script_proxy.go:37-40` (allowlist unchanged) · `:149-152` (injected consts only) | existing sandbox tests "reaches for the file system / network / clock" (unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-compat | 沒宣告種類的計算行為不變；回測破壞性變更是刻意的 | 既有 float 請求不受影響 | `indicator_result_type_domain.go:41-43` | all pre-existing float/bool/list tests still green | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `vo.SignalIndicatorKey` (`signal_vo.go:22`) | Internal map key packing the one signal into the script-result map | undocumented plumbing — named per ARCH §2 requirement ("signal 名稱不得散落"); not user-facing; not scope creep |
| `IndicatorValueVo.Signal` (`indicator_value_vo.go`) | Third content field alongside Numbers/Booleans | supports BR-06 (signal content shape); not an orphan |

No orphan matches an Out-of-Scope item. Out-of-scope boundaries checked and **not** violated:
signal carries only a direction (no size/stop/confidence); one signal per execution;
no strategy pre-run validation; backtest trading math untouched.

## Summary

- Conforms: 38/38 clauses ✅ (100%)
- Violations: none 🔴
- Mis-asserted: none 🟠
- Partial: none 🟡
- Gaps: none ❌
- Unclear: none ❔
- Orphans: 0 (2 supporting types listed, both explained)

The first audit pass found AC-14, AC-15, AC-20, BR-05, BR-07 with the code producing
the oracle but the tests shallow at the replay boundary. Two test-only additions
closed them:

1. `yaegi_indicator_script_per_candle_test.go` "a number-emitting script is refused
   under the signal kind" — a `map[string]float64` script rejected through
   `ExecuteForEachCandle` with `indicator.Signal` in the message.
2. `strategy_domain_test.go` "rewriting an existing strategy to emit signals" — the
   update path (ID set) accepts `resultType: "signal"`.

No behaviour changed; the code paths were already correct.
