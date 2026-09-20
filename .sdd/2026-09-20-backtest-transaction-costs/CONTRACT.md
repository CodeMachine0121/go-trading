# 回測的交易成本 — Contract Verification Matrix

**Contract source:** `.sdd/2026-09-20-backtest-transaction-costs/PRD.md`（Acceptance Criteria 為 oracle）
**Design map:** `.sdd/2026-09-20-backtest-transaction-costs/ARCH.md`
**Glossary:** `.sdd/UL-MAP.md`
**Scope:** `go-trading`（後端）
**Verified:** 2026-09-20
**Ceiling:** 靜態一致性稽核。逐條把**測試斷言**與**程式路徑**各自對照規格推出的 oracle，
不以「跑完全套變綠」當判準。

---

## Clauses

### US-01 — 發動重演時說得出兩個成本率

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-01.1 | 兩格選填，說的是成交金額的百分點 | 零或留白＝那一側不收費 | `backtest_transaction_costs_domain.go:119`（零值早退）、`:137`、`:155` | `backtest_transaction_costs_domain_test.go:88`（第四列兩格都零，兩筆成本皆 0） | ✅ conforms |
| AC-01.2 | 兩格都留白 → 每個數字逐字相同 | 成績單與這一刀之前相同 | 零值模型；`MaximumStakeFrom` 早退回傳**原物件**而非等值物件 | **既有的每一條回測測試一條斷言都沒改**（迴歸網）＋ `backtest_transaction_cost_simulation_test.go:95`（同一段行情不填＝11110） | ✅ conforms |
| AC-01.3 | 只填進場 → 出場沿用 | 兩側收同一費率 | `backtest_transaction_costs_domain.go:76`（`if !exitCostPercentage.IsPositive()`） | `backtest_transaction_costs_domain_test.go:97`（第一列）＋ 模擬層 `TestBacktestSimulationChargesTheEntryRateOnTheWayOutWhenNoneWasNamed` | ✅ conforms |
| AC-01.4 | 只填出場 → 進場不收 | 進場 0、出場收 | 兩格各自驗證後才沿用 | `backtest_transaction_costs_domain_test.go:110`（第三列） | ✅ conforms |
| AC-01.5 | 三個入口行為一字不差 | 腳本／交易策略／助手同一條路 | `trading_strategy_backtest_request_dto.go` 的 `ToBacktestRequestDto` 帶兩格過去，之後**單一路徑** | `backtest_domain_test.go`（交易策略重演的拒絕表格新增兩列——若轉接漏掉那兩格，這兩列會**收不到錯誤而失敗**）＋ 助手路徑端到端 `TestTradingStrategyBacktestAssistantQueryChargesWhatTheAssistantSaysItCosts`（finalEquity 11880） | ✅ conforms |
| AC-01.6 | 成本率為負 → 整次拒絕，說出是哪一格 | 句子含該格名稱與「不得為負」 | `backtest_transaction_costs_domain.go:94` | `backtest_transaction_costs_domain_test.go:35`、`:41`（進場與出場各一列，逐字斷言整句） | ✅ conforms |
| AC-01.7 | 超過 100 → 整次拒絕 | 理由是成本不會超過成交金額本身 | 同上 | 同上 `:47`、`:53` | ✅ conforms |
| AC-01.8 | 正好 100 送得出去 | 整筆成交金額拿去付成本 | `validatedCostPercentage` 用 `GreaterThan` 而非 `GreaterThanOrEqual` | `TestBacktestTransactionCostsAllowExactlyAHundred`（可押上限 5050、成本 5050）＋ 模擬層第三列 | ✅ conforms |

