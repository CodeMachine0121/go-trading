# Contract Traceability Matrix — 一份交易策略看多種粗細的 K 線（後端）

Contract: PRD.md
Design map: ARCH.md
Implementation: go-trading `feature/multi-interval-trading-strategy`
Oracle: Acceptance Criteria (18 backend clauses) + Business Rules + NFR；US-07 兩條屬前端，見前端 CONTRACT.md

## Clauses

`Test audit` / `Code audit` 為 Claude 的影子判斷（先記錄、不因 Jev 回答修改）。`Status` / `Route` 為 jev_audit.py 回傳，未改寫。多數 escalate 來自 Jev 對程式碼判讀的低信心（<0.6），Notes 記錄回頭核對的結果。

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Jev test | Jev code | Status | Route | Notes |
|----|--------|------------------------|------|------|------------|------------|----------|----------|--------|-------|-------|
| AC-1 | 三種刻度的交易策略存得下去 | 存下來，讀回時一天、四小時、十五分鐘 | trading_strategy_signal_sources_domain.go:106 | trading_strategy_domain_test.go:258 | asserts-oracle | produces-oracle | asserts-oracle 0.87 | unclear 0.19 | ❔ unclear | escalate (disagree) | 核對：刻度一致檢查已刪，每個來源照自己的刻度寫回 entity；測試斷言 ToEntity 的刻度序列 |
| AC-2 | 刻度相同時照舊存得下去 | 存下來，兩個都是一小時 | 同上 | trading_strategy_domain_test.go:258 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.49 | ✅ conforms | escalate (low confidence) | |
| AC-3 | 只有一個信號來源時照舊存得下去 | 存下來，那個來源是一天 | 同上 | trading_strategy_domain_test.go:311 | asserts-oracle | produces-oracle | asserts-oracle 0.60 | produces-oracle 0.95 | ✅ conforms | recheck | |
| AC-4 | 刻度不在六種之內時拒絕 | 拒絕，說明只能是六種之一 | trading_strategy_signal_sources_domain.go:70 | trading_strategy_domain_test.go:300 | asserts-oracle | produces-oracle | asserts-oracle 0.96 | produces-oracle 0.92 | ✅ conforms | accept | |
| AC-5 | 日線＋四小時＋十五分鐘以十五分鐘為步調 | 逐根十五分鐘；輪次記十五分鐘 | replay_pace_domain.go:19、backtest_run_domain.go:104 | replay_pace_domain_test.go:21、backtest_run_domain_test.go:188 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.64 | ✅ conforms | recheck | 審核中發現輪次原本記第一個來源的刻度，已修正 |
| AC-6 | 日線＋四小時以四小時為步調 | 逐根四小時；輪次記四小時 | 同上 | replay_pace_domain_test.go:21、backtest_run_domain_test.go:199 | asserts-oracle | produces-oracle | asserts-oracle 0.48 | produces-oracle 0.76 | ✅ conforms | escalate (low confidence) | |
| AC-7 | 全部同一種刻度時步調就是那一種 | 逐根一小時；記一小時 | replay_pace_domain.go:19 | replay_pace_domain_test.go:21、trading_strategy_backtest_application_test.go:147 | asserts-oracle | produces-oracle | asserts-oracle 0.89 | produces-oracle 0.95 | ✅ conforms | accept | |
| AC-8 | 新的一天第一格用前一天的日線 | 3/3 00:00 那格說買進 | replay_pace_domain.go:65 | replay_pace_domain_test.go:111 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.55 | ✅ conforms | escalate (low confidence) | |
| AC-9 | 一天的最後一格收盤時那一天的日線剛好走完 | 3/3 23:45 那格說賣出 | replay_pace_domain.go:65 | replay_pace_domain_test.go:111 | asserts-oracle | produces-oracle | asserts-oracle 0.99 | produces-oracle 0.49 | ✅ conforms | escalate (low confidence) | 邊界以突變（收盤相等視為未走完）驗證，測試會紅 |
| AC-10 | 一天還沒走完時仍用前一天的日線 | 3/3 23:30 那格說買進 | replay_pace_domain.go:65 | replay_pace_domain_test.go:111 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.61 | ✅ conforms | recheck | |
| AC-11 | 粗的那一路還沒有任何一根走完時是持平 | 說持平 | replay_pace_domain.go:76、contract_backtest_service.go:163 | replay_pace_domain_test.go:111、contract_backtest_service_test.go:484 | asserts-oracle | produces-oracle | asserts-oracle 1.00 | produces-oracle 0.91 | ✅ conforms | accept | |
| AC-12 | 日線要回看的根數由日線來數 | 第一格起潮汐就有意見 | replay_pace_domain.go:33、backtest_domain.go:173 | contract_backtest_service_test.go:589 | asserts-oracle | produces-oracle | asserts-oracle 0.89 | produces-oracle 0.56 | ✅ conforms | escalate (low confidence) | 原測試只驗讀取起點（shallow），已補「第一格就進場、粗來源看到全部暖機根數」的測試並以突變驗證 |
| AC-13 | 同一種刻度時讀進的行情與現在相同 | 讀進 100 根十五分鐘 | backtest_domain.go:173 | trading_strategy_backtest_pace_test.go:72 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.81 | ✅ conforms | recheck | |
| AC-14 | 要讀進的行情超過一次重演的上限時拒絕 | 拒絕，說出上限與出路 | backtest_domain.go:184 | trading_strategy_backtest_pace_test.go:84、contract_backtest_service_test.go:517 | asserts-oracle | produces-oracle | asserts-oracle 0.97 | produces-oracle 0.79 | ✅ conforms | recheck | |
| AC-15 | 同一種刻度的交易策略重演結果與改版前相同 | 明細、成績單、資金曲線相同 | trading_strategy_replay_sources_domain.go:122、backtest_service.go:122 | trading_strategy_backtest_application_test.go:147 | asserts-oracle | produces-oracle | shallow 0.64 | produces-oracle 0.97 | 🟠 mis-asserted | escalate (disagree) | 同刻度路徑不經對齊；改版前既有的同刻度重演測試（期望值未改）全數通過，作為回歸證據 |
| AC-16 | 機器人一輪讓每個來源各讀自己刻度的已走完 K 線 | 結論由各來源意見得出；通知列出刻度 | strategy_bot_run_application.go:330 | strategy_bot_run_application_test.go:808、strategy_bot_message_domain_test.go:66 | asserts-oracle | produces-oracle | asserts-oracle 0.93 | produces-oracle 0.76 | ✅ conforms | recheck | 既有行為 |
| AC-17 | 當天的日線還在走時，機器人用前一天的日線 | 用前一天日線的意見 | indicator_calculation_domain.go:110 | indicator_calculation_domain_test.go:256 | shallow | produces-oracle | asserts-oracle 0.95 | produces-oracle 0.88 | ✅ conforms | escalate (disagree) | 既有「只讀走完格子」規則；測試以小時刻度驗證 |
| AC-18 | 有一個來源算不出意見時與現在相同 | 這一輪不下結論 | strategy_bot_run_application.go:238 | strategy_bot_run_application_test.go:593 | asserts-oracle | produces-oracle | asserts-oracle 0.76 | produces-oracle 0.85 | ✅ conforms | recheck | 既有行為 |
| BR-5 | 宣告訂單流的較粗來源，足跡照它自己的刻度準備 | 四小時那一根帶整根的足跡 | contract_backtest_service.go:178 | contract_backtest_service_test.go:537 | asserts-oracle | produces-oracle | asserts-oracle 0.88 | produces-oracle 0.43 | ✅ conforms | escalate (low confidence) | 原本沒有粗刻度足跡測試，已補並以突變驗證 |
| NFR-1 | 粗的那一路只在自己的刻度上執行算式 | 執行次數＝自己刻度的根數 | backtest_service.go:122、backtest_domain.go:215 | trading_strategy_multi_interval_backtest_application_test.go:21 | asserts-oracle | produces-oracle | asserts-oracle 0.51 | diverges 0.15 | 🔴 violation | escalate (violation) | 核對：測試斷言兩個來源分別收到 8 根與 2 根；「粗來源改讀步調 K 線」的突變會讓測試變紅。判斷 Jev 誤判 |

BR-1（重演步調）、BR-2（粗刻度意見）、BR-3（持平）、BR-4（讀進的行情）、BR-6（同刻度不變）、BR-7（機器人）分別由 AC-5～7、AC-8～10、AC-11、AC-12～14、AC-15、AC-16～18 覆蓋，未另跑 Jev。

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| trading_strategy_replay_sources_domain.go:32 | 沒有任何信號來源的交易策略在重演時拒絕（原本由刻度一致檢查順帶擋下） | undocumented — 保留既有行為，不屬於任何 Out of Scope 項目 |

## Summary

- Conforms: 17/20 clauses ✅（85%）
- Violations: NFR-1（Jev 0.15 信心判 diverges；回頭核對與突變結果皆指向符合）
- Mis-asserted: AC-15（Jev 認為只有回歸證據而無直接比對）
- Partial: 無
- Gaps: 無
- Unclear: AC-1（Jev 程式碼判讀 0.19）
- Orphans: 1
- Jev: ran on 20 clauses
- Needs James: AC-1、AC-15、NFR-1（disagree / violation），以及多條 low confidence；回頭核對的結論寫在 Notes
