# Contract Traceability Matrix — assistant-injection-hardening

Contract: PRD.md (v1.0, Finalized)
Design map: ARCH.md
Implementation: branch `feat/assistant-prompt-injection-hardening` (`git diff main...HEAD`)
Oracle: Acceptance Criteria + Core Business Rules/Edge Cases + NFR (50 clauses)

Ceiling: static conformance audit. Oracles were derived from the PRD before reading code. Verdicts come from comparing test assertions and code paths to the oracle, not from pass/fail. The mapped tests were run once only to corroborate (all green, including the Postgres repository tests).

Path abbreviations:
- `SVC` = internal/domain/service/assistant_revision_service.go
- `SSA` = internal/application/strategy_script_application.go
- `AUD` = internal/domain/models/domains/strategy_script_authorship_domain.go
- `T-REV` = internal/application/assistantqueries/tests/assistant_revision_test.go
- `T-SSQ` = internal/application/assistantqueries/tests/strategy_script_assistant_queries_test.go
- `T-TSQ` = internal/application/assistantqueries/tests/trading_strategy_assistant_queries_test.go
- `T-SSA` = internal/application/tests/strategy_script_application_test.go
- `T-FOR` = internal/application/tests/foreign_strategy_script_failure_application_test.go
- `T-AUD` = internal/domain/models/domains/tests/strategy_script_authorship_domain_test.go
- `T-CNV` = internal/domain/models/domains/tests/assistant_pending_revision_conversation_test.go
- `T-SVC` = internal/domain/service/tests/assistant_revision_service_test.go
- `T-REPO` = internal/infrastructure/persistence/tests/assistant_revision_repositories_test.go

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-01 | US-01 沒有機器人使用的策略腳本照常改得動 | A is rewritten | SSA:47-72 | T-SSA:389 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | US-01 執行中的機器人使用它時拒絕改寫 | Refused with "這幾台機器人正在用它跑：X，請先停止它們"; A unchanged | SSA:62-69; strategy_script_errors.go:20; strategy_bot_service.go:189 | T-SSA:511 (case 1); strategy_script_controller_test (RefusesARewriteWhileTheOwnersRunningBotUsesIt) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-03 | US-01 已停止的機器人不擋 | A is rewritten | strategy_bot_service.go:200 | T-SSA:511 ("a stopped bot") | asserts-oracle | produces-oracle | ✅ conforms |
| AC-04 | US-01 執行中的機器人沒用到它就不擋 | A is rewritten | strategy_bot_repository.go:128-163 | strategy_bot_repository_test.go:158 + T-SSA:389 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | US-01 只要有一台執行中就擋，只點名執行中的那幾台 | Refused naming X only; Y absent | strategy_bot_service.go:198-203 | T-SSA:511 (case 2, NotContains Y) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | US-01 助手的修改在確認時一樣被擋 | Confirm refused with the running-bot sentence; proposal stays pending | SVC:147-152 | T-REV:215 (InOrder claim + reopen) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-07 | US-02 別人的算式以自訂文字失敗 | Assistant gets exactly "這支策略腳本不是你的，它執行失敗，不提供細節"; custom text absent | AUD:33-43; assistant_errors.go:31-37; assistant_conversation_service.go:240 | T-AUD:38; assistant_conversation_service_test.go:472 (NotContains); T-FOR:70 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | US-02 自己的算式失敗照舊交出完整原因 | Reason contains the text | AUD:34 | T-AUD:45; T-FOR:82 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | US-02 別人的算式讀了一個沒宣告的參數 | Fixed sentence; parameter name absent | AUD:38 | T-AUD:52; T-FOR:76 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | US-02 系統自己說的原因照舊交出 | Assistant gets the no-trading sentence verbatim | AUD:38-40 | T-AUD:67 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | US-02 別人的算式算太久也不交出細節 | Fixed sentence | AUD:38 (timeout wraps ErrIndicatorScriptFailed, indicator_script_compartment.go:144, runner.go:213) | T-AUD:59 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | US-02 重演一份交易策略時用到別人的算式 | Fixed sentence | trading_strategy_backtest_application.go:59-62,123 | T-FOR:129 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | US-02 交易策略裡只要有一個別人的來源… | Fixed sentence even when own A fails | AUD:22-25 | T-FOR:136-141; T-AUD:82,89 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | US-02 使用者自己跑時看到的不變 | Person sees the same reason incl. the text | AUD:50-56 (Error() = cause) | T-AUD:113-114; T-FOR (asserts err.Error() unchanged) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | US-02 助手自帶的算式失敗照舊交出完整原因 | Reason contains the failure detail | run_subject_domain.go:58-59 | T-FOR:64 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | US-03 助手改寫既有的策略腳本只留下待確認修改 | A unchanged; one pending proposal on this answer with full content; assistant told "已提出修改，等使用者確認，尚未生效" | SVC:48-98 | T-REV:60 (full entity equality, Update unstubbed, notice) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | US-03 確認後才生效 | A becomes the proposed content; proposal confirmed | SVC:116-156 | T-REV:131 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | US-03 拒絕就什麼都不寫 | A unchanged; proposal rejected | SVC:158-169 | T-REV:268 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | US-03 處理過的不能再處理 | Refused, "這筆修改已經處理過了" | assistant_pending_revision_domain.go:29-32 | T-REV:166 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | US-03 已拒絕的也不能再處理 | Refused, "這筆修改已經處理過了" | same | T-REV:172 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | US-03 提出之後被改過就不能確認 | Refused with the stale sentence; stays pending; A keeps the person's edit | SVC:137-139 | T-REV:178 (no transition, no Update) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | US-03 確認時被一般規則擋下 | Refused with the duplicate-name reason; stays pending | SVC:147-152 | T-REV:223 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | US-03 同一次回答對同一支提出兩次修改 | After confirming one, confirming the other gets the stale sentence | SVC:137 + GORM auto `updated_at` on Update/Save | — (only the generic stale case T-REV:178; no test confirms one and then the other) | no-test | produces-oracle | 🟡 partial |
| AC-24 | US-03 助手改寫既有的交易策略一樣要確認 | T unchanged; a pending proposal **containing T's full rewritten content** | SVC:82-97 | T-REV:325-353 | shallow — asserts only SubjectKind/SubjectID and status; `proposed.Content` never asserted (T-REV:336-337) | produces-oracle | 🟠 mis-asserted |
| AC-25 | US-03 提出時內容就不合法則不留下提議 | Assistant gets the empty-name reason; no proposal | SVC:59-62 | T-REV:86 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | US-03 助手本段對話建立的，沒有機器人引用，直接生效 | S rewritten; no proposal | SVC:78-80; assistant_revision_proposal_domain.go:30-33 | T-TSQ:186; T-SSQ:223 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | US-03 本段對話建立的，但已有機器人引用，就要確認 (**已停止**的機器人) | S unchanged; a pending proposal | trading_strategy_application.go:116-127 (TotalCount) | T-TSQ:218 | shallow — the bot is `running` (T-TSQ:228), so the test would still pass if only running bots were counted; the stopped-bot condition is never pinned for a trading strategy | produces-oracle | 🟠 mis-asserted |
| AC-28 | US-03 本段對話建立的策略腳本，被機器人間接引用 | C unchanged; a pending proposal | SSA:78-97 (TotalCount via FindAllByStrategyScript) | T-REV:98 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | US-03 別段對話建立的要確認 | C unchanged; a pending proposal | SVC:64-65; assistant_created_subject_repository.go:34-50 | T-REV:83 (asks about this conversation) + T-REPO:102 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | US-03 建立新東西照舊直接生效 | Script stored; no proposal | strategy_script_create_assistant_query.go:52-61 | T-REV:115; T-SSQ:152 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | US-03 別人看不到也動不了我的待確認修改 | V gets "找不到"; proposal untouched | assistant_pending_revision_domain.go:24-27 | T-REV:184, T-REV:296; assistant_pending_revision_controller_test.go:131 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | US-03 讀回對話時看得到每一筆待確認修改 | Answer carries both, each with kind/name, full content, proposed time, status | conversation_domain.go:122-156; conversation_repository.go:221 | T-CNV:27; T-REPO:83 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 使用它的執行中機器人…擁有權檢查之後、所有其他內容檢查之前 | Owner's running bots only; stranger gets not-found first; running-bot refusal precedes content errors | SSA:55-71 | T-SSA:511 (owner filter) | no-test for the ordering (no case with invalid content + running bot, nor stranger + bot) | produces-oracle | 🟡 partial |
| BR-2 | 別人的算式失敗：以擁有者判斷，不以有沒有發佈判斷 | Non-owned (even published/adopted) → masked | strategy_script_access_domain.go:70 | T-FOR:70 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 出自算式隔間的一切失敗→固定一句話；開跑前系統原因→原句 | Compartment failures incl. timeout masked; busy/no-trading/etc. verbatim | AUD:38 | T-AUD:59,67,74 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 一次重演裡只要有一個信號來源是別人的… | Whole replay masked | AUD:22-25 | T-AUD:82,89 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 只作用在交給助手的那一份 | HTTP/person sees unchanged | assistant_errors.go:31 (only exit) | T-AUD:113 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 待確認修改的內容是完整的改寫後樣子 | Stored content = what an ordinary rewrite would receive | assistant_revision_proposal_domain.go:45 | T-REV:64-68 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 「提出之後被改過」以目標的最後修改時刻判斷 | Different last-modified → stale | assistant_pending_revision_domain.go:38-46 | T-REV:178 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 助手本段對話建立的：該段對話裡助手建立成功留下的 | Only successful creations in the same conversation count | strategy_script_create_assistant_query.go:58; trading_strategy_create_assistant_query.go:60 | T-REV:115, T-REV:355 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-9 | 提出之前先做一次一般檢查；執行中機器人不在提出時擋 | Not-found/invalid → reason, no proposal; running bot → still proposed | SSA:78-97; trading_strategy_application.go:95-128 | T-REV:86; T-TSQ:218 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-10 | Edge: 確認時系統出錯：該筆維持待確認，使用者可再按一次 | Any system error during confirm leaves the proposal pending, re-pressable | SVC:141-152 | T-SVC:152 asserts that a failed reopen leaves it **confirmed** and only reports both errors | mis-asserted (accepts the wrong end state) | diverges — the reopen transition reuses the same request context as Apply; a cancelled/timed-out confirm request fails Apply **and** the reopen, leaving it "confirmed" but never written, after which every press gets "已經處理過了" | 🔴 violation |
| BR-11 | Edge: 目標在提出後被刪除 | Confirm says not found; stays pending | SVC:132-135 | T-REV:394 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-12 | Edge: 一次回答失敗時已留下的待確認修改照樣留著、照樣可以處理 | Proposals of failed/running answers are kept and actionable | entities/assistant_pending_revision.go (written at proposal); conversation_domain.go:127-137 | T-CNV:36-39 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-13 | Edge: 同時按兩次確認：只有一次生效 | One wins; the other gets "已經處理過了"/"已經被改過了" | assistant_pending_revision_repository.go:52-65; SVC:198-215 | T-REV:255; T-REPO:68 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | Security: 只有對話擁有者看得到、處理得了；別人的與不存在的同一句「找不到」 | Identical not-found sentence for both | assistant_pending_revision_errors.go:22-24 | T-REV:184 vs T-REV:315 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | Security: 別人的算式失敗那一句話一個字都不含作者可控的內容 | Constant sentence | assistant_errors.go:27 | T-AUD:38 (exact equality) | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | Performance: 多一次查詢，不影響感受 | No perceptible slowdown | strategy_bot_repository.go:128-163 (two queries) | — | no-test | unclear | ❔ unclear |
| NFR-4 | Compatibility: 既有對話讀回時照舊；沒有待確認修改的一次回答不帶這一項 | Read-back unchanged; field absent when none | conversation_message_dto.go:19 (`omitempty`) | T-CNV:74 | shallow — asserts `Empty` on the DTO slice; no JSON-level check that the key is absent | produces-oracle | 🟠 mis-asserted |
| NFR-5 | Compatibility: 記在一次回答最後一則訊息上 | Answered → on the answer; running/failed → on the question | conversation_domain.go:127-156 | T-CNV:27 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| SSA:94 (`BotReferenceCount: references.TotalCount`) | Counts **every** bot naming the script, including other people's bots on a published script, so an assistant-created script someone else's bot uses needs confirmation. The PRD's direct-apply exception does not say whose bots count; Out of Scope only says others' bots don't *block* a rewrite (they don't here) | undocumented — reconcile (safe direction) |
| strategy_script_revision_applier.go:59-68; entities/assistant_pending_revision.go:23 | The proposal stores and shows the assistant's raw arguments. Unknown keys are shown to the owner but silently dropped when written, so "what you review" can contain things that are never applied | undocumented — reconcile with BR-6 |