### US-02 — 成本真的從可用資金裡扣掉

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-02.1 | 進場成本開倉當下扣 | 可用資金少掉 stake ＋ 進場成本 | `backtest_account_domain.go:138` | `TestBacktestSimulationSplitsTheCashBetweenTheStakeAndItsCharge`（資金曲線第一點 10000／10049.5／5050 三列） | ✅ conforms |
| AC-02.2 | 出場成本平倉當下扣 | 收回的現金已扣掉它 | `backtest_account_domain.go:156` 的 `settleOpenPosition`（**唯一**的平倉路徑，訊號與價位共用） | `TestBacktestSimulationChargesBothEndsOfARoundTrip`（finalEquity 10890）＋ 走止損路徑的 Postman「重演：交易成本從可用資金扣」（11642.4） | ✅ conforms |
| AC-02.3 | 全押＝部位＋它的進場成本，資金歸零 | 部位略小於可用資金 | `position_sizing_domain.go:139`（全押押 `maximumStake`） | 同 AC-02.1 第一列（部位 10000、曲線 10000、初始 10100） | ✅ conforms |
| AC-02.4 | 押幾成／固定金額的押注數不變 | 成本另外扣 | `position_sizing_domain.go:141`（百分比仍乘 `availableCash`） | 同 AC-02.1 第二列（部位 5050、曲線 10049.5＝4999.5＋5050） | ✅ conforms |
| AC-02.5 | 付不起「押注＋進場成本」就跳過 | 不算失敗、不計開倉次數 | `position_sizing_domain.go:144` 的 `stake ≤ maximumStake` | `TestBacktestSimulationSkipsAnOpeningThatCannotPayItsOwnCharge`（四列：固定金額付不起／不計成本時付得起／百分之百付不起／全押自己縮） | ✅ conforms |
| AC-02.6 | 一定付不起的百分比**整次拒絕** | 不讓它交出一張空成績單 | `PositionSizingDomain.NeverStakesUnder` ＋ `BacktestDomain` 門口 | `TestPositionSizingKnowsWhenItCouldNeverStake`（六列：100%配費率／99.99%配1%／99%配1%／100%免費／全押／固定金額）＋ `backtest_domain_test.go` 拒絕表格新增一列（指向 `positionSizingValue`，證明兩條重演路徑共用同一道門） | ✅ conforms |
| — | 帳戶層仍然是「跳過這次」 | 門口擋掉的只是**可預測**的那一種 | `StakeFor` 未改 | `TestBacktestSimulationSkipsAnOpeningThatCannotPayItsOwnCharge` 第三、四列（單元層仍到得了那個狀態，前門已擋下，測試中已註明） | ✅ conforms |

### US-03 — 成本按成交金額收，做空也一樣

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-03.1 | 進場成本 ＝ 押注 × 進場成本率 | — | `backtest_position_domain.go:69` | `backtest_transaction_costs_domain_test.go:126`（含台股 0.0855 → 8.55） | ✅ conforms |
| AC-03.2 | 出場成本 ＝ **單位數 × 出場價** × 出場成本率 | 不是那一注值多少 | `backtest_position_domain.go:208`（`unitCount.Mul(exitPrice)`，**未**使用 `ValueAt`） | `TestBacktestSimulationChargesAShortOnWhatChangedHands`（出場成本 90，並**明確斷言它不是** 110） | ✅ conforms |
| AC-03.3 | 做多相等、做空不相等 | 只有做空看得出差別 | 同上 | 同上兩列（做空賺 / 做空賠）＋ 做多列 `TestBacktestSimulationChargesBothEndsOfARoundTrip`（成交金額＝部位價值＝11000） | ✅ conforms |
| AC-03.4 | 成本永遠是支出 | 方向與盈虧都不影響 | `backtest_transaction_costs_domain.go:155` 的 `.Abs()` | `TestBacktestTransactionCostsAreAlwaysACharge`（負成交金額仍回 100）＋ 做空虧錢那一列（仍收 110） | ✅ conforms |

