# Contract Traceability Matrix — 回測改為背景工作並留下回測輪次

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` (branch `feature/background-backtest-runs`)
Oracle: Acceptance Criteria (37 scenarios) + Business Rules / Edge Cases (8) + NFR (4) = 49 clauses

Path abbreviations: `ctl/` = `internal/controller/`, `app/` = `internal/application/`, `svc/` = `internal/domain/service/`,
`dom/` = `internal/domain/models/domains/`, `per/` = `internal/infrastructure/persistence/`, `job/` = `internal/job/`,
`scr/` = `internal/infrastructure/script/`, `cfg/` = `internal/config/`. Tests live in each layer's `tests/` folder.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01-1 | 合法的合約腳本回測立刻回覆排隊中的輪次 | 立刻回一筆輪次與編號，排隊中／進行中，沒有成績單 | ctl/backtest_controller.go:50, app/backtest_application.go:64, svc/backtest_run_service.go:70 | ctl TestSubmitContractBacktestEndpoint "a valid contract replay is queued…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-2 | 很短的區間一樣先回輪次 | 回輪次而不是成績單 | app/backtest_application.go:35 | app TestSubmitBacktest "a very short stretch is still queued…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-3 | 區間顛倒當場拒絕、不留輪次 | 當場拒絕並指出區間；清單不多一筆 | app/backtest_application.go:49 (validate before submit) | ctl TestSubmitBacktestEndpointRefusesBeforeQueueing "ends before it starts" (field timeRange; save never expected) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-4 | 指名別人的腳本當場「找不到」 | 找不到；清單不多一筆 | app/backtest_application.go (resolveReplayable) | ctl …RefusesBeforeQueueing "somebody else's" (404, no save) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-5 | 旋鈕名字沒宣告過當場拒絕 | 拒絕並說出「不存在的旋鈕」；清單不多一筆 | dom/backtest_domain.go (Applying) via app validate | ctl …RefusesBeforeQueueing "a knob nobody declared"; app TestSubmitBacktest | asserts-oracle | produces-oracle | ✅ conforms |
| AC-01-6 | 現貨腳本、現貨交易策略、合約交易策略三種回測同樣先回輪次 | 回輪次並記著種類 | ctl/backtest_controller.go:23, ctl/trading_strategy_backtest_controller.go:29/58 | ctl TestSubmitBacktestEndpoint (spotScript), TestTradingStrategyBacktestRouterQueuesTheStrategy… (spotTradingStrategy), TestSubmitContractBacktestEndpoint (contractTradingStrategy) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-1 | 完成的輪次帶完整成績單 | 完成；摘要、交易、權益曲線、成交時點；合約另有模式與槓桿；有驗證段時兩段各一份 | svc/backtest_runner.go:163, svc/backtest_run_service.go:128, dto/backtest_run_dto.go:49 | ctl TestGetBacktestRunEndpointAnswersTheReportExactlyAsTheReplayWrites (result JSON equals the replay DTO, inSample/validation present); per TestBacktestRunResultRepositoryKeepsEachReportWhole | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-2 | 進行中的輪次說得出進度 | 進行中、25%、沒有成績單 | dom/backtest_run_domain.go:115, svc/backtest_runner.go:119 | ctl TestGetBacktestRunEndpointForARunNotYetDone "running…"; dom TestBacktestRunProgressIsTheShareOfBarsDone | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-3 | 腳本出錯的輪次留下那句話 | 失敗，原因是腳本的那句話 | svc/backtest_runner.go:69 | svc TestBacktestRunnerEndsAFailureWithItsReason "the script failing midway" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-4 | 超過時間上限收成失敗 | 失敗，原因「超過時間上限」 | svc/backtest_runner.go (ErrBacktestTimeAllowanceSpent case), svc/replay_allowance.go | svc TestBacktestRunnerEndsAFailureWithItsReason "running past the time allowance" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-5 | 看別人的輪次「找不到」 | 找不到 | per/backtest_run_repository.go:38 | ctl TestGetBacktestRunEndpointForARunThatIsNotTheViewers; per TestBacktestRunRepositoryKeepsTheRequestAndShowsARunOnlyToItsOwner | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02-6 | 到期的輪次不再出現 | 31 天前完成的不在清單、查看找不到 | per/backtest_run_repository.go:223, job/backtest_run_retention_job.go:26 | per TestBacktestRunRepositoryDeletesRunsEndedBeforeTheCutoffWithTheirReports; job TestBacktestRunRetentionJobClearsOutOnlyOnDuty; app TestDeleteExpiredRunsUsesTheRetention | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-1 | 新到舊列出並回總數 | 最新 20 筆新到舊，總數 25 | per/backtest_run_repository.go:56 | per TestBacktestRunRepositoryListsNewestFirstWithTheWholeCount | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-2 | 只看某一種狀態 | 只回 3 筆失敗 | per/backtest_run_repository.go:56 | same, "only the failed ones" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-3 | 每一筆帶摘要數字 | 完成、+25.5%、10.1%、4 筆；不帶交易與曲線 | dom/backtest_run_domain.go (ToDto) | ctl TestListBacktestRunsEndpoint (0.255 / 0.101 / 4, no result) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03-4 | 最多幾筆的預設與上限 | 預設 20；500 → 200 | svc/backtest_run_service.go:105 | ctl TestListBacktestRunsEndpoint (missing, 500, unreadable) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-1 | 到了同時進行上限就排隊 | 記下排隊中；一筆結束後自動進行 | app/backtest_run_application.go:47 | app TestRunClaimedBacktestsFillsOnlyTheFreeWorkerPlaces | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-2 | 每人同時上限滿了就拒絕 | 拒絕，說明「你同時最多 3 筆…等其中一筆結束再送」；不多一筆 | dom/backtest_run_admission_domain.go:20, svc/backtest_run_service.go:70 | ctl TestSubmitBacktestEndpointAtTheOwnerLimit (exact JSON); dom TestBacktestRunAdmissionCaps… | asserts-oracle | produces-oracle | ✅ conforms (wording is the pinned HTTP message, which adds 回測在排隊或進行中) |
| AC-04-3 | 每人上限各算各的 | 乙照常記下 | per/backtest_run_repository.go:89 | per TestBacktestRunRepositoryCountsOnlyQueuedAndRunningRunsOfOneOwner | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-4 | 完成、失敗、已取消的不計入每人上限 | 照常記下 | per/backtest_run_repository.go:89 | same | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04-5 | 要求本身的錯先判 | 拒絕原因是區間，不是上限 | app/backtest_application.go:49 | ctl …RefusesBeforeQueueing (owner at limit in every case); app TestSubmitBacktest "owner at the limit" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-1 | 取消排隊中的輪次 | 已取消，永遠不會開始跑 | svc/backtest_run_service.go:169, per/backtest_run_repository.go:178/108 | ctl TestCancelBacktestRunEndpoint "a queued run…"; per …CancelsOnlyAnActiveRunOfItsOwner; per …FindsNothingToClaimWhenNothingIsQueued (cancelled row) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-2 | 取消進行中的輪次 | 盡快停下、已取消、沒有成績單 | svc/backtest_runner.go:119 | svc TestBacktestRunnerStopsAndStoresNothingOnceItsRunIsCancelled; per cancel "a running run" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-3 | 已結束的不能取消 | 拒絕，說已經結束 | dom/backtest_run_domain.go:173 | ctl TestCancelBacktestRunEndpoint "already ended" (409, 已經結束) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05-4 | 取消別人的「找不到」 | 找不到；仍排隊中 | per/backtest_run_repository.go:178 | ctl cancel "somebody else's"; per cancel "somebody else's queued run" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-1 | 同時認領同一筆只有一台拿到 | 恰好一台，只跑一次 | per/backtest_run_repository.go:108 | per TestBacktestRunRepositoryGivesEachQueuedRunToExactlyOneReplica | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-2 | 分身倒下時進行中的收成失敗、排隊中的不受影響 | 失敗「被重新啟動打斷」；排隊中仍排隊 | per/backtest_run_repository.go:198, app/interrupted_work_application.go:90 | per TestBacktestRunRepositoryFailsOnlyRunningRunsOfVanishedReplicas; app TestInterruptedWorkApplication* | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-3 | 只有一台分身時行為相同 | 重啟前進行中的失敗；排隊中的繼續 | app/interrupted_work_application.go (FailWorkLeftByLastRun) | app TestInterruptedWorkApplicationAtStartupSparesOnlyOtherLiveReplicas | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06-4 | 分身正常關機時放掉手上的輪次 | 失敗「被重新啟動打斷」 | job/backtest_run_worker_job.go:38, svc/backtest_run_service.go:223 | job TestBacktestRunWorkerJobLetsGoOfItsRunsWhenItStops; svc TestBacktestRunnerOnAGracefulStopMarksItsRunInterrupted | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07-1 | 讀腳本宣告的根數當暖機 | 讀起點前 1,500 根；第一筆交易可在 2025-06-09 | dom/backtest_domain.go:151/178 | dom TestBacktestReadsTheDeclaredLookBackBeforeTheStretch, TestBacktestTradesFromTheStretchStartOnceWarmedUp | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07-2 | 起點之前不夠多也照常跑 | 讀那 300 根，照常完成 | dom/backtest_domain.go:188 | dom TestBacktestRunsWithWhateverHistoryThereIsBeforeTheStretch | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07-3 | 暖機那一段不交易也不入成績單 | 暖機沒有交易；成績單與曲線從起點開始 | dom/backtest_domain.go:282, dom/contract_backtest_domain.go (ReplayOver) | dom TestBacktestNeverTradesTheWarmUp, TestContractBacktestNeverTradesTheWarmUpAndReportsFromTheStretch | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07-4 | 沒宣告讀取根數的腳本用系統預設 | 以系統預設根數暖機 | dom/backtest_domain.go:151, svc/backtest_service.go | dom TestBacktestReadsTheDeclaredLookBack… "declaring nothing" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08-1 | 沒設定時用預設值 | 2、3、30 分鐘、30 天 | cfg/application_config.go:390 | cfg TestLoadAppliesTheBacktestRunDefaultsWhenNothingIsSet | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08-2 | 設定不成立時用預設值 | 0／-1／abc → 3 | cfg/application_config.go:393 | cfg TestLoadFallsBackToTheDefaultForAnUnusableOwnerLimit | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09-1 | 實盤 vs 回測當場算、不留輪次 | 當場並排回覆；清單不多一筆 | app/spot_trade_live_comparison_application.go:87, app/contract_trade_live_comparison_application.go:88 | app TestSpotTradeLiveComparisonApplication, TestContractTradeLiveComparisonApplication | asserts-oracle | produces-oracle | ✅ conforms (no run service is reachable from this path, so no row can be written) |
| AC-09-2 | 行情助手當場算 | 助手當場拿到成績單；清單不多一筆 | app/trading_strategy_backtest_application.go:124 | assistantqueries tests; app TestTradingStrategyBacktestForTheAssistantRunsUnderTheOnDemandAllowance (fixture's run service is nil) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 狀態轉移；終態不再變 | 只有 排隊→進行→完成/失敗、排隊/進行→取消；終態不變 | dom/backtest_run_domain.go, per/backtest_run_repository.go:150/178 | dom TestBacktestRunCanBeCancelledOnlyWhileItHasWorkAhead; per …WritesProgressOnlyWhileTheRunIsStillThisReplicasRunningOne | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 排隊順序以送出先後為準，不分使用者 | 最舊的一筆先被拿，不論是誰的 | per/backtest_run_repository.go:108 | per TestBacktestRunRepositoryClaimsTheOldestQueuedRun (oldest belongs to the other owner) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 每人上限計排隊＋進行；同時進行上限計該分身進行中 | 兩種計數各如其述 | per CountActiveByOwner; app/backtest_run_application.go:47 | per …CountsOnlyQueuedAndRunning…; app …FillsOnlyTheFreeWorkerPlaces | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 背景時間上限從進行中開始算 | 上限自開始重演起算 | svc/backtest_runner.go:74 (allowance set when the claimed run replays) | svc …EndsAFailureWithItsReason "running past the time allowance" | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 暖機只影響指標；成績單從起點起算 | 成績單、曲線、交易從起點 | dom/backtest_domain.go:282 | dom TestBacktestNeverTradesTheWarmUp | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 保留期限以結束時間起算；未結束不清 | 結束 >30 天刪除；未結束保留 | per/backtest_run_repository.go:223 | per …DeletesRunsEndedBeforeTheCutoff… | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 記下輪次失敗：整次送出失敗 | 送出失敗，沒有輪次 | svc/backtest_run_service.go:70 | ctl TestSubmitBacktestEndpointWhenTheRunCannotBeRecorded | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 重演讀不到任何 K 線：失敗，原因照既有拒絕原文 | 失敗，原因含「湊不出足夠的 K 線」 | svc/backtest_runner.go:69 | svc TestBacktestRunnerFailsAStretchWithNoCandlesInTheRefusalsOwnWords | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 送出不等重演；列表不載入成績單；進度每 N 根寫一次 | 同左 | svc Submit (no replay), svc List (no result read), svc/backtest_runner.go:119 | ctl submit tests (no candle reads expected); ctl list tests (result repo never expected); svc TestBacktestRunnerWritesProgressEverySoManyBarsAndAtTheEnd | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 輪次只有擁有者看得到、取消得了；未登入不能用 | 同左 | per FindOneOwned / CancelIfActive; routes behind sign-in | ctl TestBacktestRunEndpointsNeedASignedInCaller, TestSubmitBacktestEndpointNeedsASignedInCaller, owner cases above | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 既有四種回測的輸入一字不改 | 舊 body 仍被接受 | controller request models unchanged | ctl submit tests reuse the earlier bodies | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-4 | 四個設定值可調、不成立用預設 | 同左 | cfg/application_config.go:390-401 | cfg TestLoadReadsTheBacktestRunSettings, …FallsBack… | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| svc/backtest_run_service.go:128 | `waitSeconds` read that re-reads once a second up to 60 s | undocumented in the business spec; designed in the technical design and the pinned HTTP contract |
| per/backtest_run_repository.go:56 | list filters by strategy script or trading strategy | undocumented in the business spec; designed in the technical design |
| svc/backtest_runner.go:47 | a panicking replay fails as 「回測服務壞掉了，這筆沒有跑完」; an unknown kind fails as 「認不得的回測種類，這筆沒有跑」 | undocumented safety endings |

None matches an Out of Scope item: no link to journals or bots, no sharing or export, no manual deletion, and no change to how the report's figures are calculated.

## Summary

- Conforms: 49/49 clauses ✅ (100%)
- Violations: none
- Mis-asserted: none (AC-02-1 and AC-05-1 were shallow and BR-7/BR-8 untested; tests added before this matrix was written)
- Partial: none
- Gaps: none
- Unclear: none
- Orphans: 3 (undocumented in the business spec, none out of scope)

Static conformance audit against the Acceptance Criteria: it judges test assertions and code paths against the spec's expected outcomes. It does not run invented scenarios.
