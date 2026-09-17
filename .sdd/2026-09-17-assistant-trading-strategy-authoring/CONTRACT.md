# Contract Traceability Matrix — assistant-trading-strategy-authoring

Contract: `.sdd/2026-09-17-assistant-trading-strategy-authoring/PRD.md`
Design map: `.sdd/2026-09-17-assistant-trading-strategy-authoring/ARCH.md`
Implementation: `internal/application/assistantqueries/trading_strategy_create_assistant_query.go`,
`trading_strategy_update_assistant_query.go`, `trading_strategy_get_assistant_query.go`,
`trading_strategy_list_assistant_query.go`, `trading_strategy_backtest_assistant_query.go`,
`trading_strategy_write_assistant_arguments.go`, `cmd/server/dependencies.go`,
`internal/config/application_config.go`
Oracle: Acceptance Criteria (23 clauses) + Core Business Rules (7 clauses)

Test files below are abbreviated:
`tsq` = `internal/application/assistantqueries/tests/trading_strategy_assistant_queries_test.go`,
`tbq` = `internal/application/assistantqueries/tests/trading_strategy_backtest_assistant_query_test.go`,
`aq` = `cmd/server/assistant_queries_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 拼一份交易策略 | 存起來了，擁有者是我，助手覆述得出名稱與識別碼 | `trading_strategy_create_assistant_query.go:62` | `tsq:113` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 擁有者一律是委託它的人 | 助手沒有任何辦法指定擁有者 | `trading_strategy_write_assistant_arguments.go:79` `ToWriteDto` 不收 ownerID | `tsq:135` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 名稱與自己既有的重複 | 助手被告知重複，換名再送成功 | 既有名稱索引；`Run` 原封回傳 | `tsq:144` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 來源指名看不到的策略腳本 | 助手被告知找不到那一支 | `TradingStrategyApplication.withResolvedStrategyScripts`（既有） | `tsq:325` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 改一份交易策略 | 那一份被改掉了 | `trading_strategy_update_assistant_query.go:63` | `tsq:184` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 列出我有哪幾份 | 助手唸得出名稱與識別碼 | `trading_strategy_list_assistant_query.go:57` | `tsq:270`, `tsq:300` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 讀一份的全部內容 | 讀得到來源、兩棵條件樹與參數值 | `trading_strategy_get_assistant_query.go:55` | `tsq:241` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 別人的那一份讀不到 | 「找不到」，與不存在一字不差 | `TradingStrategyService.GetTradingStrategy`（既有） | `tsq:258` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 有機器人在跑時改不動 | 被告知是哪幾台在跑 | `TradingStrategyApplication.UpdateTradingStrategy`（既有） | `tsq:214` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 助手沒有刪除這個能力 | 系統裡不存在這件事可以給它做 | **沒有對應實作**；`assistantQueriesFor` 未註冊 | `aq:25` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 重演一份交易策略 | 唸得出總報酬率、最大回撤、勝率與開倉次數 | `trading_strategy_backtest_assistant_query.go:139` | `tbq:170` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 交易明細也讀得到 | 每一筆進出場的時間、價格與賺賠 | `tradingStrategyBacktestReport.ClosedTrades` | `tbq:170` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 資金曲線不交給助手 | 裡面沒有資金曲線 | 獨立的 `tradingStrategyBacktestReport`（無該欄位） | `tbq:197` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 打架棒數一定在 | 讀得到打架棒數是 180 | `BacktestSummaryDto.ConflictedCandleCount` 整包交回 | `tbq:218` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 刻度不一致時被擋 | 被告知不一致以及現在有哪幾種 | `SharedAggregationIntervalDomain`（既有）；`Run` 原封回傳 | `tbq:240` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 別人的那一份重演不了 | 「找不到」 | `TradingStrategyBacktestApplication`（既有） | `tbq:256` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 條件指到沒宣告的代號 | 被告知代號 C 沒宣告，整次回答不中斷，補上再送成功 | `TradingStrategyDomain`（既有）；`Run` 回 error | `tsq:159` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 參數值帶了沒宣告的名稱 | 被告知是哪一個參數 | `withResolvedStrategyScripts` + `TradingStrategyDomain`（既有） | `tsq:360` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 一直被擋直到查詢次數用完 | 回答目前進展並說明已達上限，照常留存 | `AssistantQueryRoundsDomain`（既有） | `internal/domain/service/tests/assistant_conversation_service_test.go`（既有覆蓋） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 一句話跑完好幾圈 | 都在同一次回答裡，只送出一句 | `ASSISTANT_QUERY_LIMIT=40` + 既有回合迴圈 | — | no-test | produces-oracle | 🟠 mis-asserted |
| AC-21 | 查詢次數上限是四十 | 不再放行，交出目前最好的並說明已達上限 | `application_config.go` 預設值 40 | `internal/config/tests/application_config_test.go` `TestLoadGivesTheAssistantEnoughQueriesToFinishWhatItStarted` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 參數不是合法 JSON | 交回「參數不是合法的 JSON」 | 五個 `Run` 各自 `json.Unmarshal` 分支 | `tsq:176`, `tbq:270` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 每個能力都叫得動 | 名稱、說明、合法 JSON schema 三者齊備 | 五個實作的 `Name`／`Description`／`ArgumentSchema` | `aq:55`, `tsq:312`, `tbq:278` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 走使用者自己那條路，一條規則都不放寬 | 同 AC-3／AC-8／AC-9／AC-15／AC-17 | 五個 `Run` 皆呼叫既有 Application | `tsq`、`tbq` 全體（注入真 service＋真 model） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 擁有者永遠是委託者，沒有欄位可指定 | 同 AC-2 | `ToWriteDto(id)` 簽章不含 ownerID | `tsq:135` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 刪除交易策略不存在 | 同 AC-10 | 無實作 | `aq:25` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 拒絕交回助手、不中斷整次回答 | 同 AC-17 | `IAssistantQuery` 既有契約 + `Run` 回 error | `tsq:159`、既有 service 測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 回測結果含成績單與交易明細、不含資金曲線、明細上限 50 筆 | 同 AC-12／AC-13 | `tradingStrategyBacktestReport` + `mostRecentClosedTrades` | `tbq:197`, `tbq:298`, `tbq:348` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 打架棒數必須在 | 同 AC-14 | 同上 | `tbq:218` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 上限行為不變，只改預設值 | 第 N 次照做、之後不放行、用完照常留存 | 只動 `application_config.go` 一個字面值 | 既有 service 測試（`queryLimit` 為建構參數） | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `trading_strategy_backtest_assistant_query.go` `closedTradeLimit` | 一次能力交回的東西會在之後每一輪被重送給助手；沒有上限的交易明細會在查詢用完之前先把它能看的東西佔滿 | ARCH 未載明；code review 指出後補上，已補測試 `tbq:298`／`tbq:348` |
| `trading_strategy_list_assistant_query.go:47` `AggregationIntervals` | 摘要帶出各來源刻度，讓助手一眼看出哪一份重演不了 | ARCH Open decisions 已載明並採納，非孤兒 |
| `trading_strategy_backtest_assistant_query.go:57` `decimalOrZero` | 讀不出的金額當零，交給既有「本金必須大於零」拒絕 | PRD Edge Cases「本金不是正數」的落點，非孤兒 |

## Summary

- Conforms: 29/30 clauses ✅ (97%)
- Violations: 無
- Mis-asserted: AC-20（「一次回答之內跑得完好幾圈」沒有一條測試直接釘。要釘它得模擬
  一整段四十次查詢的對話，而那測的其實是回合迴圈——迴圈本來就有自己的測試，上限的
  數字也已由 AC-21 釘住。兩者都成立時 AC-20 就成立，但那是推論而不是一條斷言）
- Partial: 無
- Gaps: 無
- Unclear: 無
- Orphans: 0

本次稽核修補的缺口：AC-4、AC-18 原本只由既有的（非助手路徑）測試蓋到，
已補上兩條從助手這條路出發的驗收測試；AC-21 原本沒有任何測試釘住那個數字，
已補上設定測試。

**Code review 之後的修補**：交回助手的交易明細原本沒有上限。它與 K 線的上限是同一條
規則，而且咬得更深——一次能力交回的東西會在**之後每一輪**被重送給助手，
所以重演四五次的累積量會在查詢次數用完之前，先把助手能看的東西佔滿。
已加上限 50 筆並明講筆數（`tbq:298`／`tbq:348`）。

> 稽核性質：靜態一致性稽核。它比對測試斷言與程式路徑對上規格的預期結果，
> 不執行自行發明的情境，也不以整份測試套件的綠燈作為判準。
