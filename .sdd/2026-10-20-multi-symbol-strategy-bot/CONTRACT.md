# Contract Traceability Matrix — 一台機器人盯多個交易標的

Contract: PRD.md
Design map: ARCH.md
Implementation: internal/ (go-trading, branch feature/multi-symbol-strategy-bot)
Oracle: Acceptance Criteria (37 clauses, US-01..US-07 in order)

> Static conformance audit: it judges test assertions and code paths against each scenario's expected outcome; it does not execute invented scenarios. Mapped tests were run individually only as corroboration.

## First pass findings and resolution

The first pass found five clauses whose tests did not pin the oracle (AC-7, AC-8 only covered one symbol; AC-10 did not check the dropped symbol's history; AC-14 did not check the re-added symbol's signal and position; AC-26 covered opening, not closing). Tests were added (`TestStrategyBotApplicationRefusesAContractBotWhenAnyOfItsSymbolsCannotBeTraded`, `TestStrategyBotRepositoryKeepsADroppedSymbolsHistory`, the strengthened `TestStrategyBotRepositoryRewriteKeepsTheSymbolsStillListedAsTheyWere`, `TestAnAutoOrderClosesOnlyItsOwnSymbolsPosition`) and those clauses were re-audited; the matrix below shows the final pass.

## Clauses