## Summary

- Conforms: 43/50 clauses ✅ (86%)
- Violations: BR-10
- Mis-asserted: AC-24, AC-27, NFR-4
- Partial: AC-23, BR-1
- Gaps: none
- Unclear: NFR-3
- Orphans: 2


---

## Resolution (after this audit)

| Clause | Finding | Resolution |
| :--- | :--- | :--- |
| BR-10 | 退回待確認沿用請求的 context，請求中途取消時退不回去 | 退回改用 `context.WithoutCancel`；新測試在寫入途中取消請求，並斷言退回確實成功（已用變異驗證會失敗） |
| AC-24 | 交易策略的待確認修改沒驗內容 | 斷言 `Content` 等於助手送出的引數 |
| AC-27 | 用了執行中的機器人，擋不住「只算執行中」的錯 | 改為已停止的機器人（已用變異驗證） |
| NFR-4 | 只驗清單為空，沒驗回應裡不帶這一項 | 序列化對話後斷言沒有 `pendingRevisions` 這個鍵 |
| AC-23 | 沒有「同一支的兩筆，確認一筆後另一筆作廢」的測試 | 新增兩步測試，第一筆寫入後目標的最後修改時刻前進，第二筆得到「已經被改過了」 |
| BR-1 | 沒有測試釘住檢查順序 | 新增：執行中機器人先於內容錯誤；別人的腳本即使自己的機器人在跑也回答找不到 |
| Orphan：別人的機器人也算引用 | PRD 沒寫 | 寫進 PRD 的 Core Business Rules（往安全的方向，保留行為） |
| Orphan：未知欄位被顯示卻不被寫入 | 使用者看到的與寫入的不一致 | 解析時拒絕未知欄位並寫進 PRD；新測試（已用變異驗證） |
| NFR-3 | 讀不出「不影響感受」 | 維持 unclear：多兩次以索引為條件的查詢，無效能測試 |
