# Contract Traceability Matrix — 合約策略機器人 (contract-strategy-bot)

Contract: PRD.md (`.sdd/2026-09-24-contract-strategy-bot/PRD.md` v1.0)
Design map: ARCH.md
Implementation: branch `feat/contract-strategy-bot`, `git diff main...HEAD` (41 files)
Oracle: Acceptance Criteria — 55 clauses (38 AC Gherkin scenarios, 14 BR (11 core rules + 3 edge cases), 3 NFR)
Audit date: 2026-09-24

> Static conformance audit. Each oracle below was derived from the PRD text alone, before any code or test was opened. Test and code were then each judged against that oracle, separately. The only tests run were the ones mapped to clauses (domains/application/controller/service `-run` subsets, all green), and only as corroboration. No verdict here comes from pass/fail. Storage tests (`internal/infrastructure/persistence/tests`) need `TEST_POSTGRES_DSN` and were judged by reading them.

Path abbreviations: `svc/` = `internal/domain/service/`, `dom/` = `internal/domain/models/domains/`, `app/` = `internal/application/`, `ctl/` = `internal/controller/`, `ent/` = `internal/domain/models/entities/`, `per/` = `internal/infrastructure/persistence/`.
Test files: `dT` = `dom/tests/contract_strategy_bot_domain_test.go`, `aT` = `app/tests/contract_strategy_bot_application_test.go`, `rT` = `app/tests/contract_strategy_bot_run_application_test.go`, `cT` = `ctl/tests/contract_strategy_bot_controller_test.go`, `pT` = `per/tests/contract_strategy_bot_repository_test.go`.

## Clauses

The `Spec-expected` column holds the business-observable oracle from Phase 2. The audit columns check the concrete artifact that oracle maps to through UL-MAP and ARCH.

### US-01 — 機器人說定自己是現貨還是合約

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | 建立一台合約機器人 — 有合約交易策略、BTCUSDT 在合約追蹤名單上；建立行情種類為合約行情的機器人 → 建立成功，行情種類是合約行情 | Creation succeeds, and the saved bot reports its market kind as contract | dom/strategy_bot_domain.go:98; svc/strategy_bot_service.go:532; ent/strategy_bot.go:43 | aT:70 `TestStrategyBotApplicationCreatesAContractBot`; cT:43 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 沒說行情種類即現貨機器人 | Creation succeeds, and the bot's kind is K candle | dom/market_data_kind_domain.go:42; dom/strategy_bot_domain.go:98 | dT:31; aT:163 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | 現貨機器人不得引用合約交易策略 | The whole bot is refused, saying the bot eats K candle and the strategy eats contract data | dom/market_data_kind_domain.go:171-180; svc/strategy_bot_service.go:541 | dT:107 (wording); `dom/tests/contract_trading_strategy_domain_test.go` 「a spot bot may not follow…」 | asserts-oracle (at domain level; the shared `settle` path is covered at application level by the opposite case, AC-04) | produces-oracle | ✅ conforms |
| AC-04 | 合約機器人不得引用 K 線交易策略 | The whole bot is refused, saying the bot eats contract data and the strategy eats K candle | same as AC-03 | aT:101; dT:123 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 行情種類建立後不得更換 — 現貨機器人改成合約行情 → 拒絕、說不得更換／請另建一台；維持原狀 | The update is refused with the "cannot change kind, create another" sentence, and the bot stays unchanged | dom/market_data_kind_domain.go:146-165; svc/strategy_bot_service.go:158; per/strategy_bot_repository.go:58-63 (kind not in update columns) | aT:182 (refusal + `Save` ×0); dT:146 (full sentence); pT:27 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 修改時沒提行情種類即維持原樣 | The rename succeeds, and the bot is still a contract bot | svc/strategy_bot_service.go:153-160 | aT:199; dT:132 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | 認不得的行情種類「期貨」 | The whole bot is refused, saying the kind must be one of K candle or contract | dom/strategy_bot_domain.go:98-101; dom/market_data_kind_domain.go:57 | dT:50 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 只列出合約機器人 | Only the one contract bot is returned | svc/strategy_bot_service.go:88-110; ctl/strategy_bot_controller.go:63 | aT:225 「only contract bots」; cT:91 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 不指定種類即全部列出，每一台帶行情種類 | All three are returned, each carrying its own kind | ent/strategy_bot.go:140 (`MarketDataKindOrDefault` in `ToDto`) | aT:238 (kinds list) | asserts-oracle | produces-oracle | ✅ conforms |