`Test audit` / `Code audit` are Claude's shadow verdict. `Jev test` / `Jev code` are Jev's picks. `Status` and `Route` are as returned by `jev_audit.py`.

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|
| AC-1 | 建立盯兩個標的的現貨機器人 | 建立成功；監聽標的為 BTCUSDT、ETHUSDT；兩者皆無上次訊號、未停擺 | strategy_bot_domain.go:46-153<br>strategy_bot_repository.go:28-105 | strategy_bot_domain_test.go:325-354<br>strategy_bot_controller_test.go:854-884 | asserts-oracle | produces-oracle | asserts-oracle 0.57 | produces-oracle 0.43 | ✅ conforms | escalate (low confidence) |
| AC-2 | 只盯一個標的 | 建立成功，一個監聽標的 | strategy_bot_domain.go:46-153 | strategy_bot_domain_test.go:325-354 | asserts-oracle | produces-oracle | mis-asserted 0.56 | not-implemented 0.19 | ❌ gap | escalate (disagree) |
| AC-3 | 正好 10 個標的 | 建立成功，10 個監聽標的 | strategy_bot_domain.go:46-153 | strategy_bot_domain_test.go:325-354 | asserts-oracle | produces-oracle | asserts-oracle 0.48 | produces-oracle 0.69 | ✅ conforms | escalate (low confidence) |
| AC-4 | 超過 10 個標的 | 拒絕，說明「一台機器人最多盯 10 個交易標的」 | strategy_bot_domain.go:46-153 | strategy_bot_domain_test.go:55-137<br>strategy_bot_controller_test.go:927-955 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.99 | ✅ conforms | accept |
| AC-5 | 沒有任何標的 | 拒絕，說明「必須指定這台機器人要盯哪些交易標的」 | strategy_bot_domain.go:46-153 | strategy_bot_domain_test.go:55-137<br>strategy_bot_controller_test.go:927-955 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 1.00 | ✅ conforms | accept |
| AC-6 | 大小寫不同的同一個標的視為重複 | 拒絕，說明「交易標的 BTCUSDT 重複了」 | strategy_bot_domain.go:46-153 | strategy_bot_domain_test.go:55-137<br>strategy_bot_controller_test.go:927-955 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.57 | ✅ conforms | escalate (low confidence) |
| AC-7 | 合約機器人其中一個標的不在合約追蹤名單上 | 拒絕，說明「DOGEUSDT 不在合約追蹤名單上」（另一個標的合格也一樣拒絕） | strategy_bot_service.go:712-752<br>contract_strategy_bot_market_domain.go:40-54 | multi_symbol_strategy_bot_application_test.go:158-191 | asserts-oracle | produces-oracle | asserts-oracle 0.96 | produces-oracle 0.95 | ✅ conforms | accept |
| AC-8 | 合約機器人槓桿超過其中一個標的的上限 | 拒絕，說明「XYZUSDT 最高只能開 20 倍槓桿」 | strategy_bot_service.go:712-752<br>contract_strategy_bot_market_domain.go:40-54 | multi_symbol_strategy_bot_application_test.go:158-191 | asserts-oracle | produces-oracle | asserts-oracle 0.59 | produces-oracle 0.97 | ✅ conforms | escalate (low confidence) |
| AC-9 | 增加一個標的，原有標的保留狀態 | 修改成功；BTCUSDT 仍記得買入；SOLUSDT 無上次訊號 | strategy_bot_repository.go:28-105<br>strategy_bot_service.go:120-191 | strategy_bot_watched_symbol_repository_test.go:34-86<br>multi_symbol_strategy_bot_application_test.go:28-81 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.87 | ✅ conforms | accept |
| AC-10 | 移除一個空手的標的 | 修改成功只剩 BTCUSDT；ETHUSDT 過去的執行紀錄仍看得到 | strategy_bot_repository.go:28-105<br>strategy_bot_service.go:120-191 | strategy_bot_watched_symbol_repository_test.go:89-108<br>multi_symbol_strategy_bot_application_test.go:28-81 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.72 | ✅ conforms | recheck (mid confidence) |
| AC-11 | 移除一個還有機器人持倉的標的 | 整次修改拒絕，說明「ETHUSDT 還有機器人持倉，要先讓它空手才能移除」；監聽標的不變 | strategy_bot_service.go:120-191<br>strategy_bot_watched_symbol_domain.go:30-43 | multi_symbol_strategy_bot_application_test.go:28-81<br>strategy_bot_controller_test.go:886-925<br>strategy_bot_run_state_domain_test.go:196-233 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 1.00 | ✅ conforms | accept |
| AC-12 | 移除一個還有沒做完的自動下單委託的標的 | 整次修改拒絕，說明「ETHUSDT 還有沒做完的自動下單，要等它做完才能移除」 | strategy_bot_service.go:120-191<br>strategy_bot_watched_symbol_domain.go:30-43 | multi_symbol_strategy_bot_application_test.go:28-81<br>strategy_bot_controller_test.go:886-925 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 1.00 | ✅ conforms | accept |
| AC-13 | 執行中不能修改 | 拒絕，說明要先停止 | strategy_bot_service.go:120-191 | strategy_bot_application_test.go:262-277 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.75 | ✅ conforms | recheck (mid confidence) |
| AC-14 | 移除後再加回來從空白開始 | ETHUSDT 無上次訊號、空手 | strategy_bot_repository.go:28-105 | strategy_bot_watched_symbol_repository_test.go:34-86 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.63 | ✅ conforms | recheck (mid confidence) |
| AC-15 | 只有信號變了的標的送訊息 | 只送一則 ETHUSDT 出場；ETH 上次訊號變賣出、BTC 仍買入 | strategy_bot_run_application.go:239-303<br>strategy_bot_service.go:423-454<br>strategy_bot_service.go:522-642 | multi_symbol_strategy_bot_run_application_test.go:102-125 | asserts-oracle | produces-oracle | asserts-oracle 0.81 | produces-oracle 0.71 | ✅ conforms | recheck (mid confidence) |
| AC-16 | 兩個標的都有新結論就送兩則 | 送兩則，各寫自己的標的 | strategy_bot_run_application.go:202-236<br>strategy_bot_service.go:522-642 | multi_symbol_strategy_bot_run_application_test.go:127-140 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.57 | ✅ conforms | escalate (low confidence) |
| AC-17 | 一個標的衝突不影響另一個 | BTC 留衝突記號不送；ETH 送買入 | strategy_bot_watched_symbol_domain.go:46-59<br>strategy_bot_service.go:522-642 | multi_symbol_strategy_bot_run_application_test.go:142-164 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | unclear 0.11 | ❔ unclear | escalate (disagree) |
| AC-18 | 合約標的行情停了只跳過那一個 | ETH 跳過，記一筆持有紀錄、不送訊息；BTC 照常判斷並記紀錄 | strategy_bot_run_application.go:239-303<br>strategy_bot_service.go:522-642 | multi_symbol_strategy_bot_run_application_test.go:346-364 | asserts-oracle | produces-oracle | asserts-oracle 0.89 | unclear 0.17 | ❔ unclear | escalate (disagree) |
| AC-19 | 每個標的各記一筆執行紀錄 | 一輪兩筆紀錄，編號接續（101、102）各寫自己的標的 | strategy_bot_run_record_repository.go:28-115<br>strategy_bot_service.go:522-642 | strategy_bot_watched_symbol_repository_test.go:137-171<br>multi_symbol_strategy_bot_run_application_test.go:102-125 | asserts-oracle | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.47 | ✅ conforms | escalate (low confidence) |
| AC-20 | 只看某一個標的的執行紀錄 | 只列 ETHUSDT 的紀錄，新到舊 | strategy_bot_service.go:661-709<br>strategy_bot_run_record_repository.go:123-143 | strategy_bot_controller_test.go:957-984<br>strategy_bot_watched_symbol_repository_test.go:137-171 | asserts-oracle | produces-oracle | asserts-oracle 0.79 | produces-oracle 0.79 | ✅ conforms | recheck (mid confidence) |
| AC-21 | 每個標的各記得 50 輪 | BTC 保持最近 50 筆；ETH 少量紀錄全保留 | strategy_bot_run_record_repository.go:28-115 | strategy_bot_watched_symbol_repository_test.go:137-171 | asserts-oracle | produces-oracle | asserts-oracle 0.81 | diverges 0.24 | 🔴 violation | escalate (violation) |
| AC-22 | 每個標的各自用滿整份部位規劃 | 兩則訊息建議開倉金額都是 1,000 | strategy_bot_run_application.go:428-460 | multi_symbol_strategy_bot_run_application_test.go:166-186 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | unclear 0.59 | ❔ unclear | escalate (disagree) |
| AC-23 | 兩個標的各排一筆自動下單委託 | 排兩筆委託 BTC、ETH，彼此不等待 | strategy_bot_service.go:522-642<br>contract_auto_order_repository.go:40-79 | multi_symbol_strategy_bot_run_application_test.go:325-344<br>strategy_bot_watched_symbol_repository_test.go:173-216 | asserts-oracle | produces-oracle | asserts-oracle 0.88 | produces-oracle 0.77 | ✅ conforms | recheck (mid confidence) |
| AC-24 | 別的標的排進新委託不撤這個標的的進場單 | ETH 進場單不撤 | contract_auto_order_repository.go:135-151 | strategy_bot_watched_symbol_repository_test.go:173-216 | asserts-oracle | produces-oracle | asserts-oracle 0.89 | produces-oracle 0.65 | ✅ conforms | recheck (mid confidence) |
| AC-25 | 同一個標的排進新委託就撤掉它的舊進場單 | ETH 舊進場單被撤 | contract_auto_order_repository.go:135-151 | strategy_bot_watched_symbol_repository_test.go:173-216 | asserts-oracle | produces-oracle | no-test 0.04 | produces-oracle 0.28 | 🟡 partial | escalate (disagree) |
| AC-26 | 平倉只平那一個標的自己的機器人持倉 | 只動 ETH 的持倉，BTC 不變 | contract_auto_order_service.go:172-221<br>contract_auto_order_service.go:509-532 | contract_auto_order_execution_application_test.go:979-998<br>contract_auto_order_execution_application_test.go:953-976 | asserts-oracle | produces-oracle | shallow 0.27 | produces-oracle 0.40 | 🟠 mis-asserted | escalate (disagree) |
| AC-27 | 一個標的上策略腳本算不出來只停那一個 | ETH 標的停擺原因算不出來；收到 ETH 停擺通知說其他照常；BTC 照常、機器人執行中 | strategy_bot_round_failure_domain.go:17-41<br>strategy_bot_round_failure_domain.go:44-61<br>strategy_bot_round_outcome_domain.go:28-55<br>strategy_bot_watched_symbol_round_outcome_domain.go:79-96<br>strategy_bot_watched_symbol_domain.go:61-68<br>strategy_bot_service.go:522-642<br>strategy_bot_lifecycle_message_domain.go:54-57 | multi_symbol_strategy_bot_run_application_test.go:188-210 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.39 | ✅ conforms | escalate (low confidence) |
| AC-28 | 只盯一個標的時與今天相同 | 整台停擺原因算不出來；只收到一則整台停擺通知 | strategy_bot_round_outcome_domain.go:28-55<br>strategy_bot_service.go:522-642 | multi_symbol_strategy_bot_run_application_test.go:212-245 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.80 | ✅ conforms | recheck (mid confidence) |
| AC-29 | 最後一個標的也停擺時整台停擺 | 整台停擺原因算不出來 | strategy_bot_service.go:409-420<br>strategy_bot_round_outcome_domain.go:28-55<br>strategy_bot_watched_symbol_round_outcome_domain.go:79-96<br>strategy_bot_run_state_domain.go:128-136<br>strategy_bot_run_state_domain.go:82-91 | multi_symbol_strategy_bot_run_application_test.go:212-245<br>strategy_bot_run_state_domain_test.go:246-390 | asserts-oracle | produces-oracle | asserts-oracle 0.84 | produces-oracle 0.23 | ✅ conforms | escalate (low confidence) |
| AC-30 | 交易策略找不到時整台停擺 | 整台停擺原因交易策略找不到了 | strategy_bot_run_application.go:202-236<br>strategy_bot_round_outcome_domain.go:28-55 | multi_symbol_strategy_bot_run_application_test.go:247-266 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.56 | ✅ conforms | escalate (low confidence) |
| AC-31 | Telegram 拒絕機器人金鑰時整台停擺 | 整台停擺原因機器人金鑰不被接受 | pending_message_dispatch_application.go:20-24 | pending_message_dispatch_application_test.go:203-257 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | unclear 0.44 | ❔ unclear | escalate (disagree) |
| AC-32 | 重新啟動讓停擺的標的恢復 | ETH 不再停擺；所有標的上次訊號與衝突清空；持倉不變 | strategy_bot_run_state_domain.go:57-70 | multi_symbol_strategy_bot_application_test.go:83-115 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.80 | ✅ conforms | recheck (mid confidence) |
| AC-33 | 執行中的合約機器人完整帶過所有狀態 | 恰好一個監聽標的 BTCUSDT，持倉、止損止盈、上次訊號不變，仍執行中 | schema_migrator.go:850-911 | schema_migrator_test.go:480-530 | asserts-oracle | produces-oracle | asserts-oracle 0.84 | produces-oracle 0.57 | ✅ conforms | escalate (low confidence) |
| AC-34 | 重複升級不改變任何東西 | 第二次升級後狀態不變 | schema_migrator.go:850-911 | schema_migrator_test.go:480-530 | asserts-oracle | produces-oracle | asserts-oracle 0.91 | produces-oracle 0.64 | ✅ conforms | recheck (mid confidence) |
| AC-35 | 舊的執行紀錄歸給原本的標的 | 升級後舊紀錄都寫著 BTCUSDT | schema_migrator.go:850-911 | schema_migrator_test.go:480-530 | asserts-oracle | produces-oracle | asserts-oracle 0.72 | produces-oracle 0.87 | ✅ conforms | recheck (mid confidence) |
| AC-36 | 查一台盯兩個標的的合約機器人 | 看到兩個監聽標的，BTC 上次買入持倉做多 0.01，ETH 停擺原因算不出來 | strategy_bot.go:98-127<br>strategy_bot_watched_symbol.go:46-61 | strategy_bot_test.go:75-117 | asserts-oracle | produces-oracle | asserts-oracle 0.72 | produces-oracle 0.32 | ✅ conforms | escalate (low confidence) |
| AC-37 | 刪除時任一標的有等成交的進場單就拒絕 | 拒絕刪除 | strategy_bot_service.go:194-211 | strategy_bot_application_test.go:290-302 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.97 | ✅ conforms | accept |

