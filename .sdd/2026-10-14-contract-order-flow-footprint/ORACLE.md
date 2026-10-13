# Oracle — 合約訂單流（足跡）

Expected outcomes pinned from the PRD before implementation. Tests copy their expected values from here.

| # | Case | Expected outcome | Owner |
|---|---|---|---|
| 1 | 合約腳本宣告 `orderFlowPriceStep` 預設 10 | 建立成功；讀回參數 kind `orderFlowPriceStep`、default 10 | StrategyScriptDomain / ParametersDomain |
| 2 | K 線腳本宣告它 | 拒絕（`ErrStrategyScriptValidation`），訊息含「訂單流價位寬度只提供給合約行情種類的策略腳本」 | StrategyScriptDomain |
| 3 | 宣告兩個 | 拒絕，訊息含「訂單流價位寬度至多宣告一個」 | ParametersDomain |
| 4 | 預設 0 | 拒絕，訊息含「訂單流價位寬度」且含「大於零」 | ParametersDomain |
| 5 | 預設 0.5 | 成功 | ParametersDomain |
| 6 | Applying 給 -5 | 拒絕（`ErrStrategyScriptParameterValidation`），含「大於零」 | ParametersDomain |
| 6b | 現貨指標計算自帶算式宣告它 | 整次拒絕（計算驗證錯誤），含「訂單流價位寬度只提供給合約行情種類的策略腳本」 | IndicatorCalculationService |
| 7 | 宣告寬度 10 做合約計算 | 每格 `Footprint.PriceStep == 10` | ContractOrderFlowService |
| 8 | 沒宣告 | 每格 Footprint 零值；proxy 沒被呼叫 | ContractOrderFlowService |
| 9 | 預設 10、這次 25 | 每格 PriceStep 25 | ContractOrderFlowService |
| 10 | 寬度 10；09:00 有 84157 買 2、84151 賣 1、84163 買 3 | 09:00 格 Levels = [{84150, buy 2, sell 1, count 2}, {84160, buy 3, sell 0, count 1}]；Delta 4；POC 84150 | Reach.Footprinting |
| 11 | 成交價 84160、寬度 10 | 價位 84160 | PriceStepDomain |
| 11b | 成交價 0.3、寬度 0.1 | 價位 0.3（不是 0.2） | PriceStepDomain |
| 12 | 一分鐘格；成交於 09:01:00.000 | 屬於 09:01 格，09:00 格無此成交 | Ledger / Reach |
| 13 | 一小時格；09:00 與 09:30 的 84150 各買 2 | 09:00 格 84150 buy 4 | Reach |
| 14 | 一格沒成交 | Levels 空、Delta 0、POC 0、PriceStep 為這次的值 | Reach |
| 15 | 一格 1,000 價位 | 成功 | Reach |
| 16 | 一格 1,001 價位 | `ErrContractOrderFlowRefused`，含「一格至多 1,000 個價位」 | Reach |
| 16b | 合計 2,000,000 | 成功 | Reach |
| 16c | 合計 2,000,001 | `ErrContractOrderFlowRefused`，含「一次至多 2,000,000 個價位」 | Reach |
| 17 | 今天 10/14；需要 10/01–10/10 | 十天都以 `VisitDailyTrades` 取，沒有 `VisitTradesBetween` | ContractOrderFlowService |
| 18 | 今天 10/14；10/13 檔不存在 | 10/13 改以 `VisitTradesBetween(10/13 00:00, 10/14 00:00)` | ContractOrderFlowService |
| 19 | 需要今天 | `VisitTradesBetween(今天 00:00, 最後一格收盤)` | ContractOrderFlowService |
| 19b | 更早的日子檔不存在 | `ErrContractOrderFlowUnavailable` | ContractOrderFlowService |
| 20 | 來源回錯誤 | `ErrContractOrderFlowUnavailable`，含「讀不到訂單流」；無結果 | ContractOrderFlowService |
| 21 | 跨 31 個日曆日 | 成功 | Alignment.OrderFlowReach |
| 22 | 跨 32 個日曆日 | `ErrContractOrderFlowRefused`，含「訂單流一次至多涵蓋 31 天」 | Alignment.OrderFlowReach |
| 23 | 同標的同段同寬度算兩次 | proxy 每一天只被呼叫一次；兩次結果相同 | Memory |
| 24 | 同段換寬度 25 | 第二次 PriceStep 25、價位依 25（proxy 再被呼叫） | Memory |
| 24b | 今天已補到 10:00，再算到 10:05 | 第二次只 `VisitTradesBetween(10:00, 10:05)` | ContractOrderFlowService |
| 25 | 合約回測宣告寬度 | 交給逐格執行的每一格帶 Footprint | ContractBacktestService |
| 26 | 交易策略兩來源寬度 10 與 50 | 第一來源的格子 PriceStep 10、第二來源 50 | ContractBacktestService |
| 27 | 機器人備齊時間到 | `ErrContractOrderFlowNotReady`；已拿到的分鐘留在記憶中；輪次失敗判定為跳過 | Service + RoundFailureDomain |
| 28 | 來源不答話 | 輪次跳過（不停機器人） | RoundFailureDomain |
| 29 | 超過 31 天 | 輪次停下，停擺原因 `orderFlowRefused` | RoundFailureDomain |
| 30 | 助手說明 | system prompt 與參數 schema 都提到 `orderFlowPriceStep` 與 `Footprint` | assistant |