### US-02 — 合約機器人只盯正在追蹤的合約標的

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-10 | 盯合約追蹤名單上的標的 → 建立成功 | Creation succeeds | dom/contract_strategy_bot_market_domain.go:60 | aT:70; dT:178 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 盯不在合約追蹤名單上的 DOGEUSDT → 整台被拒絕，說要先把 DOGEUSDT 加進合約追蹤名單 | The whole bot is refused. The sentence names the symbol and tells the owner to add it to the contract watchlist first | dom/contract_strategy_bot_market_domain.go:61-64 | aT:108; dT:180-183; cT:75 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 標的後來被移出追蹤名單 — 合約行情不再進來；下一輪 → 這一輪被跳過；維持執行中、沒有停擺原因 | That round is skipped: nothing is concluded or sent from it. The bot stays running with no halt reason | app/strategy_bot_run_application.go:459-466 → svc/contract_indicator_calculation_service.go:75-89; dom/strategy_bot_round_failure_domain.go (default → skip) | rT:143 | shallow — the Given is modelled as "no bars stored at all" (`FindLatestBefore` → nil). The test checks only running + no halt reason, not that the round was skipped (e.g. no concluded verdict recorded) | unclear — the round is skipped only when the stored bars fall below the coverage floor. The read is by count, not by time (`SourceCandleLimit`, dom/indicator_calculation_domain.go:187: "gaps are free"), and contract bars are never pruned. So a contract removed from the watchlist most likely keeps being judged on its old bars: the round *concludes* on stale data, with a stale reference price, instead of skipping | ❔ unclear |
| AC-13 | 現貨機器人不看合約追蹤名單 | Same result as today. Never refused because of the contract watchlist | svc/strategy_bot_service.go:546 | aT:163 (contract readers have no expectations) | asserts-oracle | produces-oracle | ✅ conforms |

### US-03 — 合約機器人照合約交易模式說該做什麼

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-14 | 多空反手聽到賣出 → 開頭「做空」、標的後標永續合約、寫出交易模式多空反手 | The headline says 做空 with "BTCUSDT 永續合約". The message contains a line saying the mode is 多空反手 | dom/strategy_bot_market_domain.go:71-90,120-126,141-153; dom/strategy_bot_message_domain.go:67-81 | dT:414 (exact first line + mode line) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 多空反手聽到買入 → 「做多」 | The headline says 做多 | dom/strategy_bot_market_domain.go:77 | dT:221; rT:99 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 只做多聽到賣出 → 「平多」 | The headline says 平多 | dom/strategy_bot_market_domain.go:81-86 | dT:222; dT:453; rT:316 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 只做空聽到買入 → 「平空」 | The headline says 平空 | dom/strategy_bot_market_domain.go:82-83 | dT:224 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 只做空聽到賣出 → 「做空」 | The headline says 做空 | dom/strategy_bot_market_domain.go:79 | dT:225 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 現貨機器人的訊息不變 — 賣出 → 「出場」、沒有永續合約字樣、一字不差 | A spot sell headline is exactly today's 「🔴【出場】{name} · {symbol}」, with no 永續合約 | dom/strategy_bot_market_domain.go:55-57,72-74,97-106,121-123,131-134 | `dom/tests/strategy_bot_message_domain_test.go`:33,45 (unchanged, exact first line); dT:240 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 合約機器人自己的動靜也標出永續合約 — 啟動 → 「已啟動」那則在 BTCUSDT 後標永續合約 | The started message reads "…BTCUSDT 永續合約" | svc/strategy_bot_service.go:486,495 | aT:307; rT:196 (halted message as well) | asserts-oracle | produces-oracle | ✅ conforms |

