# Contract Traceability Matrix — 合約機器人自動下單

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/`（domain／application／infrastructure）、`cmd/server/`
Oracle: Acceptance Criteria（39 clauses）＋ Core Business Rules（10）＋ NFR（3）

> 引用一律寫到 method，不寫行號——行號改個註解就會爛。
> 縮寫：`Exec` = `internal/application/tests/contract_auto_order_execution_application_test.go`；
> `Edges` = `…/contract_auto_order_execution_edges_application_test.go`；`Queue` = `…/contract_auto_order_queue_application_test.go`；
> `Read` = `…/contract_auto_order_reading_application_test.go`；`Repo` = `internal/infrastructure/persistence/tests/contract_auto_order_repository_test.go`；
> `Proxy` = `internal/infrastructure/exchange/tests/binance_contract_order_proxy_test.go`。

## Clauses — 第一輪（審查當下）

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 結論改變時開倉（例 1） | 收到輪次訊息；幣安開一筆多倉；另一則下單結果寫著「已做多」與成交數量、均價 | `StrategyBotService.RecordRound`、`ContractAutoOrderService.open`、`ContractAutoOrderMessageDomain.Text` | Queue `TestARoundThatChangesItsConclusionQueuesAnAutoOrderFromItsOwnPlan`、Exec `TestAnAutoOrderOpensALongAndGuardsItFromTheFill` | asserts-oracle（斷言「做多」與「0.002 @ 85000」） | diverges（文字是「做多」不是「已做多」；但 PRD §4 的通知格式寫的是 `<動作：做多…>`——PRD 內部自相矛盾） | 🔴 violation（規格矛盾） |
| AC-02 | 結論沒變不下單（例 2） | 不送輪次訊息、不下任何單 | 既有 `DecideRound`（沒有訊息就沒有意圖） | Queue `TestARoundThatRepeatsItsConclusionQueuesNoAutoOrder` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 自動下單關著只送訊息（例 3） | 收到輪次訊息、不下單 | `RecordRound`（鎖住列的 `AutoOrderEnabled`） | Queue `TestABotWithAutoOrderOffOnlySpeaks` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | 已停止的機器人手動跑一輪不下單（例 4） | 收到輪次訊息、不下單 | `RecordRound`（鎖住列的 `RunState`） | Queue `TestAStoppedBotRunByHandNeverOrders` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 執行中的機器人手動跑一輪照常下單（例 5） | 開一筆多倉，與排定輪次相同 | 同一條 `RecordRound` | Queue `TestARunningBotRunByHandOrdersLikeAScheduledRound` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 現貨機器人一律不下單（例 6） | 收到輪次訊息、不下單 | `ContractAutoOrderIntentDomain.ToDto`（現貨沒有意圖） | `TestStrategyBotRunApplicationSpotBotStillOnlySpeaksWithAutoOrderSwitchedOn` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 照建議數量與機器人槓桿開倉（例 7） | 先設逐倉 3 倍，開多 0.002 | `ContractAutoOrderIntentDomain`、`open`、`BinanceContractOrderProxy.PrepareIsolatedLeverage` | Queue（0.002、3）、Exec（`PrepareIsolatedLeverage(…, 3)`、BUY 0.002）、Proxy（ISOLATED） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 沒有部位規劃不開倉（例 8） | 不下單；結果寫著「沒有部位規劃，不下單」 | `openQuantity`、`NextStep`、`Settled` | Queue、Exec `TestAnAutoOrderWithNothingToOpenSendsNothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 交易所不收這一筆不開倉（例 9） | 不下單；寫著「交易所不收這一筆」與原因 | `openQuantity` | Queue（前綴）、Queue `TestARoundThatCannotOpenSaysWhyInTheMessagesWords`（全文） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 幣安說餘額不足（例 10） | 結果「餘額不足」；自動下單仍開著 | `SettledByFailure` | Exec `TestAnAutoOrderRefusedForLackOfBalanceKeepsAutoOrderOn` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 只平機器人持倉（例 11） | 平多 0.002；使用者的 0.01 不動；持倉空手 | `CloseOrderFor` | Exec `TestAnAutoOrderClosesOnlyWhatTheBotOpened` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 以幣安當下倉位為上限（例 12） | 平多 0.006；持倉空手 | `CloseOrderFor` | 同上（第二案） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 沒有機器人持倉不需平倉（例 13） | 不下單；「沒有自動開出的倉位，不需要平倉」，不算失敗 | `NextStep`、`Settled` | Exec `TestAnAutoOrderWithNothingOfItsOwnToCloseSendsNothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 已有同方向持倉不加倉（例 14） | 不下單；「已經持有多倉，不加倉」 | `Settled` | Exec `TestAnAutoOrderNeverAddsToWhatTheBotAlreadyHolds` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 開多並掛止損止盈（例 15） | 止損 83725、止盈 87550；結果寫出兩價位 | `AfterOpen`、`ProtectiveOrders`、`protect` | Exec `TestAnAutoOrderOpensALongAndGuardsItFromTheFill` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 平多時先撤止損止盈（例 16） | **先**撤掉止損止盈，**再**平多 0.002；持倉空手 | `ContractAutoOrderService.close` | Exec `TestAnAutoOrderClosesOnlyWhatTheBotOpened` | shallow（沒有斷言「先撤再平」的順序） | produces-oracle | 🟠 mis-asserted |
| AC-17 | 只做空時賣出開空（例 17） | 開空成交 | `OpenOrder` | Exec `TestAnAutoOrderOpensAShortWithItsExitsSwapped` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 只做空時買入平空（例 18） | 撤掉止損止盈，平空 0.002 | `close` | Exec `TestAnAutoOrderClosesAShortByBuyingBack` | shallow（順序） | produces-oracle | 🟠 mis-asserted |
| AC-19 | 多空反手（例 19） | 撤單、平多 0.002，**再**開空；持倉空 | `close` → `open` | Exec `TestAnAutoOrderReversesByClosingThenOpening` | shallow（沒有斷言平倉在開倉之前） | produces-oracle | 🟠 mis-asserted |
| AC-20 | 反手時平倉成功、開倉失敗（例 20） | 「已平倉，但做空沒開成：餘額不足」；持倉空手 | `withOpenFailure` | Exec `TestAReverseWhoseOpenIsRefusedSaysItClosedButDidNotOpen` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 止損已在幣安成交（例 21） | 不送平倉單；撤剩下的單；寫著「幣安上已沒有這筆倉位（可能已觸發止損或止盈）」；持倉空手 | `CloseOrderFor`、`AfterCloseVanished`、`Settled` | Exec `TestAnAutoOrderFindingThePositionGoneSendsNoClose` | shallow（只斷言含「幣安上已沒有這筆倉位」，不是全文） | produces-oracle | 🟠 mis-asserted |
| AC-22 | 止損掛不上（例 22） | 倉位保留；開頭「⚠️ 止損沒有掛上，請立刻到幣安自己處理」 | `AfterProtection`、`Text` | Exec `TestAStopTheVenueRefusesLeavesThePositionAndSaysSoLoudly` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 沒設停損停利距離就不掛（例 23） | 開多成交；沒有保護單 | `ProtectiveOrders` | Exec `TestAnAutoOrderWithoutExitDistancesPlacesNoProtection`（嚴格 mock：掛單即失敗） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 改不成逐倉不開倉（例 24） | 不開倉；寫著原因；自動下單仍開著 | `SettledByFailure` | Exec `TestAnAutoOrderTheAccountSettingsRefuseKeepsAutoOrderOn` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 雙向持倉模式不下單（例 25） | 不下單；「請先在幣安把持倉模式改成單向持倉」 | `refuseToSend`、`SettledInHedgeMode` | Exec `TestAnAutoOrderOnAHedgeModeAccountSendsNothing` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 好幾台系統同時跑只送一次（例 26） | 幣安上只出現一張 | `ContractAutoOrderRepository.Claim` | Repo `TestContractAutoOrderRepositoryLetsExactlyOneReplicaTakeAnOrder`、Exec `TestAnAutoOrderAnotherReplicaTookIsLeftAlone` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 不確定時先查，已成交就不再送（例 27） | 先以編號查；不再送；照查到的記錄 | `open`／`close` 的查單 | Exec `TestAnAutoOrderTheVenueAlreadyFilledIsNotSentAgain`、Edges `TestACloseTheVenueAlreadyFilledIsNotSentAgain` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 不確定時先查，沒收到才再送（例 28） | 再送一次，編號不變 | 同上 | Exec `TestAnAutoOrderTheVenueNeverReceivedIsSentAgainUnderTheSameID` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 短暫連不上照常下單（例 29） | 恢復後照常下單 | `fail` → `Rescheduled`；下一次照常 | Exec `TestAnAutoOrderThatCannotReachTheVenueWaitsWithinItsDeadline` ＋ 上一列 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 超過 2 分鐘放棄（例 30） | 放棄；「訊號已經過時，沒有下單」；不補下 | `RefusalToSend` | Exec `TestAnAutoOrderNotSentByItsDeadlineIsGivenUp` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 金鑰不被接受（例 31） | 兩台的自動下單都關；寫著「幣安不接受你的交易金鑰，已關掉你所有機器人的自動下單，請重存金鑰」 | `SettledByFailure` | Exec `TestAKeyTheVenueRejectsSwitchesOffEveryAutoOrder` | shallow（只斷言含「重存」） | diverges（文字是「…請重存幣安交易金鑰」） | 🔴 violation |
| AC-32 | 金鑰沒有合約權限（例 32） | 只關合約機器人；現貨仍開著 | `fail`（再驗證）、`SettledByFailure` | Exec `TestAKeyWithoutContractTradingSwitchesOffOnlyContractBots`、Repo `TestDisablingAutoOrderByOwnerStaysWithinTheKindsGiven` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 關掉後重試中的單不送（例 33） | 不送；「自動下單已關閉，沒有下單」 | `RefusalToSend` | Exec `TestAnAutoOrderSendsNothingOnceTheBrakeIsPulled` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 關掉不平倉、不撤單（例 34） | 幣安上的倉位與保護單都在；機器人持倉仍記著多 0.002 | `StrategyBotRepository.DisableAutoOrder`（只寫開關欄位） | — | no-test（沒有測試證明關掉開關不動機器人持倉） | produces-oracle | 🟡 partial |
| AC-35 | 停止或刪除後不送（例 35） | 不送 | `RefusalToSend`；刪除 cascade | Exec（已停止）、Repo `TestDeletingABotDropsTheOrdersItHasNotCarriedOut` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 執行紀錄上看得到成交（例 36） | 那一輪看得到數量、均價、止損止盈 | `ListRunRecords`、`ToResultDto` | Read `TestTheHistoryShowsEachRoundsAutoOrderBesideIt` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 執行紀錄上看得到沒下單的原因（例 37） | 那一輪看得到「餘額不足」 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 看得到機器人持倉（例 38） | 看得到「多 0.002」 | `StrategyBot.ToDto` | Read `TestAContractBotShowsWhatItOpenedItself` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-39 | 外掛不能下單（例 39） | 外掛沒有這件事可以做 | 沒有新增任何寫入路由（`registerRoutes` 未變） | — | no-test（以「不存在」成立；既有自動下單路由的 `requiresWebSignIn` 測試仍在） | produces-oracle | 🟡 partial（結構性） |
| BR-01 | 觸發條件 | 合約、執行中、開著、說出新結論才排入 | `RecordRound`、`PlanAutoOrderIntent` | AC-02～06 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 排入與輪次同生共死 | 那一輪沒記成就沒有委託 | `RecordRound`（同一交易） | `internal/domain/service/tests/strategy_bot_service_auto_order_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-03 | 要做的事 = 機器人持倉 → 目標 | 表格五列 | `NextStep`、`Settled` | AC-11～19 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-04 | 開倉／平倉只減倉 | 平倉只減倉；開倉設逐倉槓桿 | `CloseOrderFor`、proxy | Proxy `TestBinanceContractOrderProxyMarksACloseAsReduceOnly` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 止損止盈以標記價格觸發、只減倉 | `MARK_PRICE`、只減倉 | `PlaceProtectiveOrder` | Proxy `TestBinanceContractOrderProxyPlacesAMarkPriceStopThatOnlyReduces` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 撤單撤不掉不算失敗 | 已不存在視為已撤 | `CancelProtectiveOrder` | Proxy `TestBinanceContractOrderProxyTakesAProtectiveOrderAlreadyGoneAsCancelled` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 機器人持倉加減 | 開倉記成交量；平倉歸零 | `AfterOpen`、`AfterClose` | AC-11、15 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 時限 2 分鐘；約每 10 秒看一次 | 2 分鐘放棄；約每 10 秒 | `ContractAutoOrderDeadline`；`AUTO_ORDER_DISPATCH_INTERVAL_SECONDS`（5）、重試等待 5 秒 | AC-30；Exec 重試時刻 `T+15s` | asserts-oracle | diverges（ARCH 定為 5 秒，PRD 寫約 10 秒——規格與設計不一致） | 🔴 violation（規格矛盾） |
| BR-09 | 送單前再檢查 | 機器人在、執行中、開著、金鑰可交易合約 | `RefusalToSend` | AC-33、35、Edges `TestAnAutoOrderWithoutAContractTradingKeySendsNothing` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 失敗分類與後果 | 表格六列 | `SettledByFailure`、proxy 分類 | AC-10、24、31、32；Proxy `TestBinanceContractOrderProxySortsTheVenuesRefusals` | asserts-oracle | produces-oracle（AC-31 文字見上） | ✅ conforms |
| NFR-01 | 15 秒內送出 | 正常 15 秒內 | 執行間隔 5 秒 | — | no-test（設定值） | produces-oracle | 🟡 partial |
| NFR-02 | 金鑰不出現在訊息與紀錄 | 不出現 | proxy 只放進簽名與標頭 | Proxy `…PlacesASignedMarketOrderAndReadsTheFill`（`NotContains secret`） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | 一筆只由一台執行 | 同 AC-26 | `Claim` | AC-26 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `ContractAutoOrderDomain.SettledUnsealable` | 金鑰存在但系統打不開時結束委託（「系統目前無法開啟你的幣安交易金鑰」） | undocumented——補進 PRD §4 Edge Cases |
| `ContractAutoOrderDomain.openRefusal`（非整數槓桿） | 「交易所不收這一筆：幣安只接受整數槓桿」 | undocumented（ARCH 決策）——補進 PRD §4 |
| `ContractAutoOrderIntentDomain.ToDto`（交易模式讀不懂） | 不排入委託 | undocumented——補進 PRD §4 |
| `MayKeepTryingProtectionAt`（5 分鐘保護重試窗） | 保護單不確定時重試 5 分鐘 | undocumented（ARCH 決策）——補進 PRD §4 |