### US-04 — 逐筆損益與勝率都是淨額

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-04.1 | 損益 ＝ 價差 − 進場成本 − 出場成本 | — | `backtest_position_domain.go:221` | `TestBacktestSimulationChargesBothEndsOfARoundTrip`（790 ＝ 1000 − 100 − 110） | ✅ conforms |
| AC-04.2 | 原地打平的損益＝負的兩筆成本 | −200 | 同上 | `TestBacktestSimulationLosesExactlyTheTwoChargesOnAFlatRoundTrip` | ✅ conforms |
| AC-04.3 | 賺不過成本不算贏 | — | `closed_trade_vo.go` 的 `IsWin` 讀淨額（**未改動**，語意隨 `Profit` 而變） | `TestBacktestSimulationWinRateCountsOnlyRoundTripsThatBeatTheirOwnCharges`（兩筆 −101） | ✅ conforms |
| AC-04.4 | 勝率是「扣掉成本還賺的比例」 | — | `backtest_account_domain.go:248` 的 `WinRate`（未改動） | 同上：**同一組三趟交易，不計成本 3/3、計成本 1/3** | ✅ conforms |
| AC-04.5 | 每一筆多記兩個成本數字 | — | `closed_trade_vo.go` ＋ `closed_trade_dto.go`（`entryCost`／`exitCost`） | 同 AC-03.2；DTO 欄位另由 Postman「兩端各收一筆」與助手測試斷言 | ✅ conforms |
| AC-04.6 | 兩格留白時兩個成本都是 0、勝率不變 | — | 零值模型 | `TestBacktestSimulationChargesBothEndsOfARoundTrip` 第二子案例（兩筆成本 "0"、損益 1010） | ✅ conforms |

### US-05 — 成績單說得出總共付了多少

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-05.1 | 成績單多一個累計成本 | — | `backtest_simulation_domain.go:102` → `backtest_summary_dto.go` | `TestBacktestSimulationChargesBothEndsOfARoundTrip`（210） | ✅ conforms |
| AC-05.2 | ＝已平倉全部成本 ＋ 未平倉已付的進場成本 | — | `backtest_account_domain.go:220`（從清單數出來，未平倉另加 `EntryCost()`） | 已平倉：`TestBacktestSimulationWinRateCounts...`（607＝201+201+205）；未平倉：`TestBacktestSimulationNeverPreChargesAPositionStillOpen`（100） | ✅ conforms |
| AC-05.3 | 兩格留白時是 0 | — | 零值模型 | `TestBacktestSimulationChargesBothEndsOfARoundTrip` 第二子案例 | ✅ conforms |

### US-06 — 還開著的那一注不預扣出場成本

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-06.1 | 資金曲線不預扣未付的出場成本 | — | `backtest_position_domain.go:108` 的 `ValueAt` **一個字都沒改**（維持毛額） | `TestBacktestSimulationNeverPreChargesAPositionStillOpen`（最後剩 11000，而非 11000 − 110） | ✅ conforms |
| AC-06.2 | 進場成本當場反映在曲線上 | 那一點立刻掉一筆 | `backtest_account_domain.go:138`（開倉即扣現金，`EquityAt` 讀現金） | `TestBacktestSimulationDrawsTheEntryChargeOnTheCurveAtOnce`（10000 vs 10100） | ✅ conforms |
| AC-06.3 | 最大回撤略大 | — | 無專屬程式碼：曲線數字變了，規則沒變 | 同上（`assert.Greater` 兩者的最大回撤） | ✅ conforms |
| AC-06.4 | 最後剩多少樂觀一筆出場成本，且要寫進說明 | 說明要說出口 | `backtest_summary_dto.go` 的 `TotalTransactionCost` 註解、`backtest_position_domain.go` 的 `ValueAt` 註解、UL-MAP「最後剩多少」 | 行為面同 AC-06.1；**說明本身是文件，不由測試守** | 🟡 partial |
| AC-06.5 | 還開著的那一注不進交易明細 | — | 未改動 | `TestBacktestSimulationNeverPreChargesAPositionStillOpen`（`assert.Empty`） | ✅ conforms |

### US-07 — 助手也說得出成本與那兩個出場距離