### US-04 — 合約機器人建議保證金、槓桿與方向正確的出場價

Defaults: capital 1,000, all-in, reference 100, long/short.

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-21 | 做多的建議部位 5x／2%／4% | Margin 1,000, 5×, notional 5,000. Stop 98 (below, loss 100). Target 104 (above, gain 200) | dom/position_plan_domain.go:136-211; dom/strategy_bot_message_domain.go:156-181 | dT:307 (all figures); rT:216 (message: margin line + stop line) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 做空的建議部位方向相反 | Margin 1,000, notional 5,000. Stop 102 (above, loss 100). Target 96 (below, gain 200) | dom/position_plan_domain.go:174-205; dom/strategy_bot_message_domain.go:165-169 | dT:322; dT:414 (「止損 102（往上，虧 100）」「止盈 96（往下，賺 200）」) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 槓桿留白即一倍 → 建議名目 1,000、槓桿 1 倍 | The suggestion shows 1× leverage and notional 1,000 | dom/market_data_kind_domain.go:208-210; dom/position_plan_domain.go:160-161 | dT:68 (only checks that leverage is *stored* as 1) | shallow — no test checks the resulting suggestion (notional 1,000, 「1 倍槓桿」) | produces-oracle | 🟠 mis-asserted |
| AC-24 | 槓桿小於一 0.5 → 整台被拒絕，說出槓桿倍數不得小於一 | Refused with the "leverage below one" sentence (the same one the contract replay uses) | dom/market_data_kind_domain.go:212-215 | dT:71 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 槓桿超過標的最高那一級 125 → 150 被拒，說出上限是 125 倍 | Refused, naming the 125× ceiling in the contract replay's words | dom/contract_strategy_bot_market_domain.go:67-71 (same sentence as dom/contract_backtest_domain.go:62) | aT:116; dT:184 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 標的還沒有分級時不擋槓桿 150 → 建立成功 | Creation succeeds | dom/contract_strategy_bot_market_domain.go:67 (`hasLadder`) | aT:145; dT:189 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 止損距離大到會先被強平 10x／10% → 照常給建議＋警告 | The suggestion is still given, plus the warning 「還沒到止損就會先被強制平倉」 | dom/position_plan_domain.go:195-196; dom/strategy_bot_message_domain.go:186-189 | dT:341 (flag + suggests); dT:430 (message text) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 止損距離在撐得住範圍內 10x／9% → 照常建議、沒有警告 | The suggestion is given with no liquidation warning | same | dT:342 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 平倉的那一輪沒有建議部位（只做多、賣出） | The message has no suggested position | dom/position_plan_domain.go:143-145; svc/strategy_bot_service.go:397 | dT:372; rT:298 (message has no 建議部位, record has no plan, with capital set) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 保證金押不下 固定金額 2,000 → 部位資金不足、押不下 2,000 | The message says the capital cannot cover 2,000 | dom/position_plan_domain.go:150-157; dom/strategy_bot_message_domain.go:146-150 | dT:378 (unaffordable, stake 2000); `strategy_bot_message_domain_test.go`:296 (the shared wording) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 現貨機器人照舊不收槓桿 3 → 措辭與今天一字不差 | Refused in today's exact spot-borrowing sentence | dom/market_data_kind_domain.go:199-206 | dT:96; `app/tests/strategy_bot_application_test.go`:651 (full sentence) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 執行紀錄記下那一輪的建議 5x／2% → 開倉金額 1,000（保證金）與止損價 98 | The round's run record holds stake 1,000 and stop 98 | app/strategy_bot_run_application.go (`suggestingRound`) → existing `RecordRound` | rT:245-249 | asserts-oracle | produces-oracle | ✅ conforms |

