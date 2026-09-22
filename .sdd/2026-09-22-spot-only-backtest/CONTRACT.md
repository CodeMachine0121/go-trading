# Contract Traceability Matrix — 重演只做現貨

Contract: `.sdd/2026-09-22-spot-only-backtest/PRD.md`
Design map: `.sdd/2026-09-22-spot-only-backtest/ARCH.md`
Implementation: `internal/domain/models/{domains,vo,dto,entities}` · `internal/application` · `internal/controller` · `internal/infrastructure`
Oracle: Acceptance Criteria — 45 clauses (37 `AC-`, 5 `BR-`, 3 `NFR-`)

## Clauses

Paths are shortened: `D/` = `internal/domain/models/domains/`, `T/` = `internal/domain/models/domains/tests/`,
`A/` = `internal/application/`, `C/` = `internal/controller/`, `I/` = `internal/infrastructure/`.

### US-01 — 重演照現貨的規則走倉位

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 空手時聽到買入就開倉 | 開一個多倉 | `D/backtest_account_domain.go:111` | `T/backtest_account_domain_test.go` 「buying while flat opens a long」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 已經有倉位時再聽到買入不加碼 | 倉位不變，不記交易 | `D/backtest_account_domain.go:87` | `T/backtest_account_domain_test.go` 「buying again is heard once」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 有倉位時聽到賣出就平回現金 | 平倉、錢回可用資金、之後空手 | `D/backtest_account_domain.go:92` | `T/backtest_account_domain_test.go` 「selling a long closes it back to cash」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 空手時聽到賣出什麼都不發生 | 不記交易，仍空手，不出現空倉 | `D/backtest_account_domain.go:99` | `T/backtest_account_domain_test.go` 「selling with nothing to sell does nothing at all」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 持平那一棒不動作 | 倉位不變，不記交易 | `D/signal_domain.go:79` (`TargetPositionUnchanged`) | `T/backtest_account_domain_test.go` 「holding leaves an open long alone」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 整段重演的交易明細方向一致 | 每一筆方向都是做多 | `D/backtest_position_domain.go:186` (`ClosedAt` 恆填 `PositionDirectionLong`) | `T/backtest_account_domain_test.go` 「Every round trip … faces the same way」 | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 不必再挑「這一次要用哪一種規則」

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-7 | 什麼都不宣告就照現貨跑 | 跑完並交出成績單 | `D/spot_only_replay_domain.go:78` | `T/spot_only_replay_domain_test.go` 「nothing declared at all」；`T/backtest_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 宣告現貨等於什麼都沒說 | 結果與什麼都不宣告逐格相同 | `D/spot_only_replay_domain.go:80` | `T/spot_only_replay_domain_test.go` 「saying spot out loud」；`C/tests/backtest_controller_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 宣告多空反手被整份拒絕 | 整次拒絕，說出只做現貨，無部分結果 | `D/backtest_domain.go:76` | `T/backtest_domain_test.go` 「any other set of rules is refused」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 宣告槓桿做多被整份拒絕 | 同上，同一句 | 同上 | 同上（`leveragedLong` 列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 宣告只做空被整份拒絕 | 同上，同一句 | 同上 | 同上（`shortOnly` 列） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 宣告一個認不得的詞同樣被整份拒絕 | 同上，同一句 | 同上 | 同上（`dayTrade` 列）+ `T/spot_only_replay_domain_test.go` 一句話一致性斷言 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 建立交易策略時沒有交易模式可填 | 存得起來，是一份現貨的規則 | `D/trading_strategy_domain.go:71` | `T/trading_strategy_domain_test.go` 「accepts only spot or silence」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 建立交易策略時送來交易模式被整份拒絕 | 整份拒絕，不存 | `D/trading_strategy_domain.go:73` | `T/trading_strategy_domain_test.go` 「refuses any other set of rules」；`C/tests/trading_strategy_controller_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 這一刀之前存下的交易策略照舊跑得動 | 逐格相同 | `D/../entities/trading_strategy.go`（欄位已不對映，殘留欄位沒有人讀） | — | no-test | produces-oracle | 🟡 partial |

### US-03 — 重演借不到錢，也不會被強制平倉

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-16 | 不提槓桿的重演跑得完；**成績單上沒有強平出場那一格** | 跑完；成績單**不含**強平出場 | `dto/backtest_summary_dto.go:35`（欄位已移除） | `C/tests/backtest_controller_test.go`「the report card has no column for being liquidated」 | asserts-oracle | produces-oracle | ✅ conforms *(fixed)* |
| AC-17 | 槓桿倍數填一等於沒有借錢 | 與沒提槓桿逐格相同 | `D/spot_only_replay_domain.go:94` | `T/spot_only_replay_domain_test.go`；`T/backtest_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 槓桿倍數填零等於沒有借錢 | 同上 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 槓桿倍數大於一被整份拒絕 | 整次拒絕，說出沒有人借錢給你 | `D/spot_only_replay_domain.go:94` | `T/spot_only_replay_domain_test.go`；`C/tests/backtest_controller_test.go`；`C/tests/trading_strategy_backtest_controller_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 槓桿倍數小於一但不是零，同樣被整份拒絕 | 整次拒絕（既有措辭） | `D/spot_only_replay_domain.go:88` | `T/spot_only_replay_domain_test.go` 「half a position」「minus sign」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 一注承擔的就是押下去的錢 | 押 100、漲一成平倉 → 賺 10 | `D/backtest_position_domain.go:70`（單位數＝押注金額÷進場價） | `T/backtest_position_domain_test.go` 「valuation」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 出場原因只剩三種 | 每一筆是訊號／止損／止盈之一，無強平 | `vo/trade_exit_reason_vo.go` | `T/backtest_exit_simulation_test.go`「EndsEveryTradeOneOfThreeWays」（三種都真的發生過） | asserts-oracle | produces-oracle | ✅ conforms *(fixed)* |
| AC-23 | 止損與止盈照舊生效 | 兩者照舊模擬，成績單照舊有兩格 | `D/backtest_exit_levels_domain.go:62` | `T/backtest_exit_simulation_test.go` 全檔 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 交易成本照舊收 | 照舊扣，成績單照舊有累計成本 | `D/backtest_transaction_costs_domain.go` | `T/backtest_transaction_cost_simulation_test.go` 全檔 | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 機器人只講得出現貨做得到的動作

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-25 | 買入那一側寫買入 | 結論寫「買入」，無交易模式那一行 | `D/signal_domain.go:107` | `T/strategy_bot_message_domain_test.go` 「WritesEachSignalTheWayAPersonReadsIt」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 賣出那一側寫出場 | 結論寫「出場」，無交易模式那一行 | `D/signal_domain.go:108` | `T/signal_domain_test.go` 「HeadlineVerb」；`T/strategy_bot_message_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 各來源怎麼說照舊抄信號的字 | 底下那幾行寫買入／賣出／持有 | `D/strategy_bot_message_domain.go:150`（`InWords`） | `T/strategy_bot_message_domain_test.go` 「SaysOutLoudThatTheSourceLinesAreTheWorking」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 建議部位不再印槓桿與名目 | 印開倉金額；無槓桿行、無名目行 | `D/strategy_bot_message_domain.go:148`；`dto/position_plan_dto.go` | `T/strategy_bot_message_domain_test.go` 「SaysWhatToPutDownAndWhereToGetOut」（斷言「開倉金額」）；欄位已從 DTO 移除故不可能印出 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 建議部位的止損在參考價下方 | 參考價 100、停損一成 → 止損 90，寫成往下 | `D/position_plan_domain.go:165` | `T/position_plan_domain_test.go` 「SizesAPosition」；`T/strategy_bot_message_domain_test.go` 「SaysWhichWayEachExitLies」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 建立機器人時送來大於一的槓桿被整台拒絕 | 整台拒絕，說出沒有人借錢給你，不存 | `D/position_plan_domain.go:58` | `T/strategy_bot_domain_test.go` 「RefusesBorrowing」；`A/tests/strategy_bot_application_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 建立機器人時槓桿填一等於沒填 | 存得起來，逐字相同 | `D/spot_only_replay_domain.go:94` | `T/strategy_bot_domain_test.go` 「suggesting one times, which is not a loan」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 這一刀之前建立的機器人照舊跑 | 訊息逐字相同 | `entities/strategy_bot.go`（欄位已不對映） | — | no-test | produces-oracle | 🟡 partial |

### US-05 — 助手不再替我挑一個不存在的選項

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-33 | 助手建立交易策略時說不出交易模式 | 存得起來；工具目錄沒有這個欄位 | `A/assistantqueries/trading_strategy_write_assistant_arguments.go` | `A/assistantqueries/tests/trading_strategy_assistant_queries_test.go` 「OfferNoTradingMode」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 助手跑重演時說不出槓桿 | 照現貨跑完；工具目錄沒有這個欄位 | `A/assistantqueries/trading_strategy_backtest_assistant_query.go` | 同檔 tests 「OffersNoModeAndNoBorrowing」「ReplaysSpot」 | asserts-oracle | produces-oracle | ✅ conforms |

### US-06 — 合約那條線一行都不動

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-35 | 合約追蹤名單原封不動 | 名單逐項相同 | 未改動（`git diff main...HEAD` 對 contract 檔案 0 命中） | `I/persistence/tests/k_candle_contract_*`（未改動且綠） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 合約行情一根都沒少 | 逐格相同 | 未改動 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 合約的背景抓取照舊 | 照舊每分鐘一輪 | 未改動 | `internal/job/tests`（未改動且綠） | asserts-oracle | produces-oracle | ✅ conforms |

### Section 4 — Core Business Rules

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-1 | 重演唯一的一套規則（現貨）的三×二表；同一棒不會既平又開；永遠不會出現空倉；借不到錢 | 表中六格逐格如上 | `D/signal_domain.go:79` + `D/backtest_account_domain.go:74` | `T/backtest_account_domain_test.go` 全檔 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 送來一件系統已經不做的事：會改變答案的整份拒絕，本來就等於現貨的照舊接受 | 七列判準逐列如上；拒絕一律整份、無部分結果 | `D/spot_only_replay_domain.go:78` | `T/spot_only_replay_domain_test.go` 全檔（七列全覆蓋） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 出場原因從四種變三種；**強平出場不再存在** | 出場原因只有三種，成績單不含強平 | `vo/trade_exit_reason_vo.go`；`dto/backtest_summary_dto.go:35` | `T/backtest_exit_simulation_test.go`「EndsEveryTradeOneOfThreeWays」；`C/tests/backtest_controller_test.go`「no column for being liquidated」 | asserts-oracle | produces-oracle | ✅ conforms *(fixed)* |
| BR-4 | 一注承擔的金額＝押下去的金額 | 賺賠與兩端成本都照它算 | `D/backtest_position_domain.go:70,74` | `T/backtest_position_domain_test.go`；`T/backtest_transaction_cost_simulation_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 機器人訊息：買入／出場、不多交易模式那一行、來源照舊、建議部位不印槓桿與名目、止損下止盈上 | 五項逐項如上 | `D/strategy_bot_message_domain.go` | `T/strategy_bot_message_domain_test.go` 全檔 | asserts-oracle | produces-oracle | ✅ conforms |