| ID | Clause | Oracle | Implementation | Test | Status |
| :--- | :--- | :--- | :--- | :--- | :--- |
| AC-07.1 | 助手說得出兩個成本率 | — | `trading_strategy_backtest_assistant_query.go` 的四個新參數 ＋ `ToRequestDto` 映射 ＋ `ArgumentSchema` | `TestTradingStrategyBacktestAssistantQueryChargesWhatTheAssistantSaysItCosts`（11880／220／逐筆 100 與 120） | ✅ conforms |
| AC-07.2 | 助手說得出兩個出場距離（補齊先前遺漏） | 填了就真的模擬 | 同上 | `TestTradingStrategyBacktestAssistantQuerySimulatesTheExitDistancesItWasGiven`（止損出場、finalEquity 9898 vs 不填的 9090） | ✅ conforms |
| AC-07.3 | 與使用者自己填同樣的值跑出同一張成績單 | — | 兩條路匯流於 `ToBacktestRequestDto` 之後的單一路徑 | 助手端 11880；同一組規則在 domain 層的 `TestBacktestSimulationChargesBothEndsOfARoundTrip` 用同一套算術 | ✅ conforms |
| AC-07.4 | 什麼都不填＝與這一刀之前一字不差 | — | 四個參數皆走 `decimalOrZero`，零即不計 | 兩個助手測試各有一個「不填」子案例（12120／9090） | ✅ conforms |
| AC-07.5 | 拒絕措辭與使用者自己發動時一字不差 | 同一個哨兵、同一句話 | 助手不複製任何驗證，錯誤原封往上拋 | `TestTradingStrategyBacktestAssistantQueryChargesWhatTheAssistantSaysItCosts` 第三子案例（`ErrBacktestValidation`、且 `outcome` 為空） | ✅ conforms |
| — | 四格皆為選填 | 沒有一格進 `required` | `ArgumentSchema` | `TestTradingStrategyBacktestAssistantQueryOffersCostsAndExitsWithoutDemandingThem`（並驗證 schema 仍是合法 JSON） | ✅ conforms |

### US-08 — 畫面上填得出那兩格、讀得出那筆錢

| ID | Clause | Status |
| :--- | :--- | :--- |
| AC-08.1〜08.5 | 全部落在 `go-trading-frontend`，不在本 repo 的契約範圍 | ⏭️ deferred（見前端 repo 的同名切片） |

---

## 稽核過程中補上的缺口

**助手的出場距離只有 schema 被驗證，映射沒有。** 一開始只斷言了
`ArgumentSchema` 宣告得出 `stopLossPercentage`，而**宣告一格與真的把值送下去是兩件事**——
`ToRequestDto` 漏掉那一行，schema 測試照樣全綠，助手卻會交回一張沒有停損的成績單，
而使用者沒有任何辦法看出差別。補上
`TestTradingStrategyBacktestAssistantQuerySimulatesTheExitDistancesItWasGiven`
之後，映射本身有了守門的斷言（填了 → 止損出場、9898；不填 → 抱到底、9090）。
實作原本就是對的，缺的是證據。

---

## 沒有 orphan

新增的每一個公開行為都指得回一條 AC：

| 新增的公開行為 | 指回 |
| :--- | :--- |
| `BacktestTransactionCostsDomain`（建構 ＋ 三個方法） | AC-01.\*、AC-02.\*、AC-03.\* |
| `BacktestPositionDomain.EntryCost()` | AC-02.1、AC-05.2 |
| `BacktestAccountDomain.TotalTransactionCost()` | AC-05.\* |
| `ClosedTradeVo/Dto` 的 `EntryCost`／`ExitCost` | AC-04.5 |
| `BacktestSummaryDto.TotalTransactionCost` | AC-05.1 |
| `BacktestTransactionCostsField` | AC-01.6、AC-01.7 |
| 四個 DTO／Request 的兩個費率欄位 | AC-01.5 |
| 助手的四個新參數 | AC-07.1、AC-07.2 |

`maximumStakeScale` 與 `validatedCostPercentage` 未匯出，不構成對外行為。

---

## 已知且刻意的落差

| 落差 | 依據 |
| :--- | :--- |
| 未平倉部位的「最後剩多少」樂觀一筆出場成本 | AC-06.4，PRD 明列，並寫進成績單說明與 UL-MAP |
| 不支援每筆最低手續費，小額交易成本被算少 | PRD Out of Scope，寫進 `BacktestSummaryDto` 註解與 UL-MAP 的「回測不計入的成本」 |
| 機器人建議的部位不預留成本 | PRD Out of Scope；`position_plan_domain.go` 以零值成本模型**在程式碼上寫明**這個決定 |
| 「進場收費、出場免費」表達不出來 | AC-01.3 的代價，PRD 與 UL-MAP 皆載明；現實中無此市場 |