### US-05 — 合約機器人照合約行情跑每一輪

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-33 | 以合約行情格判出結論並送出 — 兩來源都買、兩者都買、多空反手、上次賣出 | Signals come from contract bars. The conclusion is buy, 「做多」 is sent, and the last sent signal becomes buy | app/strategy_bot_run_application.go:459-466 | rT:81 (the spot runner has no expectation; asserts 做多 and `LastSentSignal`=buy) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 同樣的結論不重送 | No message is sent | existing `DecideRound` | rT:122 (`expectNoRoundMessage`) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | 參考價是最新一分鐘合約 K 線的收盤價 64,000.5 | The reference is 64,000.5, described as that one-minute contract candle's close | app/strategy_bot_run_application.go:568-575; svc/k_candle_contract_service.go (`GetLatestKCandleContract`); dom/strategy_bot_message_domain.go:96 | rT:101-102; `svc/tests/k_candle_contract_service_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 策略腳本找不到了照舊停擺 | The bot is stopped, with the halt reason "a strategy script is gone" | existing `StrategyBotRoundFailureDomain` | rT:179 (halt reason `StrategyScriptUnavailable`) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-37 | 執行中上限合併計算（未達上限）9 台現貨執行中 → 啟動第一台合約成功 | The start succeeds | dom/strategy_bot_run_state_domain.go:75; per/strategy_bot_repository.go:215-224 (count not split by kind) | aT:280 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 執行中上限合併計算（已達上限）10 台 → 拒絕，說上限是 10 台 | The start is refused, saying the running limit is 10 | dom/strategy_bot_run_state_domain.go:75-78 | aT:281 (sentinel); `dom/tests/strategy_bot_run_state_domain_test.go`:100 (「上限是 10 台」) | asserts-oracle | produces-oracle | ✅ conforms |

