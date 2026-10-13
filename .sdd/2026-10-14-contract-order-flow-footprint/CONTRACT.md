# Contract Traceability Matrix — 合約訂單流（足跡）

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` (go-trading backend)
Oracle: Acceptance Criteria (40 clauses: 34 AC + 4 BR + 1 edge case + 1 NFR)

> Static conformance audit. Oracles were written from the PRD before reading code; each test and each code path was judged against them separately. References name the test or method rather than a line number, so comment edits do not break them.
> Two clauses were 🟡 partial at the first pass (no test asserted the oracle): **AC-29** and **EDGE-1**. Tests were added for both (`TestAContractBotWhoseOrderFlowIsReadyConcludesAsUsual`, `TestADayWhoseArchiveBreaksHalfwayIsNotRemembered`) and each was falsified before being counted.

> **After code review (PR #105):** added clauses AC-35 to AC-41 for the review fixes (finest step, submit-time 31-day refusal, settling wait, timeout vs not-ready, too many levels skips, waiting behind another gatherer). Each has a falsified test.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 合約策略腳本宣告訂單流價位寬度 | 建立成功；讀回參數種類是訂單流價位寬度、預設值 10 | `StrategyScriptDomain` → `StrategyScriptParametersDomain` | `TestAContractStrategyScriptDeclaresAnOrderFlowPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | K 線種類的策略腳本不得宣告 | 拒絕，說「訂單流價位寬度只提供給合約行情種類的策略腳本」 | `StrategyScriptParametersDomain.RequireMarketDataKind` | `TestAKCandleStrategyScriptCannotDeclareAnOrderFlowPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 一支策略腳本至多一個 | 拒絕，說「訂單流價位寬度至多宣告一個」 | `NewStrategyScriptParametersDomain` | `TestOrderFlowPriceStepDeclarationRules/至多一個` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 預設值為零 | 拒絕，說必須大於零 | `NewOrderFlowPriceStepDomain` | `TestOrderFlowPriceStepDeclarationRules/預設值為零` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 預設值可以是小數 | 0.5 建立成功 | `NewOrderFlowPriceStepDomain` | `TestOrderFlowPriceStepMayBeAFraction` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 這次執行給的值不大於零 | 這次給 -5 整次拒絕，說必須大於零 | `StrategyScriptParametersDomain.Applying` | `TestThisRunsOrderFlowPriceStepMustBePositive` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 宣告了就每一格帶足跡 | 每一格帶足跡，價位寬度是這一次的值 | `ContractOrderFlowService.AttachFootprints` | `TestTodaysBarsCarryFootprintsAtThisRunsPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 沒宣告就沒有訂單流 | 每一格足跡全空（價位清單空、寬度/買賣差/最密集價位為零），不要任何逐筆成交 | `AttachFootprints`（未宣告即原樣回傳） | `TestAScriptWithoutAnOrderFlowPriceStepAsksForNoTrades`（嚴格 mock：任何逐筆成交查詢都失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 這一次換一個寬度 | 預設 10、這次 25 → 每一格寬度 25 | `Applying` + `AttachFootprints` | `TestTodaysBarsCarryFootprintsAtThisRunsPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 依價位整理一分鐘 | 84150：買 2 賣 1 筆數 2；84160：買 3 賣 0 筆數 1；買賣差 +4；最密集 84150 | `OrderFlowMinuteLedgerDomain` + `OrderFlowReachDomain.Footprinting` | `TestAMinuteIsTalliedByPriceLevel` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 正好落在價位邊界上的成交 | 84160 屬於 84160 價位 | `OrderFlowPriceStepDomain.LevelOf` | `TestOrderFlowPriceLevels` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 收盤那一刻的成交屬於下一格 | 09:01:00 整的成交在 09:01 格，不在 09:00 格 | ledger（分鐘歸屬）+ `Footprinting`（含起點不含收盤） | `TestATradeOnTheCloseBelongsToTheNextBar` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 較粗的一格把同價位加總 | 一小時格 84150 買 4 | `Footprinting` | `TestACoarserBarAddsTheSameLevelAcrossItsMinutes` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 沒有成交的一格 | 清單空、買賣差 0、最密集 0、寬度為這次的值 | `Footprinting` | `TestABarWithNoTradesHasAnEmptyFootprintAtThisRunsStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 一格正好一千個價位 | 照常 | `Footprinting` | `TestABarMayHoldAThousandLevelsButNoMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 一格超過一千個價位 | 整次拒絕，說價位寬度太細、一格至多 1,000 個 | `Footprinting` | `TestABarMayHoldAThousandLevelsButNoMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 全部格子合計正好兩百萬個價位 | 照常 | `Footprinting` | `TestARunMayHoldTwoMillionLevelsButNoMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 全部格子合計超過兩百萬個價位 | 整次拒絕，說一次至多 2,000,000 個 | `Footprinting` | `TestARunMayHoldTwoMillionLevelsButNoMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 過完的日子取每日歷史檔 | 十天各取一次歷史檔、不用即時查詢 | `ContractOrderFlowService.gatherDay` | `TestEndedDaysComeFromTheirArchive`（嚴格 mock，每天恰一次） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 昨天的歷史檔還沒出現 | 10/13 改以即時查詢 10/13 00:00–10/14 00:00，結果照常 | `gatherDay` | `TestYesterdayWithoutItsArchiveYetIsReadLive` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 今天這一段 | 今天 00:00 到最後一格收盤以即時查詢 | `gatherDay` | `TestTodaysBarsCarryFootprintsAtThisRunsPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 來源不答話 | 整次失敗，說讀不到訂單流，無半份結果 | `AttachFootprints`（錯誤分類） | `TestATradeHistoryThatWillNotAnswerFailsTheWholeCalculation`、`TestAContractReplayWhoseOrderFlowCannotBeReadFails` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 剛好 31 個日曆日 | 照常 | `ContractKCandleAlignmentDomain.OrderFlowReach` | `TestOrderFlowReachesThirtyOneCalendarDaysButNoMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 超過 31 個日曆日 | 整次拒絕，說一次至多涵蓋 31 天 | `OrderFlowReach` | 同上 + `TestOrderFlowPastThirtyOneDaysIsRefusedBeforeAskingForAnyTrade` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 剛算過的不必重拿 | 不再向來源要，結果相同 | `ContractOrderFlowMemory` | `TestARememberedDayIsNotFetchedAgainUnlessThePriceStepChanges` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 換寬度就重新整理 | 以 25 再算時每一格依 25 整理 | memory 以寬度為鍵 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 合約回測逐格帶足跡 | 交給逐格執行的每一格都帶足跡 | `ContractBacktestService.RunContractBacktest` | `TestAContractReplayHandsEveryBarItsFootprint` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 交易策略的兩個信號來源各用各的寬度 | 第一來源 10、第二來源 50 | `RunContractTradingStrategyBacktest` | `TestEachTradingStrategySourceReadsFootprintsAtItsOwnPriceStep` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 訂單流已備齊（機器人） | 照常判出結論 | `StrategyBotRunApplication.readSignals` | `TestAContractBotWhoseOrderFlowIsReadyConcludesAsUsual` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 第一次要拿一大段，這一輪來不及 | 跳過、維持執行中；已拿到的留著下一輪接著補 | 備齊時間 + `StrategyBotRoundFailureDomain` + 中斷時保留完整分鐘 | `TestAContractBotWhoseOrderFlowIsNotGatheredInTimeSkipsTheRound`、`TestOrderFlowNotGatheredInTimeIsNotReadyAndKeepsItsWholeMinutes` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 來源不答話（機器人） | 跳過、維持執行中 | `StrategyBotRoundFailureDomain`（Unavailable 落入跳過） | `TestNewStrategyBotRoundFailureDomainReadsWhatAFailedRoundMeans/order flow the venue would not give…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 超過 31 天（機器人） | 停下，停擺原因說出 31 天上限 | `StrategyBotRoundFailureDomain` + 停擺訊息 | `TestAContractBotAskingForOrderFlowPastThirtyOneDaysIsHalted`、`TestStrategyBotLifecycleMessageSaysWhichHaltItWas` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 外掛的說明 | 參數種類說明列出訂單流價位寬度，說只限合約行情 | go-trading-mcp 工具說明 | 在 go-trading-mcp 那一邊的切片 | — | — | ↗ 另一個 repo（見該 repo 的 CONTRACT） |
| AC-34 | 畫面說明（US-07 前言） | 畫面能選這種參數、合約欄位說明有足跡 | go-trading-frontend | 在 go-trading-frontend 那一邊的切片 | — | — | ↗ 另一個 repo |
| AC-35 | 最細 0.00000001 | 0.000000009 被拒絕 | `NewOrderFlowPriceStepDomain` | `TestOrderFlowPriceStepIsNoFinerThanEightDecimals` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 回測送出時就拒絕 | 送出即拒絕、說 31 天；沒宣告的不受限 | `KCandleQueryDomain.requireOrderFlowReach`（兩個合約回測建構子） | `TestAContractReplayReachingPastThirtyOneDaysOfOrderFlowIsRefusedWhenSubmitted` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 剛收盤就讀 | 收盤後 3 秒前不向即時來源要 | `ContractOrderFlowDayDomain.LiveSettledAt` + `gatherDay` | `TestTheLastMinuteIsNotReadBeforeItsTradesHaveSettled` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 來源連線逾時 | 讀不到訂單流，不是還沒備齊 | `AttachFootprints`（依誰的時間用完判斷） | `TestAVenueConnectionTimingOutIsTheVenueFailingNotTheAllowance` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 一格價位太多（機器人） | 停下，原因說超過一次能給的範圍 | `StrategyBotRoundFailureDomain`（任何訂單流拒絕都停下） | `TestNewStrategyBotRoundFailureDomainReadsWhatAFailedRoundMeans/a bar holding too many price levels stops the bot too` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-41 | 送出時連預設暖機一起算 | 25 天加 200 根小時暖機超過 31 天，送出即拒絕 | 兩個 Validate 補上預設暖機 | `TestTheUpFrontOrderFlowCheckCountsTheDefaultWarmUpTheReplayWillAdd` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-40 | 等別的計算拿同一天時時間到 | 跳過（還沒備齊），不卡住 | `ContractOrderFlowMemory.Gathering`（等待可被時間到打斷） | `TestARunWaitingBehindAnotherForTheSameDayStillStopsWhenItsOwnTimeRunsOut` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 速度上限：即時查詢每分鐘至多 40 次 | 預設 40，自己一份節奏 | `ContractOrderFlowConfig`、`venuePacers.cryptoContractTrades` | `TestLoadAppliesTheContractOrderFlowDefaultsWhenNothingIsSet` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 同時下載的歷史檔至多 2 份 | 第二份等第一份 | `BinanceContractTradeHistoryProxy.archiveDownloads` | `TestArchiveDownloadsWaitForAFreeSlot`、config 測試 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 暫留上限，滿了先丟最久沒用的 | 最久沒用的那一天被丟 | `ContractOrderFlowMemory.Keep` | `TestTheLeastRecentlyUsedDayIsForgottenWhenMemoryIsFull`、`TestADayLargerThanTheWholeMemoryIsNeverKept` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 歷史檔不存在、已過完兩天以上即失敗 | 整次失敗，說那一天歷史檔不存在 | `gatherDay` | `TestAnOlderDayWithoutAnArchiveCannotBeRead` | asserts-oracle | produces-oracle | ✅ conforms |
| EDGE-1 | 歷史檔讀到一半斷線：整次失敗，那一天不留在記憶中 | 失敗；下一次整份重拿 | `gatherDay`（錯誤時不 Keep） | `TestADayWhoseArchiveBreaksHalfwayIsNotRemembered` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 沒宣告的策略腳本零額外成本 | 不向來源要任何東西 | `AttachFootprints` | `TestAScriptWithoutAnOrderFlowPriceStepAsksForNoTrades` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `IndicatorCalculationDomain.RequireSpotParameters`、`BacktestDomain.RequireSpotParameters`、`TradingStrategyReplaySourcesDomain` | 現貨計算／重演自帶算式宣告訂單流時拒絕 | AC-2 規則延伸到「沒存下來的算式」；同一條業務規則，不是新行為 |
| `indicatorCalculationController.respondWithError` | 拒絕的訂單流回 400、讀不到回 502 | 屬於 AC-24／AC-22 的對外呈現 |
| `BinanceContractTradeHistoryProxy` 讀微秒時間戳 | 交易所歷史檔可能改用微秒 | 正規化細節，不改變業務結果 |

## Summary

- Conforms: 45/45 in-repo clauses ✅ (100%)
- Second review round: AC-39 reversed (too many levels halts again, so a too-fine step is never silently skipped forever); AC-41 added.
- Violations: —
- Mis-asserted: —
- Partial: — (AC-29, EDGE-1 were partial at first pass; tests added and falsified)
- Gaps: —
- Unclear: —
- Out of this repo: AC-33（外掛）、AC-34（畫面）
- Orphans: 3（皆為既有條款的延伸，無範圍外行為）
