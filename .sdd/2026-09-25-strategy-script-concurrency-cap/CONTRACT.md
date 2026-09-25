# Contract Traceability Matrix — 算式隔間同時上限

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/infrastructure/script/`、`internal/domain/service/`、`internal/controller/`、`internal/config/`、`cmd/server/dependencies.go`
Oracle: Acceptance Criteria (15 scenarios) + 6 business rules + 3 NFR

> Static conformance audit: test assertions and code paths are judged against the spec's expected outcome; the only tests run were the ones mapped to a clause, as corroboration.

測試檔簡寫：`slots` = `internal/infrastructure/script/tests/indicator_script_compartment_slots_test.go`，`worker` = `.../indicator_script_worker_test.go`。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 沒有其他計算在跑時照常算出結果 | 得到 120 | `indicator_script_compartment_slots.go:30` | `slots` TestCompartmentSlotsLetACalculationStartWhileASlotIsFree/nothing else running | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 還有空位時不必等 | 前一個不結束，這次也立刻開始 | `indicator_script_compartment_slots.go:30` | `slots` …/one of two slots taken（佔住者永不結束，5 秒內算完） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 等到空位就照常算 | 前一個結束後開始並得到正確結果 | `indicator_script_compartment_slots.go:64` | `slots` TestCompartmentSlotsMakeACalculationWaitForAFreedSlot | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 在發動者願意等的時間內等不到空位 | 約 0.2 秒後以「算式隔間忙碌中，請稍後再試」收場，且不是算式失敗 | `indicator_script_compartment_slots.go:44-61` | `slots` TestCompartmentSlotsReportBusyWhenTheCallerStopsWaiting（≥0.2 秒且 <2 秒；完整說法；NotErrorIs 算式失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 重演因為等空位而用完整次允許時間 | 以「忙碌中」收場，不是「用完允許時間」 | `backtest_service.go:133`、`contract_backtest_service.go:145` | `backtest_time_allowance_test.go` TestReplayWaitingOutItsAllowanceForACompartmentIsReportedAsBusy | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 算式失敗後名額歸還 | 下一個立刻開始並得到結果 | `indicator_script_compartment.go:87` | `slots` TestCompartmentSlotsAreReturnedHoweverACalculationEnds/the script failed on purpose | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 超時中止後名額歸還 | 同上 | `indicator_script_compartment.go:87` | 同上 /the script ran out of time | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 發動者中途離開後名額歸還 | 同上 | `indicator_script_compartment.go:87` | 同上 /the caller left mid-run | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 排隊中離開的人不佔走名額 | 名額空出後下一個立刻開始 | `indicator_script_compartment_slots.go:50-59` | 同上 /the caller left while queueing | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 機器人一輪插在排隊中的隨選計算前面 | 機器人先開始，隨選其後 | `indicator_script_compartment_slots.go:68`；機器人輪次的標記 `cmd/server/dependencies.go:333` | `slots` TestCompartmentSlotsServeStrategyBotRoundsFirst/a bot round overtakes… | asserts-oracle | produces-oracle | ✅ conforms（組裝根的標記無自動測試，屬組裝根慣例） |
| AC-11 | 隨選計算之間先來先算 | 先排的先開始 | `indicator_script_compartment_slots.go:68-73` | 同上 /on-demand calculations go first come first served | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 機器人一輪在時限內等不到空位 | 這一輪跳過，機器人沒有被停下 | `strategy_bot_round_failure_domain.go`（default 分支） | `strategy_bot_round_failure_domain_test.go` every script compartment staying busy only skips the round | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 用完處理器時間額度的隔間被作業系統收掉 | 用完約 1 秒處理器時間後被收掉，沒有回答 | `indicator_script_request.go:36-56` | `worker` TestWorkerIsStoppedByTheProcessorTimeLimitWithoutAnyHelpFromItsParent | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 處理器時間額度隨重演根數放大 | 一千根重演正常完成 | `indicator_script_compartment.go:90` | `indicator_script_compartment_test.go` TestCompartmentGivesEveryCandleOfALongReplayItsOwnAllowance | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | （UI）忙碌以不同於算式失敗的類別回給畫面 | 回應可分辨為「忙碌中，稍後再試」 | 三個 controller 的 503 + `compartmentsBusy` | `indicator_calculation_controller_test.go`、`backtest_controller_test.go`、`trading_strategy_backtest_controller_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 上限不分使用者、市場、計算種類；預設 6、可設定 | 全服務一份名額池；未設或非法時為 6 | `dependencies.go:328-333`、`application_config.go:244` | `application_config_test.go` TestLoadReadsTheIndicatorScriptCompartmentCap | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 一次計算佔一個名額；隔間完全收掉後歸還；重演只佔一個 | 同 AC-6..9；重演同一個 `run` | `indicator_script_compartment.go:82-87` | AC-6..9 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 排隊順序：機器人優先、同類先來先算 | 同 AC-10/11 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 等待上限就是發動那一方本來的時限 | 不另設等待上限 | `indicator_script_compartment_slots.go:44-48` | AC-4、AC-5 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 忙碌中不是算式失敗、也不是用完允許時間 | 同 AC-4/5/12 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 處理器時間上限與允許時間一致，隔間只用一顆處理器 | 額度 = 允許時間 × 根數 + 5 秒；單一處理器 | `indicator_script_compartment.go:90`、`indicator_script_request.go:38` | AC-13/14（單一處理器無直接斷言） | shallow（單一處理器） | produces-oracle | 🟠 mis-asserted（單一處理器一項為實作手段，不另測） |
| NFR-1 | 名額未滿時不增加可察覺延遲 | 空位時直接開始 | `indicator_script_compartment_slots.go:30` | AC-2（5 秒內） | shallow | produces-oracle | 🟠 mis-asserted（未量測延遲，快速路徑只是一次加鎖） |
| NFR-2 | 單一使用者無法擠掉他人的機器人 | 同 AC-10 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 處理器時間上限只保證在正式環境作業系統 | Linux 上設不了即失敗 | `indicator_script_request.go:51` | — | no-test | produces-oracle | 🟡 partial（與記憶體上限同一模式） |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `indicator_script_request.go:41-46` | 子行程自己接 `SIGXCPU` 並結束（macOS 在 hard 上限不送 `SIGKILL`） | 實作手段，已記錄於 ARCH；AC-13 依賴它 |

## Summary

- Conforms: 20/24 clauses ✅ (83%)
- Violations: none
- Mis-asserted: BR-6（單一處理器）、NFR-1（延遲）— 皆為不宜以行為測試斷言的實作特性，接受
- Partial: NFR-3
- Gaps: none
- Unclear: none
- Orphans: 1（已說明）