### Section 4 — Core Business Rules & Edge Cases

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| BR-01 | 機器人的行情種類二選一、建立決定不得更換；沒說即 K 線；修改沒提即維持、重述同一種不算更換、改成另一種拒絕；認不得拒絕，措辭與交易策略相同 | See AC-01/02/05/06/07. Restating the same kind succeeds. An unknown kind gets the same sentence a trading strategy gets | dom/market_data_kind_domain.go:40-59,146-165 | dT:128-157 (restate, keep, change, unknown) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-02 | 行情種類必須相同：建立與修改時各檢查一次；每一輪照機器人自己的行情種類讀行情 | A kind mismatch is refused on create **and** on update. Each round reads the bot's own kind of market | svc/strategy_bot_service.go:541 (`settle`, called by both Create and Update); app/strategy_bot_run_application.go:464,568 | create: aT:101. **update: no test**. per-round: rT:81 | no-test (update-time half) | produces-oracle | 🟡 partial |
| BR-03 | 合約機器人的標的必須在合約追蹤名單上，建立與修改時檢查；之後被移出不回頭作廢，行情不再進來時跳過該輪 | Refused on create **and** update when not watched. Removal after saving never voids the bot; a round with no data skips | svc/strategy_bot_service.go:546-564 | create: aT:108. **update: no test** (the rename test aT:199 only takes the admit path). Skip half: see AC-12 | no-test (update-time half) | produces-oracle (save gate); skip half tracked in AC-12 | 🟡 partial |
| BR-04 | 合約機器人跑一輪：每來源一次合約指標計算；結論/要不要送/衝突/兩類失敗/停擺原因與現貨一字不差；參考價＝最後一根走完的一分鐘合約 K 線收盤價 | One contract calculation per source, the shared decision and failure rules, and a contract 1m close as the reference price | app/strategy_bot_run_application.go:459-485,568-575 | rT:81,122,179,253 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-05 | 開頭動作：做多「做多」、做空「做空」、只做多賣出「平多」、只做空買入「平空」；現貨照舊（買入／出場） | Headline verb table as stated. Spot keeps 買入／出場 | dom/strategy_bot_market_domain.go:71-90 | dT:212 (full 6-row table); dT:240 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-06 | 合約部位規劃：槓桿留白或零即一倍、小於一拒絕、大於最高級拒絕（無分級不擋）、措辭與合約重演相同；保證金＝倉位大小模式算出的一筆；名目＝保證金×槓桿；止損止盈依方向；虧賺照名目 | Formulas and refusals as stated | dom/position_plan_domain.go:150-208; dom/market_data_kind_domain.go:196-218; dom/contract_strategy_bot_market_domain.go:60-74 | dT:61,169,307,322 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-07 | 停損距離×槓桿 ≥ 100% 時警告（粗略、不計維持保證金） | Warn at ≥100%, say it is rough and that maintenance margin is not counted | dom/position_plan_domain.go:195-196; dom/strategy_bot_message_domain.go:186-189 | dT:334 (boundary 100 vs 90); dT:430 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-08 | 只在目標做多或做空的那一輪建議；平多、平空、持有、讀不到參考價都沒有 | No suggestion for close, hold or missing reference | dom/position_plan_domain.go:139-145 | dT:372; rT:298; `dom/tests/position_plan_domain_test.go`:165-214 (hold / no reference) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-09 | 現貨機器人：送來大於一倍的槓桿照舊拒絕，措辭不變；其餘一字不變 | Spot behaviour and wording unchanged | dom/market_data_kind_domain.go:199-206; spot branches of dom/strategy_bot_market_domain.go | AC-19/AC-31 tests; unchanged `strategy_bot_message_domain_test.go` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | 執行中上限每人 10 台，現貨與合約合併計算 | A single per-owner count of 10 | per/strategy_bot_repository.go:215-224 | aT:274 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-11 | 列出機器人：可選填行情種類只列其中一種；認不得的種類拒絕；不填全部列出，依名稱排序 | Filter works. An unknown kind is refused (not an empty list). Blank lists all, by name | svc/strategy_bot_service.go:87-110; per/strategy_bot_repository.go:138 (`Order("name ASC")`) | aT:225,263; cT:91 (400 for `futures`) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | Edge：已存下的機器人都沒有行情種類紀錄 → 一律讀作 K 線 | Legacy bots read as K candle, with no manual migration | ent/strategy_bot.go:43 (`default:kCandle`), :140 (`MarketDataKindOrDefault`) | pT:39; cT:139 (stored row with blank kind → `"marketDataKind":"kCandle"`) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | Edge：存下之後分級改變使槓桿超上限 → 不回頭作廢；下次修改時才檢查 | Rounds never re-check the ceiling. The next update refuses when it is now exceeded | svc/strategy_bot_service.go:552-564 (save-time only; no round path reads tiers) | Only implicit: rT's `StrategyBotService` gets tier mocks with no expectations. **No test for update refusing after the ladder tightens** | no-test (the "checked on next modification" half) | produces-oracle | 🟡 partial |
| BR-14 | Edge：合約標的沒有任何一分鐘合約 K 線 → 參考價那一行說讀不到，沒有建議部位（與現貨相同） | The message says the latest contract K candle cannot be read, and there is no suggestion | app/strategy_bot_run_application.go:568-575; dom/strategy_bot_message_domain.go:99-101; dom/position_plan_domain.go:139 | rT:253 (message line); dT:443; `position_plan_domain_test.go`:193 (no reference → no plan) | asserts-oracle | produces-oracle | ✅ conforms |

### Section 6 — Non-Functional Requirements

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| NFR-01 | Performance：一輪讀取次數與合約指標計算相同；不為了機器人向來源補抓行情 | A contract round reads what a contract calculation reads, plus one latest-candle read. It never fetches from the exchange | app/strategy_bot_run_application.go:459-485,568-575 (only stored reads) | none. rT uses `.AnyTimes()` on `FindLatestBefore` (rT:55-57), so read counts are not pinned | no-test | produces-oracle | 🟡 partial |
| NFR-02 | Security：歸屬規則照舊；別人的交易策略與不存在的一樣回覆「找不到」 | A stranger's strategy gets the same "not found" as a missing one | app/strategy_bot_application.go:174 (`GetTradingStrategy(viewerID…)` before `settle`) | `app/tests/strategy_bot_application_test.go`:166 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-03 | Compatibility：既有機器人與它們的訊息、執行紀錄一字不變；不需要人工資料遷移 | Spot messages, run records and stored bots are unchanged, and the schema default carries legacy rows | ent/strategy_bot.go:43,81; spot branches of the message/market domains; `StrategyBotRunRecordDto` untouched | unchanged spot message tests; pT:39; cT:139 | asserts-oracle | produces-oracle (but see orphan O-1: the bot *response* key spelling changed) | ✅ conforms |