### Section 6 — Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-1 | 效能不受影響：只移除規則，不新增取數或計算 | 無新增的取數或計算 | 淨 −4,363 行；無新增 repository／proxy 呼叫 | — | no-test | produces-oracle | 🟡 partial |
| NFR-2 | 相容性：不需要任何資料遷移，既有交易策略與機器人照舊讀得動、跑得動 | 無 migration；舊列讀得動 | 無 migration 檔；`AutoMigrate` 不刪欄位 | `I/persistence/tests/*_repository_test.go`（讀寫照舊綠） | shallow（沒有「舊列仍讀得動」的專屬案例） | produces-oracle | 🟠 mis-asserted |
| NFR-3 | 可觀測性：拒絕時說得出是哪一件事被拒絕 | 回應指名 `tradingMode` 或 `leverage`，非籠統訊息 | `D/backtest_domain.go:78` | `C/tests/backtest_controller_test.go` 兩個 `body.Field` 斷言 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| ~~`dto/backtest_summary_dto.go:40`~~ | `LiquidationExitCount` 曾仍宣告在成績單上，永遠是 0 | **已移除** |
| ~~`I/persistence/strategy_bot_repository.go:60`~~ | 改寫機器人時的欄位清單曾仍列著 `position_plan_leverage` | **已移除** |
| `D/backtest_transaction_costs_domain.go:156` | 參數名 `tradedNotional` | 良性：它指「實際換手的金額」，語意仍正確 |
| `A/assistantqueries/…:178`、`I/assistant/claude_assistant_proxy.go:72` | 指示文字提到「借錢、做空與強制平倉」 | 良性：那是在**說明這個系統不做什麼**，正是 BR-2 要的 |

