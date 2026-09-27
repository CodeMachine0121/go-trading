# Contract Traceability Matrix — 交易日誌軟刪除

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/infrastructure/persistence/{contract,spot}_trade_record_repository.go`, `internal/infrastructure/persistence/schema_migrator.go`, `internal/domain/service/{contract,spot}_trade_journal_service.go`, `internal/domain/models/entities/{contract,spot}_trade_record.go`
Oracle: Acceptance Criteria (25 clauses) + Business Rules (6) + NFR (3) = 34 clauses

> Static conformance audit. The verdicts compare test assertions and code paths with the outcomes the spec expects. The audit does not run scenarios it made up. The first pass found two 🟠 clauses: AC-9 had no test for pre-existing trades, and AC-10–AC-14 (every change to a deleted trade) were asserted only for adding a note. Tests for both were added in the same branch (commit `test(trade-journal): pin that deleted trades refuse every change and older trades stay visible`). The table below shows the state after those tests were added.

Path shorthands: `CR` = `internal/infrastructure/persistence/contract_trade_record_repository.go`, `SR` = `…/spot_trade_record_repository.go`, `SM` = `…/schema_migrator.go`, `CS`/`SS` = `internal/domain/service/{contract,spot}_trade_journal_service.go`, `CRT` = `…/persistence/tests/trade_journal_repositories_test.go`, `SRT` = `…/persistence/tests/spot_trade_record_repository_test.go`, `CAT`/`SAT` = `internal/application/tests/{contract,spot}_trade_journal_application_test.go`.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 刪除自己的交易後它仍保存並記下刪除時間 | 刪除成功；#12 與其開倉平倉紀錄、附註仍在；標為已刪除並記下刪除時間 | CS:220, CR:261 | CAT:502 (MarkDeleted 帶當下時間), CRT:196 (列仍在、is_deleted、deleted_at＝給定時間、fill 1、note 1) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 刪除自己的現貨交易同樣只是標為已刪除 | 刪除成功；#20 與買賣紀錄仍在，標為已刪除 | SS:198, SR:263 | SAT:415, SRT:169 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 列表不列出已刪除交易，總筆數也不計入 | 只列 #13；總數 1 | CR:170 (`notDeleted`) | CRT:224 (page＝kept、totalCount 1)；SRT:197 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 只剩已刪除交易時列表是空的 | 空列表、總數 0 | CR:170 | CRT:266 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 依狀態篩選時也不列出已刪除交易 | 持倉中篩選不含 #12 | CR:170 | CRT:224 (openPage 空、openCount 0)；SRT:197 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 查看已刪除交易回覆找不到 | 「找不到」這筆交易 | CR:313 `withChildren`→`notDeleted`；CS:393 | CRT:224 (FindOne→ErrContractTradeNotFound)；CAT:430 (GetTrade→「找不到這筆交易」) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 再刪一次已刪除交易回覆找不到 | 「找不到」這筆交易 | CS:216 findOwnedTrade；CR:270 RowsAffected==0 | CAT:430 (DeleteTrade)；CAT:510、CRT:196 (第二次 MarkDeleted→NotFound) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 刪除別人的交易回覆找不到且不影響它 | 「找不到」；#27 仍未刪除 | CS:393 擁有者比對 | CAT:521 (NotFound；嚴格 mock 未預期 MarkDeleted＝未被動到)；SAT 既有 strangers case | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 改動上線前記下的合約交易照常出現 | #5 照常列出 | entity `IsDeleted default:false`；AutoMigrate | CRT:546 (先移除欄位插入舊列→遷移→合約與現貨各列出 1 筆) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 為已刪除的持倉中交易加平倉紀錄回覆找不到 | 「找不到」；開倉平倉紀錄不變 | CS:393 → CR FindOne；CR:80 Save 守門 | CAT:430 (AddFill；未預期 Save)；CRT:297 (Save 已刪除→NotFound、子紀錄未寫入) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 修正或刪除已刪除交易的一筆開倉紀錄回覆找不到 | 「找不到」 | 同上 | CAT:430 (AmendFill、RemoveFill) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 為已刪除的已平倉交易寫檢討回覆找不到 | 「找不到」 | 同上 | CAT:430 (WriteReview) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 為已刪除交易加附註、改計畫或改型態標籤回覆找不到 | 「找不到」 | 同上 | CAT:430 (AddNote、AmendPlan、AssignSetupTags) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 為已刪除的現貨交易加一筆賣出回覆找不到 | 「找不到」 | SS:382 → SR FindOne；SR:80 | SAT:365 (AddFill 等全部)；SRT:255 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 未刪除的交易照舊可以修改 | 照舊記下平倉紀錄 | CR:61 Save（未刪除列 RowsAffected=1） | 既有 CAT AddFill/AddNote 成功案例；CRT:71 Save 既有測試 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 績效統計不計入已刪除交易 | 只計 #13 | CR FindClosedByOwner（`withChildren`→`notDeleted`） | CRT:224 (allClosed 只剩 kept) | asserts-oracle（在唯一改變的邊界：取已平倉交易） | produces-oracle | ✅ conforms |
| AC-17 | 現貨統計不計入已刪除交易 | 只計 #21 | SR FindClosedByOwner | SRT:197 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 實盤 vs 回測不拿已刪除交易對照 | 只拿 #13 | CR FindClosedByOwnerAndTradingStrategy | CRT:224 (strategyClosed 只剩 kept)；SRT:197 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 策略的已平倉交易全被刪除時等同沒有實單 | 呈現沒有已平倉實單、不重演 | 同上 → 既有「沒有已平倉實單不重演」 | CRT:224（刪除者不回傳）＋ `contract_trade_live_comparison_application_test.go:196` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 刪除持倉中的合約交易後可以重記同標的同方向 | 記下成功，成為新的持倉中交易 | SM:242 部分索引 `AND is_deleted = false`；CR FindOpenByOwnerSymbolDirection | CRT:281；CRT:526（舊索引升級） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 未刪除的持倉中交易照舊擋下同標的同方向 | 拒絕，說已有持倉中 #13 | SM:242；CR writeFailureOf | CRT:281 (第三筆→ErrContractTradeOpenPositionExists)；CRT:57 既有 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 刪除持有中的現貨交易後可以重記同標的 | 記下成功 | SM:254；SR FindOpenByOwnerSymbol | SRT:239；SRT:303 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 記到交易日誌連結不把已刪除交易當成已有持倉中 | 預填新增一筆，而非加開倉 | CS:350 → FindOpenByOwnerSymbolDirection（`notDeleted`） | CRT:224 (hasOpenTrade=false)＋CAT:770（無持倉中→預填新增） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 只貼在已刪除交易上的標籤可以刪除 | 刪除成功 | CR:278 / SR:280 CountByTag 經 `notDeleted` | CRT:316 / SRT:274 (只剩已刪除→0)＋ `trade_journal_setting_application_test.go` 兩邊 0 時可刪 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 仍貼在未刪除交易上的標籤照舊刪不掉，且只算未刪除的筆數 | 拒絕，說還有 1 筆 | 同上＋TradeJournalSettingService 加總 | CRT:316 / SRT:274 (一刪一留→1)＋ `trade_journal_setting_application_test.go:277-282`「還有 1 筆交易貼著它」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 已刪除等同不存在（對擁有者）：所有讀取路徑排除 | 列表/總數/查看/統計/對照/連結/標籤計數皆排除 | CR:305 / SR:307 `notDeleted` 為唯一讀取起點 | AC-3～AC-7、AC-16～AC-25 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 「找不到」一致：已刪除與別人的交易回同一句 | 同一句「找不到這筆交易」 | CS:393 / SS:382 | CAT:344 與 CAT:430 皆斷言同一訊息；SAT 同 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 持倉名額只算未刪除的交易 | 同 AC-20～AC-22 | SM:242、SM:254 | CRT:281、SRT:239 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 內容凍結：已刪除交易維持刪除當下的樣子 | 任何儲存都不改寫 | CR:76-82 / SR 同位 Save 守門；欄位清單不含刪除欄位 | CRT:297、SRT:255 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 既有資料視為未刪除 | 同 AC-9 | entity default | CRT:546 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 並發兩次刪除：只標記一次、刪除時間不被覆寫；儲存失敗時刪除不成立 | 第二次「找不到」、時間不變；失敗回暫時失敗 | CR:270 條件含未刪除 | CRT:196 (第二次 +1h→NotFound，deleted_at 仍是第一次)；controller 測試 MarkDeleted 失敗→502 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 對外回覆形狀不變 | 刪除仍 204、找不到同一錯誤、DTO 無新欄位 | entity `ToDto` 未輸出刪除欄位；controller 未改 | controller 測試（204／502 不變） | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 歸屬規則不變、不透露差異 | 同 BR-2 | CS:393 | CAT:344、CAT:430 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 列表、統計、對照回應時間不因保留已刪除交易而明顯變慢 | 無明顯變慢 | `is_deleted` 加索引；部分唯一索引 | — | no-test | unclear | ❔ unclear |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| SM:51-52 `retiredIndexes` 兩筆 | 升級時移除舊的持倉唯一索引 | 屬 BR-3 的遷移實作細節（ARCH 已載明），非新行為 |
| CR:76-82 / SR 同位 Save 回「找不到」 | 讀取後才被刪除的交易，儲存時拒絕 | 由 BR-4「內容凍結」涵蓋 |

Out-of-scope check: 單筆成交刪除仍是當場移除（未改）；沒有還原、沒有瀏覽已刪除的路徑；帳號刪除的連帶行為未改——無越界。

## Summary

- Conforms: 33/34 clauses ✅ (97%)
- Violations: —
- Mis-asserted: —（初審 AC-9～AC-14 為 🟠，已補測試）
- Partial: —
- Gaps: —
- Unclear: NFR-3（效能無量測；以索引緩解，個人規模資料量下不構成風險）
- Orphans: 2（皆由既有條款解釋，無越界）