## Orphans (code with no clause)

None of the orphans below implements a PRD Out-of-Scope item: there is no liquidation-price estimate, funding-cost estimate, slippage, contract-spec rounding, mark-price reference, dual-market bot, whole-bot replay or assistant bot control.

| Code | Description | Verdict |
|------|-------------|---------|
| O-1 `internal/domain/models/dto/position_plan_dto.go:22-36` | `PositionPlanSettingsDto` gains camelCase `json` tags. **Every existing bot's** `positionPlan` response changes from `Capital`/`SizingMode`/… to `capital`/`sizingMode`/…, and `leverage` is `omitzero`. The decision is recorded in ARCH §8 only; the PRD says nothing about it, and it sits against NFR-03's "既有機器人…一字不變" | undocumented (contract-relevant: reconcile with NFR-03) |
| O-2 `internal/domain/service/strategy_bot_service.go:151-156` | A stored bot whose kind cannot be read (e.g. `option`) is refused on rewrite rather than normalised (test aT:358) | undocumented |
| O-3 `internal/domain/models/domains/strategy_bot_market_domain.go:59-61,89,142` | A contract bot whose trading mode cannot be read names no act: the headline falls back to the signal word (「賣出」) with a neutral mark, there is no mode line and no suggestion (test dT:275) | undocumented |
| O-4 `internal/domain/service/strategy_bot_service.go:549-560` | A failure to read the contract symbol or its ladder on save is surfaced as a storage failure, and nothing is saved (test aT:319) | undocumented (benign) |

## Summary

- Conforms: 49/55 clauses ✅ (89.1%)
- Violations: none 🔴
- Mis-asserted: AC-23 🟠 (the test pins only the stored leverage, not the 1× / 1,000 suggestion)
- Partial: BR-02, BR-03, BR-13, NFR-01 🟡 (the update-time gates and the read-count property have no test)
- Gaps: none ❌
- Unclear: AC-12 ❔ (the skip after removal from the watchlist most likely does not happen while old contract bars are stored; confirm via /tdd)
- Orphans: 4 (O-1 conflicts with NFR-03 wording and should be reconciled)


---

## Follow-up — findings resolved after the audit

| ID | Finding | Resolution |
| :--- | :--- | :--- |
| AC-12 | Stale bars left after a contract leaves the watchlist were still judged | Contract rounds now refuse a source whose newest bar is more than one bucket behind the latest finished bucket (`StrategyBotMarketDomain.RequireCurrentBars`, asked through `StrategyBotService.RequireCurrentSourceReading`); the round is skipped. Test asserts no message, last sent signal unchanged, running, no halt, history `hold` (`TestStrategyBotRunApplicationSkipsAContractRoundJudgedByBarsNoLongerArriving`) plus a boundary table in the domain tests. → conforms |
| AC-23 | Blank leverage only checked as stored | Added end-to-end assertion of 「保證金 1000（1 倍槓桿，名目 1000）」. → conforms |
| BR-02, BR-03, BR-13 | Rewrite path untested | `TestStrategyBotApplicationChecksARewrittenContractBotLikeANewOne` covers other-kind strategy, unwatched contract, and a ladder tightened to 20× under a stored 50× bot. → conforms |
| NFR-01 | Read counts not pinned | `TestStrategyBotRunApplicationReadsTheContractMarketOncePerSource` pins bars read once per source and the newest candle once. → conforms |
| O-1 … O-4 | Undocumented behaviour | Added to PRD §4 Edge Cases and §6 Compatibility. → reconciled |

After follow-up: 55 of 55 clauses conform; 0 orphans.
