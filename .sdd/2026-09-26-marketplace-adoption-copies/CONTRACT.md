# Contract Verification — marketplace-adoption-copies

**Contract:** `.sdd/2026-09-26-marketplace-adoption-copies/PRD.md` (v1.0, Finalized)
**Map:** `ARCH.md` (same folder) · `.sdd/UL-MAP.md`
**Implementation:** branch `feat/marketplace-adoption-copies` (`git diff main...HEAD`)
**Date:** 2026-09-26

> **Ceiling.** This is a static conformance audit. Each oracle was derived from the PRD before any code was read. Tests and production code were then judged separately against that oracle. The only tests run were the mapped Postgres migration and repository tests, as corroboration: `TestMigrate*` (6) and `TestDeletingAStrategyScriptTakesItsPublicationButLeavesEveryMarketplaceCopy`. All passed, and none were skipped. No new probes were written, and the full suite was not the basis of any verdict.

Out-of-scope negative checklist (PRD §1): remember the source · new-version notice · "already adopted" badge in the marketplace · renaming a copy · seeing a copy's algorithm. **None of these is implemented.** The copy keeps no link to its original (`strategy_script_marketplace_copy_domain.go:63-74`), and a rename is refused like any other rewrite (`strategy_script_service.go:126-129`).

---

## 1. Clauses