### Notes on rechecked / escalated clauses

- **AC-21** — Recheck: Append keeps the 50 newest of the appended (bot, symbol) by cutting below that symbol's 50th-newest number; a manual mutation dropping the symbol filter is killed by the mapped test. Shadow verdict stands; Jev low confidence (0.24).
- **AC-26** — The mapped tests mirror the scenario (BTC closes/opens while ETH holds a position); the bot repository mock accepts a position write for BTCUSDT only, so any write to ETH fails the test. Mutation reading WatchedSymbols[0] is killed.
- **AC-17** — Recheck: Concluded() sets Conflicting per watched symbol; ApplyTo replaces only that symbol. Jev unclear (0.11).
- **AC-18** — Recheck: stale contract market returns ReadRoundFailure(symbol, ErrStrategyBotMarketDataStale) → skipped → hold record, no message.
- **AC-22** — Recheck: composeRoundMessage takes botDto.PositionPlan for every symbol.
- **AC-31** — Unchanged path: PendingMessageService halts the whole bot via StrategyBotRunStateDomain.Halt.
- **AC-2** — Jev disagreement at low confidence; domain test case "exactly one" asserts the single watched symbol.
- **AC-25** — Persistence test asserts ExistsUnsettledAfter(bot, BTCUSDT, first BTC order) is true.

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| strategy_bot_run_application.go `playRound` | Symbols of one round run concurrently (one goroutine per watched symbol) | undocumented in AC; stated in PRD §6 Performance — consistent |
| strategy_bot_run_record_repository.go `FindLatestByBot` | History read capped at 500 records (50 × symbol limit), including removed symbols' leftovers | undocumented detail of the retention rule |

No orphan implements an Out of Scope item (no concurrent-position cap, no per-symbol overrides, no mixed kinds).

## Summary

- Conforms: 29/37 clauses ✅ (78%)
- Violations: AC-21 (Jev low-confidence; shadow verdict produces-oracle, see notes)
- Mis-asserted: AC-26
- Partial: AC-25
- Gaps: AC-2
- Unclear: AC-17, AC-18, AC-22, AC-31
- Orphans: 2 (benign)
- Jev: ran on 37 clauses
- Needs James: AC-1 (low confidence), AC-2 (disagree), AC-3 (low confidence), AC-6 (low confidence), AC-8 (low confidence), AC-16 (low confidence), AC-17 (disagree), AC-18 (disagree), AC-19 (low confidence), AC-21 (violation), AC-22 (disagree), AC-25 (disagree), AC-26 (disagree), AC-27 (low confidence), AC-29 (low confidence), AC-30 (low confidence), AC-31 (disagree), AC-33 (low confidence), AC-36 (low confidence)