沒有任何程式碼落在 Out of Scope（現貨下單、記日誌、移動止損、外掛說明）。

## Summary（第一輪）

- Conforms: 43/52 clauses ✅（82.7%）
- Violations: AC-01、BR-08（規格自相矛盾）、AC-31（措辭）
- Mis-asserted: AC-16、AC-18、AC-19（沒斷言順序）、AC-21（只斷言片段）
- Partial: AC-34、AC-39、NFR-01
- Gaps: —
- Unclear: —
- Orphans: 4（皆為合理的防護，需補進 PRD）

---

## 修正紀錄（依 feedback 修正後重驗）

| ID | 修正 | 重驗 |
|----|------|------|
| AC-01 | PRD 例 1 的 Then 改成與 §4 通知格式一致：寫著「做多」與「✅ 成交 0.002 @ 85000」（§4 是唯一定案的格式，例 1 的「已做多」是寫 PRD 時的鬆散措辭） | ✅ conforms |
| AC-31 | 程式改成 PRD 原句「…請重存金鑰」；測試改為斷言全文 | ✅ conforms |
| AC-16／18／19 | 測試以 `gomock.InOrder` 斷言「先撤保護單再平倉」「先平倉再開倉」 | ✅ conforms |
| AC-21 | 測試改為斷言全文「幣安上已沒有這筆倉位（可能已觸發止損或止盈）」 | ✅ conforms |
| AC-34 | 新增 Repo 測試：關掉自動下單後機器人持倉原封不動 | ✅ conforms |
| BR-08 | PRD §4 改為「約每 5 秒」（ARCH 的決定；比 10 秒更貼近「不要晚」的目的，也仍在 NFR 15 秒之內） | ✅ conforms |
| Orphans | PRD §4 Edge Cases 補上四條 | 已有條款 |

