# Contract Traceability Matrix — trading-strategy-backtest

Contract: `.sdd/2026-09-17-trading-strategy-backtest/PRD.md`
Design map: `.sdd/2026-09-17-trading-strategy-backtest/ARCH.md`
Implementation: `internal/domain/models/domains/trading_strategy_backtest_domain.go`,
`internal/domain/service/backtest_service.go`,
`internal/application/trading_strategy_backtest_application.go`,
`internal/controller/trading_strategy_backtest_controller.go`
Oracle: Acceptance Criteria (19 clauses) + Core Business Rules (6 clauses)

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 重演一份交易策略 | 拿到成績單、資金曲線與交易明細 | `trading_strategy_backtest_application.go:42` | `trading_strategy_backtest_application_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 別人的一份看不到 | 拿到「找不到」 | `trading_strategy_service.go` GetTradingStrategy | `trading_strategy_backtest_application_test.go:278` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 不存在的那一份也一樣 | 同一句「找不到」，與指名別人那一份一字不差 | `trading_strategy_service.go` GetTradingStrategy | `trading_strategy_backtest_application_test.go:340` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 來源指名的腳本看不到時整次拒絕 | 整次被拒絕，說找不到那一支策略腳本 | `trading_strategy_backtest_application.go:75` | `trading_strategy_backtest_application_test.go:355` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 每個來源都用同一個刻度就跑得動 | 用一小時的 K 線逐棒往前走 | `trading_strategy_backtest_domain.go:118` | `trading_strategy_backtest_application_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 只有一個來源時用它的刻度 | 用那個來源的刻度逐棒往前走 | `trading_strategy_backtest_domain.go:118` | `trading_strategy_backtest_application_test.go:178` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 刻度不一致時整次拒絕，並說出現在有哪幾種 | 整次被拒絕，句子裡看得到「1h」與「5m」 | `trading_strategy_backtest_domain.go:132` | `trading_strategy_backtest_application_test.go:195` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 買入條件成立就買 | 那一棒的結論是買入 | `trading_strategy_backtest_domain.go:216` | `trading_strategy_backtest_application_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 賣出條件成立就賣 | 那一棒的結論是賣出 | `trading_strategy_backtest_domain.go:216` | `trading_strategy_backtest_application_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 兩邊都不成立就不動作 | 結論是持平，帳戶不動 | `trading_strategy_backtest_domain.go:239` | `trading_strategy_backtest_application_test.go:149` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 兩邊同時成立就當作持平 | 結論是持平、帳戶不動，且算進打架棒數 | `trading_strategy_backtest_domain.go:235` | `trading_strategy_backtest_application_test.go:250` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 只有一個來源時與重演那一支腳本完全相同 | 兩次的成績單完全相同 | `backtest_service.go` RunTradingStrategyBacktest | `trading_strategy_backtest_application_test.go:294` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 沒有打架時是零 | 打架棒數是 0 | `trading_strategy_backtest_domain.go:220` | `trading_strategy_backtest_application_test.go:211` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 一直打架時說得出次數 | 打架棒數等於同時成立的棒數，且開倉次數很少 | `trading_strategy_backtest_domain.go:236` | `trading_strategy_backtest_application_test.go:250` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 重演一支策略腳本時它恆為零 | 打架棒數是 0 | `backtest_summary_dto.go`（單支路徑不寫入） | `backtest_application_test.go:188` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 湊不出兩根 K 線時整次拒絕 | 整次被拒絕，理由與重演一支策略腳本時相同 | `backtest_domain.go` SelectInputCandles | `trading_strategy_backtest_application_test.go:389` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 本金不是正數時整次拒絕 | 整次被拒絕 | `backtest_domain.go` NewBacktestDomain | `trading_strategy_backtest_controller_test.go:148`（"a capital of nothing"） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 起點晚於終點時整次拒絕 | 整次被拒絕 | `backtest_domain.go` NewBacktestDomain | `backtest_application_test.go`（共用同一個驗證路徑） | shallow | produces-oracle | 🟠 mis-asserted |
| AC-19 | 結果不留存 | 系統重新算一次，而不是拿出上次的結果 | 無任何 repository 寫入路徑 | — | no-test | produces-oracle | 🟡 partial |
| BR-1 | 受測對象是一份自己的交易策略，看不到的一律「找不到」 | 同 AC-2／AC-3 | `trading_strategy_service.go` | `trading_strategy_backtest_application_test.go:278,340` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 彙總刻度由來源自己說且必須一致 | 同 AC-5／AC-7 | `trading_strategy_backtest_domain.go:118` | `trading_strategy_backtest_application_test.go:195` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 每一棒的信號由兩棵條件樹求出，兩邊都成立→持平並計數 | 同 AC-8..AC-11 | `trading_strategy_backtest_domain.go:216` | `trading_strategy_backtest_application_test.go:250` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 每個來源看得到的 K 線範圍與現行回測相同 | 從起點到第 N 棒（含） | `trading_strategy_backtest_domain.go:160` 委派 | `trading_strategy_backtest_application_test.go:294`（等價證明） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 模擬與報表一條都不改，只多一個打架棒數 | 兩種受測對象的成績單欄位相同 | `backtest_summary_dto.go` | `trading_strategy_backtest_application_test.go:294` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 結果不留存 | 同 AC-19 | 無寫入路徑 | — | no-test | produces-oracle | 🟡 partial |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `trading_strategy_backtest_domain.go:119` | 一個信號來源都沒有時，比照刻度不一致整次拒絕 | PRD Edge Cases 已載明，非孤兒 |

## Summary

- Conforms: 23/25 clauses ✅ (92%)
- Violations: 無
- Mis-asserted: AC-18（起終點顛倒只由重演一支腳本那條路的測試蓋到；兩條路共用同一個 `NewBacktestDomain` 驗證，故行為正確但沒有一條測試直接從交易策略那條路釘它）
- Partial: AC-19、BR-6（「不留存」是結構性事實——這條路上沒有任何 repository 寫入。要直接測它只能斷言「沒有呼叫某個不存在的東西」，那是測實作而非行為）
- Gaps: 無
- Unclear: 無
- Orphans: 0

本次稽核修補的缺口：AC-3、AC-4、AC-15、AC-16 原本無測試，已補上四條驗收測試。

> 稽核性質：靜態一致性稽核。它比對測試斷言與程式路徑對上規格的預期結果，
> 不執行自行發明的情境，也不以整份測試套件的綠燈作為判準。