| ID | Clause (verbatim title) | Oracle (from spec) | Implementation | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-01 | 加入建立一份屬於採用者的副本 | A now has one more script "動能" that A owns and that is marked as adopted from the marketplace. Its name, description, algorithm, result kind, market data kind and parameters match B's script at the moment of adoption | `strategy_script_marketplace_service.go:79-102`; `strategy_script_marketplace_copy_domain.go:49-75` | `strategy_script_marketplace_application_test.go` "gives the adopter a copy of their own, marked as adopted" (asserts every field and the parameters); controller `strategy_script_marketplace_controller_test.go` "adopting copies it…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-02 | 再加入一次撞名 | The adoption is refused with the message 「策略腳本名稱「動能」已被使用」, and A still has exactly one "動能" | Save → unique index → `strategy_script_repository.go:137` wording | marketplace app test "a name the adopter already holds is refused": the mock `Save` returns the bare sentinel `ErrStrategyScriptNameConflict` | shallow: the wording and "A 仍只有一支" are never asserted on this path, because the mock stands in for the index that enforces them | produces-oracle | 🟠 mis-asserted |
| AC-03 | 與自己既有的同名 | Refused with the same name-taken message | same as AC-02 | same mocked test (no separate case for "A's own, not a copy") | shallow | produces-oracle | 🟠 mis-asserted |
| AC-04 | 沒發佈的加入不了 | A is told "not found" | `strategy_script_marketplace_service.go:92-94` | marketplace app "reports one that is not on the marketplace…"; controller "…answers not found" (Save unstubbed) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-05 | 加入自己的不做任何事 | A's set of scripts is unchanged: nothing is created and there is no error | `strategy_script_marketplace_service.go:88-91` | "adopting one's own does nothing…" (Save unstubbed, so gomock fails on any copy) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-06 | 作者改寫、收回或刪除都不影響副本 | After B rewrites, withdraws or deletes the original, A's copy has the same content and still runs | Copy is an independent row with its own parameters (`copy_domain.go:52-74`); nothing links back | `strategy_script_marketplace_repository_test.go` `TestDeletingAStrategyScriptTakesItsPublicationButLeavesEveryMarketplaceCopy` (checks only that the copy still exists after a delete) | shallow: no test for rewrite or withdraw, none for unchanged content, none for "跑得動" | produces-oracle | 🟠 mis-asserted |
| AC-07 | 副本看不到算式 | A sees the name, description and parameters, and that the script was adopted from the marketplace, but no algorithm | `entities/strategy_script.go:37-56` | `strategy_script_application_test.go` "reading it shows no algorithm…"; `TestStrategyScriptToDtoNeverCarriesACopysAlgorithm` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-08 | 副本改不動 | The rewrite is refused with 「從市集加入的策略腳本不能改寫」, and nothing is written | `strategy_script_access_domain.go:34-43`; `strategy_script_service.go:126-129` | "rewriting it is refused and nothing is written" (Update unstubbed, wording asserted) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-09 | 副本不能再發佈 | Publishing is refused with 「從市集加入的策略腳本不能再發佈」 | `strategy_script_marketplace_service.go:39-42` | `TestStrategyScriptMarketplacePublishRefusesAMarketplaceCopy` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 不要了就刪掉副本 | The copy is deleted, and B's original is untouched | `strategy_script_service.go:153-161` (ownership passes for the copy; only its own ID is deleted) | "deleting it works like deleting any of one's scripts" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 列出可用策略腳本時副本在採用來的那一組 | The own group holds only "均線". The adopted group holds "動能" with no algorithm and under the copy's own ID | `strategy_script_service.go:59-79` | app "hands back marketplace copies apart…" (checks ID 2, flag, empty script); controller list tests | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 取消加入不再是一個動作 | No action exists for cancelling an adoption | `dependencies.go` route removed; application, service and controller methods removed | `routes_test.go` `TestMountedRoutesAreExactlyTheOnesIntended` (exact list) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 交易策略可以指名副本 | A trading strategy that names A's copy is saved | `strategy_script_access_domain.go:47-50`; `trading_strategy_application.go:165` | `TestTradingStrategyApplicationBuildsOnlyOnThisPersonsOwnScripts` "a copy adopted…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 交易策略不能直接指名別人的 | Both **creating and rewriting** a trading strategy that names B's published script are refused with 「這支策略腳本不是你的，請先把它加入你的策略腳本」 | `trading_strategy_application.go:157-170` (shared by Create, Update and Inspect) | same test covers **create only**; controller test maps create to 400 | shallow: the rewrite path is not tested | produces-oracle | 🟠 mis-asserted |
| AC-15 | 重演交易策略時只接受自己的 | A replay whose source is someone else's script is refused with the same sentence | `trading_strategy_backtest_application.go:107` | `TestTradingStrategyBacktestApplicationRefusesToReplaySomeoneElsesScript` (published foreign script, wording asserted) | asserts-oracle | produces-oracle (note: a foreign *unpublished* script answers "not found" at `access_domain.go:55`, which fits the rule that existence is never disclosed) | ✅ conforms |
| AC-16 | 機器人一輪遇到不是自己的就停 | The bot halts, with the reason that a strategy script is no longer available | `strategy_bot_run_application.go:374`; `strategy_bot_round_failure_domain.go:18-22` | `strategy_bot_run_application_test.go` "a strategy script that belongs to someone else" → `StrategyBotHaltStrategyScriptUnavailable`; round-failure domain test | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 一次性試算照舊可以用別人已發佈的 | Both indicator calculation and strategy-script backtest on B's published script return a result | `indicator_calculation_application.go:81`, `backtest_application.go:79` (still `ResolveRunnableStrategyScript`) | `foreign_strategy_script_failure_application_test.go` indicator cases do run someone else's script as far as the proxy; this slice adds nothing for the strategy-script backtest | shallow: no mapped test shows the backtest half returning a result | produces-oracle | 🟠 mis-asserted |
| AC-18 | 既有的加入換成副本 | After the update, A has a "動能" marked as adopted from the marketplace | `schema_migrator.go:642-656` | `TestMigrateTurnsAnOldAdoptionIntoACopyAndDropsTheAdoptions` (name, flag, script, parameter) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 依賴別人腳本的信號來源改指副本 | The signal source now points at A's copy, **and the copy's algorithm equals B's at update time** | `schema_migrator.go:672-693` | `TestMigratePointsASignalSourceNamingSomeoneElsesScriptAtTheOwnersCopy` checks the count and that the source was repointed | shallow: the copy's algorithm is not asserted in this test | produces-oracle | 🟠 mis-asserted |
| AC-20 | 既有加入與既有依賴只換成一份 | A has exactly one copy, and the source points at it | `schema_migrator.go:606-609` (map keyed by owner and original) | `TestMigrateMakesOneCopyForAnAdoptionAndASourceOfTheSameScript` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 撞名時加上標記 | The copy is named 「動能（市集）」 | `copy_domain.go:33-47`; `schema_migrator.go:618-631` | `TestMigrateNamesACopySoItClashesWithNoneOfTheOwnersScripts` "a clash is marked"; domain table test | asserts-oracle | produces-oracle (see Orphan O-2 for long names) | ✅ conforms |
| AC-22 | 再撞名就編號 | The copy is named 「動能（市集 2）」 | same | same test "…numbered"; domain case "numbering goes on" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 記加入的清單被移除 | The adoptions table no longer exists | `schema_migrator.go:700`; removed from AutoMigrate list (`:81`) | `TestMigrateTurnsAnOld…` `assert.False(HasTable)` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 再更新一次什麼都不多 | A second startup creates no copies and leaves the signal sources unchanged | Copies repoint their sources to the same owner, so the second pass skips them (`:675`); the missing table skips the adoption pass | `TestMigrateAgainMovesNothingMore` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 副本的算式失敗時助手只拿到固定一句話 | When A's copy fails, the assistant receives only 「這支策略腳本不是你的，它執行失敗，不提供細節」 | `access_domain.go:101` (`AuthoredByViewer` false for a copy) → existing foreign marking → `assistant_errors.go:27` | indicator test "the viewer's marketplace copy failing…is marked"; the sentence mapping is pinned by `assistant_conversation_service_test.go:477` | asserts-oracle (in two links) | produces-oracle | ✅ conforms |
| AC-26 | 助手改不了副本 | The assistant receives 「從市集加入的策略腳本不能改寫」, and no pending revision is left | `assistant_revision_service.go:52-55` → `Inspect` → `preparedRewrite` refusal before `pendingRevisionRepository.Save` | `TestStrategyScriptUpdateAssistantQueryCannotProposeRewritingAMarketplaceCopy` (Save and Update unstubbed, wording asserted) | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 副本是一支普通的策略腳本，擁有者是採用者，多一個「從市集加入」標記 | The copy is an ordinary script row owned by the adopter, with a flag | `entities/strategy_script.go:22` | AC-01 test | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 副本：讀（不含算式）、執行、刪除；不能改寫、發佈 | Read without the algorithm, run, and delete are allowed; rewrite and publish are refused | access domain and services as in AC-07 to AC-10 and AC-13 | AC-07 to AC-10 and AC-13 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 「別人寫的算式」：不是提問者的，或是從市集加入的 | An algorithm counts as foreign when it is not the asker's or when the script is a copy | `access_domain.go:101` | `TestStrategyScriptAccessDomainTreatsAMarketplaceCopyAsItsAuthorsWords`; backtest "…marketplace copy" cases | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 搬移只在需要時做事（冪等）；同一位擁有者對同一支在一次搬移裡只建一份 | Idempotent, with one copy per owner and script | `schema_migrator.go:601-705` (one transaction) | AC-20 and AC-24 tests | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 搬移找不到原本那一支（已刪除）的信號來源維持原樣 | A source whose original was deleted is left pointing where it did | `schema_migrator.go:673-676` (`!scriptExists` → continue) | — | no-test | produces-oracle | 🟡 partial |
| NFR-1 | Security: 副本仍拿不到算式；別人的腳本不再能進入任何機器人的規則 | No response or assistant tool carries a copy's algorithm. Every path that builds or runs bot rules accepts only the owner's scripts | Only `StrategyScriptDto` serialises the algorithm, and it is blanked for copies (`strategy_script.go:38-41`). `get_`/`list_strategy_scripts` go through it. All three bot-rule resolvers use `ResolveOwnedStrategyScript` (the only remaining `ResolveRunnable` callers are one-shot runs) | AC-07, AC-11, AC-13 to AC-16 tests | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | Compatibility: 「採用來的」那一組改成副本（同形狀、不含算式）；取消加入的路徑移除 | `adopted` has the same shape as own scripts, with no algorithm, and the cancel-adoption route is gone | `available_strategies_dto.go:9`; route removed; Postman updated | controller list tests, `routes_test.go` | asserts-oracle | produces-oracle | ✅ conforms |

