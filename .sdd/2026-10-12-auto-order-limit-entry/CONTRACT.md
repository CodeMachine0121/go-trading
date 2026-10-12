# Contract Traceability Matrix — 自動下單限價進場（滑價容忍度）

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` (branch `feature/auto-order-limit-entry`)
Oracle: Acceptance Criteria (23 scenarios) + Business Rules (8) + NFR (2) = 33 clauses

Path abbreviations: `ctl/` = `internal/controller/`, `app/` = `internal/application/tests/`, `svc/` = `internal/domain/service/`,
`dom/` = `internal/domain/models/domains/`, `per/` = `internal/infrastructure/persistence/`, `ex/` = `internal/infrastructure/exchange/`.
`LE` = `app/contract_auto_order_limit_entry_application_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01-1 | 沒給容忍度就是 0.1% | 建立成功，容忍度 0.1 | dom/strategy_bot_domain.go (default) | ctl TestStrategyBotRouterCarriesTheEntrySlippageTolerance「建立時沒給就是 0.1」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-2 | 容忍度可以是 0 | 建立成功，容忍度 0 | 同上；per Save 寫入 0 | ctl 同上「給 0」；per TestStrategyBotRepositoryKeepsTheEntrySlippageTolerance | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-3 | 容忍度可以是上限 5 | 建立成功，容忍度 5 | dom/strategy_bot_domain.go (GreaterThan) | ctl 同上「給 5」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-4 | 超過上限被拒絕 | 拒絕，「進場滑價容忍度必須介於 0 到 5 之間（單位：%）」，不存 | dom/strategy_bot_domain.go | ctl TestStrategyBotRouterRefusesAnEntrySlippageToleranceOutOfRange 5.01（400、訊息、無 Save） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-5 | 負數被拒絕 | 同上 | 同上 | 同上 -0.1 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-6 | 改寫時沒給就保留原值 | 仍是 0.3 | svc/strategy_bot_service.go UpdateStrategyBot；per Save 白名單 | ctl 同上「改寫時沒給就保留」；per rewrite 0.3 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-7 | 既有的機器人一律是 0.1% | 讀到 0.1 | entities/strategy_bot.go `default:0.1`（AutoMigrate 加欄時回填） | per TestStrategyBotRepositoryKeepsTheEntrySlippageTolerance（沒寫這欄的列讀回 0.1） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-1 | 做多的限價往下取整 | 買進限價 84238.1、掛到撤為止 | dom EntryOrder (Floor)；ex PlaceLimitOrder (LIMIT/GTC) | LE TestAnEntryOrderIsPricedFromTheReferenceWithinTheTolerance「做多」；ex TestBinanceContractOrderProxyRestsALimitOrderUntilCancelled | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-2 | 做空的限價往上取整 | 賣出限價 84069.9 | dom EntryOrder (Ceil) | LE 同上「做空」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-3 | 容忍度 0 就掛在參考價 | 限價 84154 | dom EntryOrder | LE 同上「容忍度 0」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-4 | 沒有價格跳動單位不開倉 | 不下單；原因「這個合約標的還沒有價格跳動單位，無法掛限價進場單，不下單」 | svc open → dom SettledWithoutTickSize | LE TestAnEntryOrderOnAContractWithoutATickSizeSendsNothing（嚴格 mock：無 PlaceLimitOrder） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-5 | 沒有參考價不開倉 | 不下單；原因「這一輪沒有參考價，無法掛限價進場單，不下單」 | dom openRefusal | LE TestAnEntryOrderThatCannotBePricedSendsNothing | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-1 | 立刻全部成交 | 持倉多 0.002；止損由 84180 起算；通知含參考價、限價、0.002 @ 84180、滑價 +0.031% | svc recordEntryPlaced → actOnEntry → resolveEntry；dom ResolveEntry/AfterOpen；message domain | LE TestAnEntryOrderFilledAtOnceIsGuardedAndReportedWithItsSlippage（止損 82917.3、📌 行） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-2 | 等了一陣子才全部成交 | 那時才掛止損止盈；持倉 0.002 | svc followEntry | LE TestAWorkingEntryOrderWithinItsWindowWaitsKeepingWhatItFilled + TestAWorkingEntryOrderFillingLaterIsGuardedThen | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-3 | 時候到了只成交一部分 | 撤剩下；持倉 0.001；保護 0.001；「部分成交 0.001／0.002，其餘已撤單」 | dom ShouldWithdrawEntryAt；svc actOnEntry；message domain | LE TestAnEntryOrderThatMustComeDownIsCancelledAndResolvedByWhatItFilled「進場時限到了」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-4 | 時候到了一點都沒成交 | 撤單；空手；結果「沒有成交」「進場單沒有成交，已撤單」；通知帶參考價與限價 | dom ResolveEntry (unfilled) | LE TestAnEntryOrderThatFilledNothingByItsDeadlineIsCancelledAndSaidSo | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-5 | 等待中策略說要出場，已部分成交 | 立刻撤；持倉 0.001；下一筆以市價平 0.001 | per ExistsUnsettledAfter；dom ShouldWithdrawEntryAt；既有平倉步驟 | LE 同 AC-03-3「有新的結論排在後面」；市價平機器人持倉由既有 TestAnAutoOrderClosesOnlyWhatTheBotOpened；per TestContractAutoOrderRepositoryTellsWhetherANewerOrderWaits | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-6 | 等待中策略說要出場，一點都沒成交 | 撤單、「沒有成交」；下一筆「沒有自動開出的倉位，不需要平倉」 | 同上 + dom ResolveEntry | 撤單＋0 成交由 TestAnEntryOrderThatFilledNothingByItsDeadline…（觸發條件不同、同一收尾路徑）；「不需要平倉」由既有 TestAnAutoOrderWithNothingOfItsOwnToCloseSendsNothing | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-7 | 等待中擁有者關掉自動下單 | 立刻撤；持倉 0.001 並掛保護 | dom ShouldWithdrawEntryAt (!autoOrderEnabled / !botRunning) | LE 同 AC-03-3「自動下單已關」「機器人已停止」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-8 | 要撤的時候幣安說單已不在 | 以最後成交收尾：全部成交、掛保護 | svc actOnEntry (NotFound → FindOrder) | LE TestAnEntryOrderGoneWhenCancelledIsLookedUpAgain | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-9 | 反手時進場單沒成交 | 「已平倉，但做空沒開成：進場單沒有成交，已撤單」；空手 | dom ResolveEntry → withOpenFailure | LE TestAReverseWhoseEntryFilledNothingSaysItClosedButDidNotOpen | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-1 | 進場單還掛著時不能刪 | 拒絕並說明；機器人仍在 | svc DeleteStrategyBot；per ExistsOutstandingEntry；ctl 409 `autoOrderEntryOutstanding` | app TestStrategyBotApplicationDeleteKeepsABotWhoseEntryOrderStillRests；ctl TestStrategyBotRouterKeepsABotWhoseEntryOrderStillRests；per TestContractAutoOrderRepositoryTellsWhetherAnEntryOrderStillRests | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-2 | 進場單已收尾就照常刪 | 刪除 | 同上 | app TestStrategyBotApplicationDeleteDoesNotAskForAStopFirst；ctl TestStrategyBotRouterRewritesAndDeletes；per（已收尾／已結束／未送出 → false） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 容忍度取排入那一刻的機器人設定 | 委託帶 0.2、參考價 85000、時限 ranAt+觸發間隔 | dom NewQueuedContractAutoOrderDomain(bot…) | app TestARoundThatChangesItsConclusionQueuesAnAutoOrderFromItsOwnPlan | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 進場單帶系統編號、先查再送 | 已在幣安就不再送 | svc open (FindOrder first) | LE TestAnEntryOrderAlreadyAtTheVenueIsFollowedNotSentAgain；既有 TestAnAutoOrderTheVenueAlreadyFilledIsNotSentAgain | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 約每 5 秒看一次進場單 | 還在等 → now+5s 再看 | dom AwaitingEntry | LE TestAnEntryOrderStillWorkingIsLookedAtAgainShortly / …WithinItsWindowWaits… | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 撤單結果不確定稍後再試 | 回到等待、不收尾 | svc actOnEntry → fail (uncertain) / Working after cancel | LE TestAnEntryOrderWhoseCancelIsUnclearIsTriedAgain | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 幣安自己結束而沒成交 | 「沒有成交」 | svc actOnEntry (non-working → resolve) | LE TestAnEntryOrderTheVenueEndedByItselfIsResolved | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 滑價對擁有者不利為正、小數三位 | 做多 +0.031／-0.012；做空 0.064 | dom EntrySlippagePercentage | LE TestAnEntryOrderFilledAtOnceIsGuardedAndReportedWithItsSlippage、TestAnEntryFilledBetterThanTheReferenceShowsANegativeSlippage；app TestTheHistoryShowsHowEachEntryOrderWent | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 下單結果多帶進場欄位與狀態 | 參考價、限價、容忍度（有參考價才帶）、要求數量、進場狀態、已成交量、滑價 | entities ToResultDto；dom ToResultDto | app TestTheHistoryShowsHowEachEntryOrderWent | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 進場單可能還掛著時結束要請擁有者自己撤（金鑰失效等） | 原因補「進場單可能還掛在幣安上，請自己到幣安撤單」 | dom withOpenFailure；svc followEntry (SettledWithoutKey/Unsealable) | LE TestAWorkingEntryOrderWhoseKeyIsGoneIsHandedToTheOwner（三種） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 全部成交後 10 秒內掛上保護 | 下一次看單（≤5 秒）即收尾並掛保護 | dom AwaitingEntry 5s；job 5s 掃描 | 間接：BR-3 + AC-03-2 | shallow | produces-oracle | 🟠 mis-asserted（時間上限僅由設定推得，無端到端計時測試） |
| NFR-2 | 進場單不在幣安上超過一個觸發間隔加 10 秒 | 時限到的那一次看單就撤 | dom ShouldWithdrawEntryAt；deadline = ranAt + 間隔 | AC-03-3/4 + BR-1 | shallow | produces-oracle | 🟠 mis-asserted（同上） |

## Orphans

| Behavior | Site | Classification |
| :--- | :--- | :--- |
| 進場單的交易所編號（`entryExchangeOrderId`）存在委託上但不出現在結果 | entities/contract_auto_order.go | 未記載於 PRD 的內部追查欄位；不對外，無衝突 |
| 讀「後面有沒有新委託」失敗時本輪不動、等 claim 過期 | svc actOnEntry | 可靠性行為，與既有「儲存失敗留給 claim 過期」一致；PRD 未記載 |

Out-of-scope checklist：現貨、平倉限價、追價、等待中先掛止損——均未實作 ✔

## Summary

✅ 31 conforms · 🔴 0 violations · 🟠 2 mis-asserted（NFR 時間上限，無計時測試） · 🟡 0 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 2 orphans
Conformance: 94%（31/33）

The two NFR rows are timing ceilings that follow from configuration (5-second dispatch and check wait) and are not worth a wall-clock test; accepted as is.
Static conformance audit against the Acceptance Criteria; it judges assertions and code paths against the spec's expected outcomes and does not execute invented scenarios.