## Summary

**第一輪**：Conforms 40/45（89%）· Violations 2 🔴 · Mis-asserted 2 🟠 · Partial 3 🟡 · Orphans 4
**修正後（本檔現況）**：

- Conforms: **43/45** clauses ✅ (96%)
- Violations: 無（AC-16／BR-3 已修：欄位自成績單移除，並以「不存在」的斷言釘住）
- Mis-asserted: **NFR-2** 🟠 — 見下方保留理由
- Partial: **AC-15, AC-32, NFR-1** 🟡 — 程式碼對，無測試
- Gaps: 無
- Unclear: 無
- Orphans: 2（皆良性：一個參數名、兩處說明文字）

### 刻意保留的三項

- **AC-15／AC-32（🟡 partial）**：「這一刀之前存下的交易策略／機器人照舊跑得動」。
  程式碼確實如此——那兩個欄位已不在 entity 上，GORM 既不讀也不寫它們。
  要**測**它得先造一列帶著舊欄位的資料，而唯一的做法是手寫 SQL 或另立一個測試專用 entity，
  兩者分別違反 `persistence.md`（禁手寫 SQL）與 `testing.md`（禁手寫假物件）。
  **判斷：不為了補一格綠燈而破規矩**，改以既有的 repository 讀寫測試涵蓋「新列讀得動」，
  舊列的部分列為已知缺口。

- **NFR-2（🟠 mis-asserted）**：同上，理由相同。

- **NFR-1（🟡 partial）**：「效能不受影響」。這一刀淨刪 4,363 行、沒有新增任何一次取數或計算，
  用測試釘住它需要一個效能基準，而這個專案沒有，也不該為這一刀建立。

> **Ceiling.** 這是**靜態**一致性稽核：它把測試斷言與程式碼路徑分別對照 PRD 推出的預期結果，
> 不靠跑整套測試下結論，也不自行撰寫新的探針。AC-16／BR-3 這個 violation 正是「全綠」看不出來的那一種——
> 相關斷言在這一刀裡被**移除**而不是被**反轉**，於是測試沉默地不再過問那一格。