---

## 2. Orphans

| # | Behaviour (file:line) | Maps to clause? | Classification |
|---|---|---|---|
| O-1 | **Migration copies scripts the author has *withdrawn*.** The signal-source pass copies any existing foreign script, published or not (`schema_migrator.go:672-693`). An adopter whose bot was already halted because B withdrew the script gets a runnable copy of B's withdrawn algorithm. PRD §4 only speaks to *deleted* originals ("它的機器人本來就已停擺"), and the same reasoning applies to withdrawn ones. Old adoptions cannot hit this, because a withdraw cascade already removed them. | none | Undocumented behaviour: reconcile. Either extend BR-5 to cover withdrawn originals, or confirm this is intended. |
| O-2 | **Migration can abort server startup.** `NamedAvoiding` adds 「（市集）」 or 「（市集 N）」 (4 or more characters) to the original name without checking the 128-character limit (`strategy_script_marketplace_copy_domain.go:39-44`; column `size:128`, `entities/strategy_script.go:14`). A clashing original name longer than about 124 characters makes `transaction.Create` fail (`schema_migrator.go:632-634`). The whole migration then rolls back and `Migrate()` returns an error on every restart. No clause covers name length. | adjacent to AC-21 | Latent defect: fix (truncate the base name to fit) and add a test. |
| O-3 | **Stale wording about 採用 and shelves after the table was removed.** UL-MAP rows 修改策略腳本 (`UL-MAP.md:354`, "採用它的人下一次執行拿到的就是新的"), 刪除策略腳本 (`:355`, "所有人對它的採用一併消失") and 取消發佈策略腳本 (`:357`, "清掉所有人對它的採用", "重新發佈不會讓別人的採用自己回來") now contradict the copy model. The same outdated wording is in code comments at `published_strategy_script_repository.go:39`, `i_published_strategy_script_repository.go:16`, `strategy_script_service.go:152,164`, `strategy_script_marketplace_service.go:48` and `entities/strategy_script.go:26`. | none | Document drift (CLAUDE.md forbids drift): update UL-MAP and the comments. |