- AC-39、NFR-01 維持 🟡：前者以「沒有這條路由」成立，後者是部署設定，皆無法以單元測試有意義地斷言。
- **修正後：50/52 ✅（96.2%）、🟡 2、其餘 0。**

Note: static conformance audit against the Acceptance Criteria — it judges test assertions and code paths against the spec's expected outcome, not by running the full suite.


## Code review 後的更新（2026-10-04）

Code review（PR #102）指出「先撤止損止盈再平倉」會在平倉沒成時讓倉位失去保護。已改為「平倉成交後才撤」，並同步更新 BRIEF／PRD／ARCH／ORACLE。

| 條款 | 變更 | 測試 | 狀態 |
|---|---|---|---|
| AC-16 平多成交後才撤止損止盈 | 順序改為平倉 → 撤單 | `TestAnAutoOrderClosesOnlyWhatTheBotOpened`（`gomock.InOrder(closing, cancels…)`） | ✅ conforms |
| AC-16a 平倉沒成，止損止盈留著（新） | 平倉被拒／讀倉位被拒／過時都不撤 | `TestACloseThatDoesNotGoThroughLeavesTheGuardsStanding` | ✅ conforms |
| AC-16b 平倉成交但止損撤不掉（新） | 照樣記下平倉 | `TestAGuardTheVenueRefusesToTakeDownStillLetsTheCloseBeRecorded` | ✅ conforms |
| AC-18 平空成交後撤單 | 同上 | `TestAnAutoOrderClosesAShortByBuyingBack` | ✅ conforms |
| AC-20a 反手平倉後開倉來不及（新） | partiallyDone，不再記成放棄 | `TestAReverseWhoseOpenIsHeldBackAfterItsCloseSaysItClosed`、`TestAReverseThatFoundNothingToCloseIsGivenUpWhenItsOpenIsTooLate` | ✅ conforms |
| BR 止損掛單不確定仍記下 | 持倉保留那張單的 id | `TestAProtectiveOrderStillStuckAfterAWhileIsHandedToTheOwner`、`TestAProtectiveOrderTheVenueRefusedIsNotTakenDownLater` | ✅ conforms |
| BR 金鑰驗證正常不全面關閉 | 改為一般拒絕 | `TestARejectionAGoodKeyContradictsSwitchesNothingOff` | ✅ conforms |

另：`AUTO_ORDER_EXECUTION_TIMEOUT_SECONDS` 預設由 150 改為 70、`AUTO_ORDER_REQUEST_TIMEOUT_SECONDS` 由 10 改為 5，讓分身掛掉時那一筆在 2 分鐘下單時限內還來得及被別台接手。
