# Contract Conformance — 合約 K 線即時跟盤

**Contract:** `.sdd/2026-09-24-contract-live-follow/PRD.md` · **Implementation:** branch `feat/contract-live-follow`
**Ceiling:** static audit against the acceptance criteria; mapped tests were run only as corroboration.

Paths: `S` = `internal/domain/service/k_candle_contract_follow_service.go`,
`C` = `internal/controller/k_candle_follow_controller.go`,
`ST` = `internal/domain/service/tests/k_candle_contract_follow_service_test.go`,
`CT` = `internal/controller/tests/k_candle_contract_follow_controller_test.go`.

## Clauses

| ID | Clause | Oracle (from spec) | Code | Test | Test audit | Code audit | Status |
|---|---|---|---|---|---|---|---|
| AC-01 | 第一個觀看者開始看 | 開始跟；他立刻收到目前進行中那一根 | S:85-131, `kCandleFollowSymbol.join` | ST:91 (receives forming after report), ST:138 | asserts-oracle | produces-oracle | ✅ |
| AC-02 | 第二個觀看者共用同一份 | 兩人同一份結果；只跟一個合約標的 | S:117-134 | ST:91 | shallow — count is 1, but "only one line opened" is not pinned | produces-oracle | 🟠 |
| AC-03 | 還有人在看就繼續跟 | 繼續跟；剩下那人照常收到 | S:200-220 | ST:116 | asserts-oracle | produces-oracle | ✅ |
| AC-04 | 最後一人離開就停 | 不再跟任何合約標的 | S:200-220 | ST:116 | asserts-oracle | produces-oracle | ✅ |
| AC-05 | 同一個代號的現貨與合約各跟各的 | 合約另外跟、收合約價格；現貨照常 | 兩個 service 各自登記簿 | ST:157 | asserts-oracle | produces-oracle | ✅ |
| AC-06 | 名單上的跟得動 | 開始跟 | S:103 | ST:184 | asserts-oracle | produces-oracle | ✅ |
| AC-07 | 系統不認得的代號 | 被拒絕，**說找不到這個合約標的** | S:100 → C:88 | ST:184, CT:95 | asserts the rejection and 404, not the wording | **diverges** — the message reads `trading symbol not registered: NOPEUSDT` (English sentinel text), not 「找不到這個合約標的」 | 🔴 |
| AC-08 | 認得但不在名單上 | 被拒絕，說要先加進合約追蹤名單 | S:104 → C:94 | ST:184, CT:95 | asserts-oracle | produces-oracle | ✅ |
| AC-09 | 進行中那一根至多每十秒送一次 | 十秒內只送一次 | `kCandleFollowSymbol.pass` | ST:238 | asserts-oracle | produces-oracle | ✅ |
| AC-10 | 走完那一根不等上限 | 立刻收到走完的樣子 | `pass` | ST:238 | asserts-oracle | produces-oracle | ✅ |
| AC-11 | 送出的是最新價那一份 | 有開高低收、量、額、主動買入量；**沒有**標記／指數／溢價 | `LiveKCandleVo.ToDto` | ST:258 | shallow — presence asserted, absence not | produces-oracle (the update's candle has no such fields) | 🟠 |
| AC-12 | 跟盤不存走完的那一根 | 收到走完的樣子；跟盤沒有存任何一根 | S:224-228 (no store dependency) | ST (test bed has no store) | asserts-oracle (structural: nothing to store with) | produces-oracle | ✅ |
| AC-13 | 由每分鐘那一輪存入 | 以四份齊全存入 | 收取流程不變 | `contract_k_candle_ingestion_service_test.go:141` | asserts-oracle | produces-oracle | ✅ |
| AC-14 | 進行中那一根不存也不算 | 查不到、也不使用 | 同 AC-12；查詢／計算只讀儲存 | ST（結構） | asserts-oracle | produces-oracle | ✅ |
| AC-15 | 來源斷線 | 收到「即時更新已停止」 | `kCandleFollowFeed.keep` :96 | ST:284 | asserts-oracle | produces-oracle | ✅ |
| AC-16 | 重新跟上給現在的樣子 | 收到現在的樣子，不補播 | feed | ST:284 | asserts-oracle | produces-oracle | ✅ |
| AC-17 | 合約即時跟盤不能用時其他照常 | 其他合約功能照常 | 獨立 service／路由 | — | no-test | produces-oracle (nothing else depends on it) | 🟡 |
| AC-18 | 合約永不休市、沒有名額 | 不會收到休市中或名額已滿 | S（無名單分支） | ST:328 | asserts-oracle | produces-oracle | ✅ |
| BR-01 | 跟盤單位是合約標的，連線斷即離開 | 同 AC-01..04 | S | ST:91,116 | asserts-oracle | produces-oracle | ✅ |
| BR-02 | 只跟合約追蹤名單上的 | 同 AC-06..08 | S | ST:184 | asserts-oracle | produces-oracle | ✅ |
| BR-03 | 一分鐘一根、十秒、先給進行中 | — | 來源／`pass`／join | ST:138,238 | asserts-oracle | produces-oracle | ✅ |
| BR-04 | 內容不含標記價格等 | 同 AC-11 | — | ST:258 | shallow | produces-oracle | 🟠 |
| BR-05 | 不存、不參與計算 | 同 AC-12/14 | S | ST | asserts-oracle | produces-oracle | ✅ |
| BR-06 | 中斷：告知、逐次拉長最長三十秒、不放棄、恢復給現在 | — | feed + `LiveChannelHealthDomain` | ST:284,308, `live_channel_health_domain_test.go:60` | asserts-oracle | produces-oracle | ✅ |
| BR-07 | 現貨與合約分開，現貨一字不變 | — | 兩條線 | ST:157 + 既有現貨跟盤測試全綠 | asserts-oracle | produces-oracle | ✅ |
| EC-01 | 看到一半被移出名單，已在看的照常直到離開 | 已在看的人繼續收到 | S:85（只在開始時檢查） | — | no-test | produces-oracle | 🟡 |
| EC-02 | 服務關閉 → 全部結束、之後的被告知暫時無法跟 | 更新關閉；之後的拒絕 | S:151, C default 503 | ST:342, CT:95 | asserts-oracle | produces-oracle | ✅ |
| NFR-01 | 同一合約標的只開一條來源連線 | 一條 | S:117-131 | ST:91 | shallow（同 AC-02） | produces-oracle | 🟠 |
| NFR-02 | 現貨即時跟盤、合約收取與查詢一字不變 | — | feed 為純搬移 | 既有測試全綠 | asserts-oracle | produces-oracle | ✅ |
| NFR-03 | 不需登入 | 與現貨相同 | 路由未加 `requiresSignIn` | routes test | asserts-oracle | produces-oracle | ✅ |

## Orphans

| Behaviour | Where | Classification |
|---|---|---|
| A candle naming a contract the line does not carry is dropped | `kCandleFollowFeed.consume` | Inherited spot behaviour, pinned by ST:384; not an out-of-scope item |
| A blank or malformed contract code is refused as a bad request | S:88-91 → C | Undocumented in the PRD; harmless (mirrors every other contract endpoint) |

## Summary

✅ 23 conforms · 🔴 1 violation · 🟠 4 mis-asserted · 🟡 2 partial · ❌ 0 gaps · ❔ 0 unclear · ⚠️ 2 orphans — conformance 23 / 30 = 76.7%.

## Follow-up — resolved after the audit

| ID | Resolution |
|---|---|
| AC-07 | Unknown contract now says 「找不到這個合約標的」 in the refusal, still counted as not registered (404); asserted in ST and CT. → conforms |
| AC-02, NFR-01 | ST:91 now asserts no second line is opened for the second viewer. → conforms |
| AC-11, BR-04 | CT asserts the streamed event carries no mark, index or premium figures. → conforms |
| EC-01 | New test: a contract taken off the watchlist mid-watch keeps its viewer's picture while a newcomer is refused. → conforms |
| AC-17 | Left partial by design: independence is structural (separate service, route and source); no test can meaningfully probe "every other feature" from this slice. |
| Orphans | Blank-code refusal added to PRD edge cases; dropped foreign candle recorded as inherited behaviour. |

| AC-01 (code review) | A new follow's throttle was seeded with the current time, so the first forming candle of a quiet contract was held back for a whole ceiling (hidden by a 1 ns test ceiling). The contract follow now starts its throttle with nothing sent; pinned with a real 10 s ceiling, which also replaces the earlier throttle test (ST:238) that had encoded the held-back first candle; AC-09/AC-10 now trace to the new test. → conforms |

After follow-up: 29 / 30 conform; AC-17 partial (structural).