---

## 3. Summary

| Status | Count | Clauses |
|---|---|---|
| ✅ conforms | 26 | AC-01, 04, 05, 07–13, 15, 16, 18, 20–26; BR-1–4; NFR-1, NFR-2 |
| 🔴 violation | 0 | — |
| 🟠 mis-asserted | 6 | AC-02, AC-03, AC-06, AC-14, AC-17, AC-19 |
| 🟡 partial | 1 | BR-5 |
| ❌ gap | 0 | — |
| ❔ unclear | 0 | — |
| ⚠️ orphan | 3 | O-1, O-2, O-3 |

**Conformance:** 26 / 33 = **79%**

Leak sweep (no finding): the copy's algorithm reaches only `RunnableStrategyScriptDto`, which is internal and never rendered. `StrategyScriptDto.ToDto` blanks it, so `GET /strategy-scripts[/:id]`, `get_strategy_script`, `list_strategy_scripts` and the assistant revision `Inspect` all return it empty. Rewriting and publishing a copy are refused before any pending revision is stored. No bot-rule path still calls `ResolveRunnableStrategyScript`.


---

## Resolution (after this audit)

| Finding | Resolution |
| :--- | :--- |
| O-2 撞名加標記可能超過名稱長度上限，讓搬移每次啟動都失敗 | 原名先截短再加標記；新測試（128 字的名稱加上「（市集）」仍是 128 字），經變異驗證 |
| O-1 搬移會複製作者已收回的腳本 | 決定不複製：它的機器人本來就已停擺，複製等於推翻收回；寫進 PRD，新測試經變異驗證 |
| O-3 UL-MAP 與註解仍描述舊的加入清單 | 更新刪除、取消發佈、修改策略腳本三列與六處註解 |
| AC-02 / AC-03 沒驗撞名的那一句 | 斷言加入原樣轉達「策略腳本名稱「二十根均線」已被使用」 |
| AC-06 只驗刪除 | 新增儲存測試：作者改寫並收回原本那一支，副本的算式不變 |
| AC-14 只驗建立 | 新增改寫交易策略時指名別人腳本被拒絕 |
| AC-17 腳本回測那一半 | 既有測試已涵蓋（`TestRunBacktestWalksTheSameGatesACalculationWalks`「somebody else's published strategy script replays」） |
| AC-19 沒驗算式相同 | 斷言副本算式與原本那一支相同 |
| BR-5 沒測原本那一支已刪除 | 新增：指名不存在腳本的信號來源維持原樣 |
