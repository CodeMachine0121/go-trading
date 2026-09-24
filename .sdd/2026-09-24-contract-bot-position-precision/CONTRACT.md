# Contract Traceability Matrix — 合約機器人建議部位的精確度

Contract: PRD.md
Design map: ARCH.md
Implementation: `git diff origin/main...HEAD` (branch `feat/contract-bot-position-precision`)
Oracle: Acceptance Criteria + Business Rules + Edge Cases + NFR (34 clauses)

Defaults for every AC (PRD §3 preamble): contract bot, capital 1,000, whole stake, 5×, reference 100, long/short;
quantity step 0.001, min qty 0.001, min notional 5, tick 0.01, MMR 0.5% (c = 0), 8h funding; latest rate +0.01%.

Paths abbreviated: `ppd` = internal/domain/models/domains/position_plan_domain.go ·
`msg` = internal/domain/models/domains/strategy_bot_message_domain.go ·
`svc` = internal/domain/service/strategy_bot_service.go ·
`repo` = internal/infrastructure/persistence/strategy_bot_run_record_repository.go ·
`rules` = internal/domain/models/domains/contract_trading_rules_domain.go ·
`Tdom` = internal/domain/models/domains/tests/contract_bot_position_precision_domain_test.go ·
`Tapp` = internal/application/tests/contract_strategy_bot_run_application_test.go ·
`Tapp2` = internal/application/tests/contract_bot_position_precision_application_test.go ·
`Tent` = internal/domain/models/entities/tests/strategy_bot_run_record_test.go ·
`Tper` = internal/infrastructure/persistence/tests/contract_bot_run_record_repository_test.go (skips without TEST_POSTGRES_DSN; judged by reading).

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 取整後的數量與保證金 — When 這一輪做多 Then 建議數量 50、名目 5,000、保證金 1,000 | qty 50, notional 5000, margin 1000 | ppd:272-285 | Tdom `StepsAContractSuggestionToTheVenue/five times at a hundred`; Tapp:180 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 數量照步進往下取整，保證金照取整後重算 — Given 槓桿一倍、參考價 810 … Then 建議數量 1.234、名目 999.54、保證金 999.54 | qty 1.234 (1000/810 floored), notional 999.54, margin 999.54 | ppd:266,272 (OpenFor → QuantityFor, OpeningMargin) | Tdom `…/the quantity stepped down…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 止損價對齊價格跳動單位 — Given 價格跳動單位 0.1、停損距離 2.03% … Then 止損價 98 | 97.97 aligned to tick 0.1 → stop 98 | rules:207-213 (Round to nearest tick) via OpenFor exit prices, ppd:273 | Tdom `…/the stop on the nearest tick` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 名目低於最小名目 — Given 固定金額 3、槓桿一倍 … Then 交易所不收，名目 3 低於最小名目 5；And 不給止損價、止盈價與預估強平價 | refused, reason "notional 3 < min notional 5"; no stop, no take-profit, no liquidation price | ppd:250-262; rules:146; msg:166-187 | Tdom `SaysWhyTheVenueWouldRefuse…/less notional…` (Tdom:152-162) | shallow — asserts no stop and no liquidation, never asserts no take-profit (fixture has TP 4%) | produces-oracle (early return msg:166; DTO has no TP) | 🟠 mis-asserted |
| AC-5 | 數量低於最小下單量 — Given 最小下單量 100 … Then 交易所不收，數量 50 低於最小下單量 100 | refused, reason "qty 50 < min qty 100" | rules:157-162; msg:176-178 | Tdom `…/fewer units…` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 名目那一級不允許這麼高的槓桿 — Given 槓桿 20 倍，名目 20,000 落在最高 10 倍那一級 … Then 交易所不收，這個名目那一級最高只能開 10 倍 | refused, tier for notional 20000 allows max 10× | rules:179-186; msg:185-186 | Tdom `…/more leverage than the notional's tier allows` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 做多的預估強平價 — Then 預估強平價 80.4，在參考價下方 | (5000−1000)/(50×0.995)=80.402 → 80.4, shown below | ppd:287-291; msg:209-216 | Tdom `EstimatesWhere…/a long one below`; Tapp:204 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 做空的預估強平價 — Then 預估強平價 119.4，在參考價上方 | (5000+1000)/(50×1.005)=119.403 → 119.4, shown above | ppd:287-291; msg:209-216 | Tdom `…/a short one above` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 沒有分級時用最小那一級估算 — Then 預估強平價以交易規格最小那一級算出，並說明「用最小那一級估算」 | estimate from spec's smallest tier and says 「用最小那一級估算」 | ppd:286 (`!HasLadder`); msg:210-213 | Tdom `EstimatesWhere…` (flag + text), `EstimatesFromTheNotionalsOwnTier` (negative) | asserts-oracle | produces-oracle | ✅ conforms (see Orphans O-4 on §5 formatting) |
| AC-10 | 止損比預估強平價還遠 — Given 10 倍、停損 15% … Then 警告止損比預估強平價還遠，還沒到止損就會先被強制平倉 | stop 85 ≤ liq 90.45 → warning naming the liquidation estimate | ppd:298-302; msg:250-251 | Tdom `WarnsOfAStopBeyond…/fifteen percent` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 止損在預估強平價之內 — Given 10 倍、停損 5% … Then 沒有強制平倉的警告 | stop 95 > liq 90.45 → no liquidation warning | ppd:298-302; msg:250-255 | Tdom `…/five percent` (NotContains 強制平倉) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 費率為正時做多付 — Then 寫出每 8 小時約付 0.5，並標明是估算 | "every 8h pay ≈0.5", marked estimate | ppd:317-336; msg:225-234 | Tdom `EstimatesWhatFundingCosts/a positive rate is paid by a long`; Tapp:205 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 費率為正時做空收 — Then 寫出每 8 小時約收 0.5 | "every 8h receive ≈0.5" | ppd:328-330; msg:229-230 | Tdom `…/received by a short` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 費率為負時做多收 — Then 寫出每 8 小時約收 0.5 | "every 8h receive ≈0.5" | ppd:327; msg:229-230 | Tdom `…/a negative rate is received by a long` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 還沒有結算紀錄 — Then 寫出「還沒有資金費率紀錄」，不給估算金額 | says 「還沒有資金費率紀錄」 and no amount | ppd:323-325; msg:223-224 | Tdom `…/no settlement yet` (Tdom:283) | shallow — only Contains the phrase; never asserts absence of 約付/約收 amount | produces-oracle (if/else exclusive) | 🟠 mis-asserted |
| AC-16 | 沒有交易規格仍給建議 — Then 建議保證金 1,000、名目 5,000 與止損止盈價；不給數量與預估強平價；說明還沒有交易規格… | margin 1000, notional 5000, stop AND take-profit; no qty, no liq; explanation | ppd:234-240; msg:242-243 | Tdom `SuggestsOnAContractWithNoSpecificationYet/figures…` (Tdom:296-310); Tapp2 | shallow — asserts stop present but never take-profit (settings TP 4%) | produces-oracle | 🟠 mis-asserted |
| AC-17 | 沒有交易規格時退回粗略的強平判斷 — Given 10 倍、停損 10% … Then 警告還沒到止損就會先被強制平倉 | 10%×10 ≥ 100% → rough warning | ppd:194 (PlanFor rough rule); msg:252-255 | Tdom `…/the rough rule warns of a close-out` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 合約那一輪記下方向、槓桿、名目 — When 做空並送出建議 Then 記下方向做空、槓桿 5、名目 5,000；讀回時一併給出 | record short / 5 / 5000; read-back shows all three | repo:84-88; entities/strategy_bot_run_record.go:57-59,84-86 | Tper `RemembersWhichWayAndHowFar…` (short fixture, read back); Tent `…CarriesWhatAContractRoundSuggested`; Tapp:219-221 (long, write side) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 現貨那一輪不記 — Then 執行紀錄沒有方向、槓桿、名目，讀回的內容與今天一字不差 | spot record has none of the three; read-back identical | repo:84 (`ForContract` gate); dto omitempty | Tper `KeepsASpotRoundAsItWas`; Tent `…LeavesASpotRoundAsItWas` (JSONEq exact) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 交易所不收的那一輪沒有數字可記 — Then 沒有開倉金額、止損價、止盈價、方向、槓桿與名目 | none of the six recorded | repo:78-79 | Tper `RemembersNothingTheVenueWouldHaveRefused` (Tper:80-100) | shallow — fixture has no take-profit and SuggestedTakeProfitPrice is never asserted nil | produces-oracle | 🟠 mis-asserted |
| AC-21 | 記下的是取整後的數字 — Given 跳動單位 0.1、停損 2.03% … Then 執行紀錄記下的止損價是 98 | recorded stop 98 (rounded, not 97.97) | ppd:273 → repo:90-92 | Tapp:218 — but scenario uses tick 0.01 and 2% stop (Tapp:190), where 98 needs no rounding | shallow — would pass if the unrounded stop were recorded | produces-oracle (record stores the DTO's already-rounded stop) | 🟠 mis-asserted |
| AC-22 | 現貨訊息一字不差 — Then 訊息與今天一字不差，沒有數量、預估強平價、資金費率 | spot message unchanged; no qty/liq/funding | msg:166-255 (all new branches gated on contract-only flags); svc:422 | Tdom `StrategyBotMessageKeepsASpotSuggestionAsItWas`; pre-existing spot message tests untouched | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 只有合約機器人、有部位規劃、這一輪要建議做多或做空時，才讀交易規格、分級與最近一次資金費率結算 | venue read only for contract + plan + long/short | svc:422-437; ppd:208 | strict gomock: spot plan tests and Tapp `ReadsAContractConclusionByTheRulesTradingMode` (contract, plan, nothing to open) would fail on an unexpected read | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 建議照合約重演的開倉規則算… 數量往下取整、實際名目＝數量×參考價、實際保證金＝實際名目÷槓桿、止損止盈對齊跳動單位 | as AC-1..3 | ppd:244-285 (reuses ContractPositionTermsDomain.OpenFor, no costs/slippage) | Tdom `StepsAContractSuggestionToTheVenue` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 交易所不收：數量低於最小下單量、名目低於最小名目、名目那一級最高槓桿低於機器人的槓桿（判斷順序照這個） | first broken rule in that order is the one named | rules:146-190 | none breaks two rules at once; order unpinned | no-test (for the ordering) | produces-oracle | 🟡 partial |
| BR-4 | 預估強平價：逐倉，名目所在那一級的 r 與 c … 做多 (qE−M−c)÷q(1−r)、做空 (qE+M+c)÷q(1+r)；照跳動單位取整；沒有分級時用最小那一級並說明 | formula values; rounded to tick; smallest-tier fallback noted | contract_backtest_position_domain.go:85 (reused); ppd:290 | Tdom AC-7/8/10 values; replay tests in contract_backtest_domain_test.go:604-615 pin the notional's-tier rate | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 強平警告：做多止損 ≤ 強平價、做空止損 ≥ 強平價即警告；估不出強平價時退回「停損距離×槓桿 ≥ 100%」 | comparison per direction; rough fallback | ppd:298-302, ppd:194; msg:250-255 | Tdom `WarnsOfAStopBeyond…`, `WarnsOfAShortStop…`, AC-17 case | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 資金費率估算：金額＝實際名目×\|最近費率\|；正做多付做空收，負反過來；零寫「不付也不收」 | amount, sign by direction, zero wording | ppd:317-336; msg:231-236 | Tdom `EstimatesWhatFundingCosts` (5 cases incl. zero) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 執行紀錄：合約那一輪有建議時記方向、槓桿、名目（取整後）；其他情況不記 | as AC-18..21 | repo:78-88 | Tper ×3; Tent ×2; Tapp:214-221 | asserts-oracle | produces-oracle | ✅ conforms |
| EC-1 | 部位資金押不下（固定金額超過部位資金）→ 照今天說押不下，不讀交易規格 | "can't afford" as today, AND no venue read | svc:422 gates on `Suggests` (ppd:208: capital>0, reference, long/short) — affordability not checked, so svc:429-437 read symbol, tiers and funding before ppd:229-232 discovers unaffordable | Tdom `…SuggestsNothingPlanForWouldNot` (domain only; no read assertion) | shallow (message half only) | diverges — reads venue on an unaffordable round | 🔴 violation |
| EC-2 | 讀交易規格、分級或資金費率失敗 → 當作沒有那一樣，這一輪照常送出 | failed read ≡ absent; round still delivered | svc:429-437 | Tapp2 `StillSuggestsWhenTheVenueCannotBeRead` | asserts-oracle | produces-oracle | ✅ conforms |
| EC-3 | 預估強平價不大於零 → 說「這個槓桿下不會被強平」，不給價位，也不警告 | says won't be liquidated at this leverage; no price; no warning | ppd:288,298; msg:217-218 (「這個槓桿下不會被強制平倉」— same meaning, longer form) | Tdom `SaysALowLeverageLongCannotBeClosedOut` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 現貨機器人與不需要建議的合約那一輪，不多讀任何資料 | no extra reads | svc:422-427 | strict gomock (as BR-1) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 現貨機器人的訊息與執行紀錄一字不變；既有執行紀錄讀回時新增的三樣為空 | unchanged spot; legacy rows read back without the three | entities/strategy_bot_run_record.go:57-59 (default '' / NULL); dto omitempty | Tent `…LeavesASpotRoundAsItWas` (JSONEq); Tper spot | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| msg:226-228 | When the specification gives no funding interval the line reads 「每次結算」 instead of 「每 N 小時」 | undocumented |
| ppd:234-239 | No-specification path still attaches a funding estimate on the unrounded notional (ARCH §4 says so; PRD US-05 silent) | undocumented (ARCH-only) |
| svc:432-434 | A failed tier read alone degrades to "no ladder" → smallest-tier estimate with its note (consistent with EC-2's spirit, not spelled out) | undocumented |
| msg:209-216 | §5 UI says the no-ladder line appends 「（用最小那一級估算）」 after the line; code renders 「預估強平價 80.4（往下，用最小那一級估算）」 inside one parenthesis. §5 is not a clause source; AC-9 is met. | undocumented (formatting deviation from §5) |

Out-of-scope check: no position tracking, no mark-price estimate (reference price used), fees/slippage passed as empty (`BacktestTransactionCostsDomain{}`, `BacktestSlippageDomain{}`), no funding forecast — no scope creep.

## Summary

- Conforms: 27/34 clauses ✅ (79%)
- Violations: EC-1  (unaffordable contract round still reads the venue)
- Mis-asserted: AC-4, AC-15, AC-16, AC-20, AC-21  (green tests skip part of the oracle)
- Partial: BR-3  (refusal ordering unpinned)
- Gaps: none
- Unclear: none
- Orphans: 4 (all undocumented, none out-of-scope)

Ceiling: static conformance audit. Only the mapped tests in domain/application/entity packages were run as corroboration (all green); persistence tests were judged by reading.

---

## Follow-up — findings resolved after the audit

| ID | Finding | Resolution |
| :--- | :--- | :--- |
| EC-1 | A stake the capital cannot cover still read the venue | `PositionPlanDomain.NeedsVenue` now also requires the stake to be affordable; `TestStrategyBotRunApplicationReadsNoVenueForAStakeItCannotPutDown` (venue readers carry no expectations) + domain assertion. → conforms |
| AC-4 | Refused suggestion's take-profit not asserted absent | Asserts `HasTakeProfit` false and no 「止盈」 in the message. → conforms |
| AC-15 | No-settlement case did not rule out a figure | Asserts neither 「約付」 nor 「約收」. → conforms |
| AC-16 | Only the stop was asserted without a specification | Asserts the take-profit (104) too. → conforms |
| AC-20 | Refused suggestion carried no take-profit | Test gives it one and asserts none is stored. → conforms |
| AC-21 | Recorded stop was 98 without any rounding | `TestStrategyBotRunApplicationRemembersTheRoundedContractStop` (0.1 tick, 2.03% stop → 98). → conforms |
| BR-3 | Rule order unpinned | Refusal table case breaking both minimums asserts `belowMinimumQuantity`. → conforms |
| O-1..O-4 | Undocumented behaviour | Added to PRD §4 Edge Cases. → reconciled |

After follow-up: 34 of 34 clauses conform; 0 orphans.
