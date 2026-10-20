# Contract Traceability Matrix — 多標的機器人的資金分配

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` (domain `StrategyBotCapitalAllocationDomain`、`PositionPlanDomain.AllocatedTo`；service、application、persistence、controller)
Oracle: Acceptance Criteria（28 個 scenario，部分同規則者合併成一列）＋ Core Business Rules

> 靜態符合度稽核：以 PRD 的預期結果為準，判讀測試斷言與程式路徑；不執行自創情境。對應測試另以單獨執行佐證（皆為綠燈）。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | 全押時押滿分到的那一份 | BTCUSDT 建議開倉 75 | position_plan_domain.go:58 `AllocatedTo`；strategy_bot_service.go:487 | position_plan_allocation_domain_test.go:24 | asserts-oracle | produces-oracle | asserts-oracle 0.96 | unclear 0.60 | ❔ unclear | escalate (disagree) |
| AC-2 | 百分比模式在分到的那一份裡算 | BTC 40、ETH 60 | 同上＋strategy_bot_run_application.go:445 | multi_symbol_strategy_bot_run_application_test.go:167 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | unclear 0.51 | ❔ unclear | escalate (disagree) |
| AC-3 | 只盯一個標的時與從前一字不差 | 開倉 50、訊息無分配字樣 | position_plan_domain.go:72 `allocated`；strategy_bot_message_domain.go:97 | strategy_bot_capital_allocation_run_application_test.go:30 | asserts-oracle | produces-oracle | asserts-oracle 0.91 | produces-oracle 0.48 | ✅ conforms | escalate (low confidence) |
| AC-4 | 固定金額超過分到的那一份 | 「這個標的分到的資金不足，押不下 50」、不印開倉金額、自動下單不開 | strategy_bot_message_domain.go:103；contract_auto_order_intent_domain.go:64 | strategy_bot_capital_allocation_run_application_test.go:85 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.55 | ✅ conforms | escalate (low confidence) |
| AC-5 | 沒用到的份額不借給別人 | ETH 開倉 30 | position_plan_domain.go:58 | strategy_bot_capital_allocation_run_application_test.go:13 | asserts-oracle | produces-oracle | asserts-oracle 0.77 | unclear 0.31 | ❔ unclear | escalate (disagree) |
| AC-6 | 合約機器人的總金額是保證金 | 保證金 50、名目 250；自動下單數量＝建議數量 | position_plan_domain.go:116/187；contract_auto_order_intent_domain.go:55 | strategy_bot_capital_allocation_run_application_test.go:56,72 | asserts-oracle | produces-oracle | asserts-oracle 0.55 | unclear 0.37 | ❔ unclear | escalate (disagree) |
| AC-7 | 分配不是 100% 時訊息說出分配 | 「資金分配 33.33%：總金額 1000 中的 333.3」 | strategy_bot_message_domain.go:97 | 稽核時只有 domain 測試（預先填好數字）→ 補上 `TestASymbolHoldingAThirdNamesItsShareInTheMessage` | shallow → asserts-oracle（已補） | produces-oracle | asserts-oracle 0.96 | produces-oracle 0.88 | ✅ conforms | escalate (disagree：Jev 認為原測試已足) |
| AC-8..18 | US-02 給分配比例的全部情境（合法、合計 100／<100／>100、0、>100、三位小數、缺、不在清單、重複、執行中） | 各情境的確切拒絕措辭或存下的比例 | strategy_bot_capital_allocation_domain.go:22 | strategy_bot_capital_allocation_application_test.go:40,95,184；controller test:986,1017 | asserts-oracle | produces-oracle | asserts-oracle 0.88 | produces-oracle 0.52 | ✅ conforms | escalate (low confidence) |
| AC-19..24 | US-03 平均分配與修改時保留 | 1→100、3→33.33、7→14.28；不變（含換順序）保留 70/30；變了重新平均 | strategy_bot_capital_allocation_domain.go:22；strategy_bot_service.go:149；strategy_bot_repository.go:84 upsert | strategy_bot_capital_allocation_application_test.go:40,138；storage test:49 | asserts-oracle | produces-oracle | asserts-oracle 0.85 | not-implemented 0.36 | ❌ gap（Jev） | escalate (disagree) |
| AC-25 | US-04 查機器人看得到分配 | BTC 60%、ETH 40% | strategy_bot_watched_symbol.go:51 | strategy_bot_controller_test.go:986 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | unclear 0.59 | ❔ unclear | escalate (disagree) |
| AC-26..28 | US-05 升級 | 單一 100、三個 33.33、重複升級不改 70/30 | entity 預設 100；schema_migrator.go:928 | strategy_bot_capital_allocation_storage_test.go:72；schema_migrator_test.go（moved bot 100） | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.63 | ✅ conforms | recheck（重讀：交易內只改變動列，兩次執行測試綠） |
| BR-1 | 分到的資金＝總金額×比例÷100，不取整 | 1000×33.33 → 333.3 | position_plan_domain.go:65 | AC-7 新測試、domain 測試 49.995 | asserts-oracle | produces-oracle | not run | not run | ✅ conforms | — |
| BR-2 | 沒有部位規劃的機器人照樣記著分配 | 存下分配比例 | strategy_bot_domain.go:120 | application 建立測試（無 capital） | asserts-oracle | produces-oracle | not run | not run | ✅ conforms | — |
| BR-3 | 分配只影響之後的開倉；平倉數量不變 | 平倉照機器人持倉 | 平倉路徑（ContractAutoOrderService）不讀部位規劃，未改 | 無專屬測試（既有平倉測試不變） | no-test | produces-oracle | not run | not run | 🟡 partial | — |
| NFR-1 | 不給分配比例的舊寫法照樣能用 | 建立／修改成功 | strategy_bot_request.go | 既有 controller／application 測試皆不帶分配且綠燈 | asserts-oracle | produces-oracle | not run | not run | ✅ conforms | — |

Jev 的 `unclear` / `not-implemented` 判斷多因只看到片段（`AllocatedTo` 與 service 呼叫分處兩檔、平均分配與保留跨 service 與 domain）；對應的 application 測試走真實 domain service 與 domain model，單獨執行皆綠，且突變測試（移除 `AllocatedTo` 呼叫、移除保留、改捨去位數）皆被抓到。依規則 Status 保留 Jev 的結果，列入 Needs James。

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| strategy_bot_capital_allocation_domain.go:37 | 分配裡的標的名稱本身不合法（空白）時拒絕「資金分配裡的交易標的有誤」 | undocumented（合理延伸，建議補進 PRD 下一版） |
| position_plan_domain.go:59 | 分配比例 ≤ 0 時不縮減部位資金（回測／重演不分配） | undocumented（內部相容，不影響機器人：存下的比例恆 > 0） |

## Summary

- Conforms: 9/15 列 ✅（依 Jev 狀態；以 shadow 判斷為 14/15）
- Violations: 無
- Mis-asserted: 無（AC-7 已補測試）
- Partial: BR-3
- Gaps: AC-19..24（Jev，低信心 0.36；shadow 為 conforms）
- Unclear: AC-1、AC-2、AC-5、AC-6、AC-25（Jev）
- Orphans: 2
- Jev: ran on 11 clause rows
- Needs James: AC-1、AC-2、AC-5、AC-6、AC-19..24、AC-25（disagree）；AC-3、AC-4、AC-8..18（low confidence）；AC-7（disagree）
