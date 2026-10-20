# 一台機器人盯多個交易標的 — Architecture Design

**Status:** Confirmed（autonomous run — decisions recorded below）
**Source PRD:** `.sdd/2026-10-20-multi-symbol-strategy-bot/PRD.md`
**Tech context:** Go · Gin · GORM · PostgreSQL · Clean/Onion（`internal/{controller,application,domain,infrastructure,job}`）

---

## 1. Design Goal & Guiding Principle

- **In one sentence:** 把機器人身上「屬於一個交易標的」的狀態（上一次送出的信號與那一輪、衝突記號、標的停擺原因、機器人持倉四欄）搬到一個新的子實體 `StrategyBotWatchedSymbol`（監聽標的），機器人保留整台共用的設定與執行狀態；一輪變成「讀一次交易策略 → 每個監聽標的同時各跑一次 → 所有標的的結果在**同一個交易**裡一次記下」。

- **Guiding principle — 監聽標的是機器人聚合的一部分，不是獨立的東西：**
  1. **一個聚合、一個 repository。** 監聽標的沒有脫離機器人的意義（比照交易策略與它的信號來源、條件樹由 `TradingStrategyRepository` 一起存）。`StrategyBotRepository` 讀機器人時一併帶出 `WatchedSymbols`（依 id 排序＝加入順序），`Save` 同步子列（留下的不動、新的插入、移除的刪掉），`UpdateRunState` 同時寫機器人與每個監聽標的的輪次狀態欄。不新增 `IStrategyBotWatchedSymbolRepository`。
  2. **狀態轉換住在 `StrategyBotWatchedSymbolDomain`。** 單一標的的「一輪結束／跳過／標的停擺／啟動清空／能不能移除」全部在它身上；`StrategyBotRunStateDomain` 只管整台並把每個標的委給它。
  3. **一輪一次寫。** `StrategyBotService.RecordRound` 鎖住機器人那一列，套用整輪結果，對每個參與的標的依序 `Append` 執行紀錄（輪次編號整台共用一串、只增不減）、排輪次訊息、排自動下單委託，再排標的停擺通知或整台停擺通知——全在同一個交易，所以不會有一半標的記了一半沒記。
  4. **輪次編號整台唯一** → 交易紀錄的「來源機器人＋輪次編號」、自動下單委託的 `FindByBotRunNumbers`、記到交易日誌連結都**不必加標的維度**。只有以「這一輪」辨認的唯一索引（待送訊息、自動下單委託）需要加 `symbol`。

---

## 2. Change Scope

| Area | Action | What / Why |
| :--- | :--- | :--- |
| `entities.StrategyBotWatchedSymbol` | **Add** | 表 `StrategyBotWatchedSymbols`：`StrategyBotID`+`Symbol` 唯一（`idx_strategy_bot_watched_symbols_bot_symbol`）、`LastSentSignal`、`LastSentRoundDueAt`、`Conflicting`、`HaltReason`、`AutoOrderPositionDirection`、`AutoOrderPositionQuantity`、`AutoOrderStopLossClientID`、`AutoOrderTakeProfitClientID`、`CreatedAt`。隨機器人 cascade 刪除。`ToDto(marketDataKind)`、`ToAutoOrderPositionVo()` |
| `entities.StrategyBot` | **Modify** | 移除 `Symbol`、`LastSentSignal`、`LastSentRoundDueAt`、`Conflicting`、`AutoOrderPosition*` 四欄；加 `WatchedSymbols []StrategyBotWatchedSymbol`（`OnDelete:CASCADE`）。`HaltReason` 留著＝整台停擺。`ToDto()` 帶出 `WatchedSymbols`；`WatchedSymbol(symbol)` 查找 |
| `entities.StrategyBotRunRecord` | **Modify** | 加 `Symbol`（`size:64;not null;default:''`）＋索引 `(strategy_bot_id, symbol, run_number desc)`；`ToDto` 帶出；`ToJournalLinkRoundDto` 改用自己的 `Symbol` |
| `entities.PendingMessage` | **Modify** | 加 `Symbol`（`default:''`）；唯一索引改為 `idx_pending_messages_bot_symbol_round (strategy_bot_id, symbol, round_due_at)` |
| `entities.ContractAutoOrder` | **Modify** | 唯一索引改為 `idx_contract_auto_orders_bot_symbol_round (strategy_bot_id, symbol, round_due_at)` |
| `dto.StrategyBotWatchedSymbolDto` | **Add** | `symbol`、`lastSentSignal`、`conflicting`、`haltReason`、`autoOrderPosition`（合約才有） |
| `dto.StrategyBotDto` | **Modify** | 以 `WatchedSymbols []StrategyBotWatchedSymbolDto` 取代 `symbol`、`lastSentSignal`、`conflicting`、`autoOrderPosition`；`haltReason` 留著＝整台 |
| `dto.StrategyBotWriteDto` | **Modify** | `Symbol` → `Symbols []string` |
| `dto.StrategyBotRoundOutcomeDto` | **Modify** | 變成整輪：`WatchedSymbolOutcomes []StrategyBotWatchedSymbolRoundOutcomeDto` |
| `dto.StrategyBotWatchedSymbolRoundOutcomeDto` | **Add** | 一個標的這一輪的結果（原 `StrategyBotRoundOutcomeDto` 的欄位＋`Symbol`）；`Kind` 為 `skipped` / `halted`（整台）/ `watchedSymbolHalted` / `concluded` |
| `dto.StrategyBotRunRecordDto` / `StrategyBotRunRecordWriteDto` | **Modify** | 加 `Symbol` |
| `domains.StrategyBotDomain` | **Modify** | `symbol` → `symbols []string`：1–10 個、各自 `NewTradingSymbolDomain`、正規化後不得重複；`Symbols()`；`ToEntity()` 帶 `WatchedSymbols`。常數 `StrategyBotWatchedSymbolLimit = 10` |
| `domains.StrategyBotWatchedSymbolDomain` | **Add** | 單一標的的狀態轉換與規則：`IsActive`、`RoundFinished`、`RoundSkipped`、`Halt`、`ClearedForStart`、`RequireRemovable(hasUnsettledAutoOrder)` |
| `domains.StrategyBotRunStateDomain` | **Modify** | `Start` 清整台與每個標的；`Halt` 只動整台；新增 `ActiveWatchedSymbols()`、`AllWatchedSymbolsHalted()`；`RoundFinished/RoundSkipped` 改為只排下次時間（標的的部分交給標的 domain） |
| `domains.StrategyBotRoundOutcomeDomain` | **Modify** | 整輪：`ApplyTo(runState, dueAt, now) (entities.StrategyBot, vo.StrategyBotHaltReasonVo)`——逐標的套用、再決定整台停擺（任一標的帶整台原因 → 停擺；否則全部標的停擺 → 以最後一個標的停擺原因停擺）；`WatchedSymbolOutcomes()` 每個標的的 `RecordedResult`、`HaltedWatchedSymbols()` |
| `domains.StrategyBotRoundFailureDomain` | **Modify** | 原因分兩種範圍：策略腳本找不到、交易策略找不到 → 整台（`halted`）；算不出來、訂單流超過範圍 → 標的（`watchedSymbolHalted`）；其餘 → 跳過 |
| `domains.StrategyBotLifecycleMessageDomain` | **Modify** | 收一串標的：開頭標出全部標的（合約加一次「永續合約」），一檔／N 檔措辭；新增 `WatchedSymbolHaltedText()`（開頭【標的停擺】） |
| `domains.StrategyBotMarketDomain` | **Modify** | 加 `SymbolsLabel([]string)` |
| `domains.PendingMessageDomain` | **Modify** | `NewRoundPendingMessageDomain` 收 `symbol`；`Symbol()` |
| `domains.ContractAutoOrderDomain` | **Modify** | `NewQueuedContractAutoOrderDomain(bot, symbol, …)` |
| `domains.strategy_bot_errors.go` | **Modify** | 新哨兵 `ErrStrategyBotWatchedSymbolHoldsPosition`、`ErrStrategyBotWatchedSymbolAutoOrderOutstanding` |
| `IStrategyBotRepository` / `StrategyBotRepository` | **Modify** | 讀取一律 preload `WatchedSymbols`（依 id）；`Save` 同步子列（交易內）；`UpdateRunState` 寫整台＋各標的輪次欄；`UpdateAutoOrderPosition(ctx, botID, symbol, position)`；`ForgetSentSignal(ctx, botID, symbol, roundDueAt)` |
| `IStrategyBotRunRecordRepository` / 實作 | **Modify** | `Append` 依 `(bot, symbol)` 修剪到 50 筆；`FindLatestByBot(ctx, botID, symbol)`（空字串＝全部，上限 50×10 筆） |
| `IPendingMessageRepository` / 實作 | **Modify** | `Enqueue` 的 on-conflict 欄位加 `symbol` |
| `IContractAutoOrderRepository` / 實作 | **Modify** | `Enqueue` on-conflict 加 `symbol`；`FindDispatchCandidates` 每 `(bot, symbol)` 取頭；`ExistsUnsettledAfter(ctx, botID, symbol, id)`；新增 `ExistsUnsettled(ctx, botID, symbol)` |
| `service.StrategyBotService` | **Modify** | `CreateStrategyBot` / `UpdateStrategyBot` 每個標的各做合約檢查；`UpdateStrategyBot` 在交易內鎖機器人、檢查被移除標的可否移除再存；`DecideRound(watchedSymbolDto, …)`；`RecordRound` 整輪寫入；`ListRunRecords(…, symbol)`；生命週期訊息用全部標的 |
| `service.ContractAutoOrderService` | **Modify** | 持倉讀寫改以委託的標的找監聽標的 |
| `service.PendingMessageService` | **Modify** | 作廢時 `ForgetSentSignal(bot, symbol, roundDueAt)` |
| `application.StrategyBotRunApplication` | **Modify** | `playRound` 讀一次交易策略，再對每個 active 標的並行 `playWatchedSymbolRound` |
| `application.StrategyBotApplication` | **Modify** | `ListRunRecords(…, symbol)` |
| `controller.StrategyBotController` + `models.StrategyBotRequest` | **Modify** | `symbols []string`；`GET /strategy-bots/:id/runs?symbol=`；兩個新 409 reason |
| `persistence.SchemaMigrator` | **Modify** | 註冊新實體；`moveBotSymbolsOntoWatchedSymbols`（見 §6）；舊唯一索引列入 `retiredIndexes` |
| `postman/` | **Modify** | 建立／修改改用 `symbols`；runs 加 `symbol` 篩選 |
| 交易紀錄（`SourceStrategyBotID` + `SourceRunNumber`） | **Not touched** | 輪次編號整台唯一，已能唯一指到那個標的的那一筆 |
| `StrategyBotRunningLimit`（10 台） | **Not touched** | PRD 範圍外 |
| 部位規劃、`PositionPlanDomain` | **Not touched** | 每個標的照整份規劃各自算，原算法直接適用 |
| 輪次認領（`RoundClaimedBy/Until`） | **Not touched** | 認領的單位仍是整台機器人 |

---

## 3. New Classes / Modules

| Name | Kind | Responsibility (purpose) | Collaborators | Satisfies (PRD scenario) |
| :--- | :--- | :--- | :--- | :--- |
| `entities.StrategyBotWatchedSymbol` | Entity | 一個監聽標的與它自己記著的狀態 | `StrategyBot`（父） | US-01 全部、US-06 全部、US-07 |
| `domains.StrategyBotWatchedSymbolDomain` | Domain Model | 一個標的的輪次狀態轉換（結束、跳過、停擺、啟動清空）與可否移除 | `entities.StrategyBotWatchedSymbol` | US-02 移除三則、US-03 判斷與衝突、US-05 停擺與重啟 |
| `dto.StrategyBotWatchedSymbolDto` | DTO | 對外呈現一個監聽標的的狀態 | — | US-07 |
| `dto.StrategyBotWatchedSymbolRoundOutcomeDto` | DTO | 一個標的這一輪的結果，在 application 與 domain 之間傳遞 | — | US-03、US-04、US-05 |

> 刻意**不**新增 watched symbol 的 repository／service：它的生命週期完全跟著機器人，拆開只會讓「存一台機器人」變成呼叫端要排順序的兩件事。

---

## 4. Modified Components

| Component | Current role | Change needed |
| :--- | :--- | :--- |
| `StrategyBotRunApplication.playRound` | 讀交易策略 → 判斷 → 組訊息 | 讀交易策略一次；失敗 → 每個 active 標的同一個整台失敗結果；成功 → 每個 active 標的一個 goroutine 跑 `playWatchedSymbolRound`（原本的單標的流程，`botDto.Symbol` 換成標的），結果依標的順序收集 |
| `StrategyBotService.RecordRound` | 一個標的、一筆紀錄 | 鎖機器人＋標的 → token 檢查 → `outcome.ApplyTo` → `UpdateRunState` → 依標的順序 `Append` 紀錄、排訊息、排委託 → 整台停擺則排整台通知，否則排各標的停擺通知 |
| `StrategyBotService.UpdateStrategyBot` | 停止中的機器人整份改寫 | 交易內 `FindOneLocked` → 算出被移除的標的 → 每一個 `RequireRemovable(ExistsUnsettled)` → `Save` |
| `StrategyBotRepository.Save` | 建立或改寫機器人欄位 | 同一交易內同步 `WatchedSymbols`：建立時一併插入；改寫時刪掉不在清單裡的、插入新的（`OnConflict DoNothing`），留下的不動 |
| `ContractAutoOrderRepository.FindDispatchCandidates` | 每台機器人取最舊一筆 | 每 `(機器人, 標的)` 取最舊一筆——不同標的的委託不互相等待 |
| `StrategyBotRunRecordRepository.Append` | 每台保留 50 筆 | 每 `(機器人, 標的)` 保留 50 筆：取該標的第 50 新的輪次編號，刪掉更舊的 |

---

## 5. Component Relationships

```mermaid
flowchart TD
    Job[StrategyBotScanJob] --> RunApp[StrategyBotRunApplication]
    RunApp -->|一次| TS[TradingStrategyService]
    RunApp -->|每個 active 監聽標的並行| Play[playWatchedSymbolRound]
    Play --> Calc[Indicator/ContractIndicator CalculationService]
    Play --> BotSvc[StrategyBotService.DecideRound / PlanRoundPosition / PlanAutoOrderIntent]
    RunApp -->|整輪結果| Record[StrategyBotService.RecordRound]
    Record --> Outcome[StrategyBotRoundOutcomeDomain]
    Outcome --> RunState[StrategyBotRunStateDomain]
    RunState --> WS[StrategyBotWatchedSymbolDomain]
    Record --> BotRepo[(StrategyBotRepository: bot + watched symbols)]
    Record --> RunRepo[(StrategyBotRunRecordRepository)]
    Record --> PMRepo[(PendingMessageRepository)]
    Record --> AORepo[(ContractAutoOrderRepository)]
    AOSvc[ContractAutoOrderService] --> AORepo
    AOSvc -->|UpdateAutoOrderPosition(bot, symbol)| BotRepo
```

---

## 6. Migration（正式資料、執行中的機器人）

順序（`SchemaMigrator.Migrate`）：
1. `AutoMigrate`：建 `StrategyBotWatchedSymbols`；`StrategyBotRunRecords`、`PendingMessages` 加 `symbol`（`default ''`，既有列為空）；建新唯一索引（既有資料一定滿足：舊的 `(bot, round)` 唯一已保證 `(bot, '', round)` 唯一；委託本來就有 `symbol`）。
2. **`moveBotSymbolsOntoWatchedSymbols`（新）**——只在 `StrategyBots` 還有 `symbol` 欄時做，**一個交易**：
   - 以 `retiredStrategyBotSymbolRow`（`TableName() = "StrategyBots"`，只讀 id 與九個舊欄，明列 Select）讀出每一台；
   - 對每一台插入一個監聽標的，帶過 `Symbol`、`LastSentSignal`、`LastSentRoundDueAt`、`Conflicting`、持倉四欄（`OnConflict DoNothing` on `(strategy_bot_id, symbol)`，重跑不覆蓋）；
   - 該台 `symbol = ''` 的執行紀錄與待送訊息（有輪次的）補上那個標的；
   - 在同一交易裡 `DropColumn` 這九個舊欄。
   交易失敗整個回滾，舊欄與資料原封不動，下次啟動重跑。成功後舊欄不在，條件不再成立，之後不再做任何事。
3. `dropRetiredIndexes`：加入 `idx_pending_messages_bot_round`、`idx_contract_auto_orders_bot_round`（新索引已建好才丟舊的，期間沒有無索引的空窗）。
4. 測試：從舊結構（手動建出帶舊欄的 `StrategyBots`、舊索引）出發，放一台執行中、有未平倉機器人持倉與止損止盈單的合約機器人、一筆執行紀錄、一則待送輪次訊息；跑 `Migrate` 兩次；驗證監聽標的完整帶過、紀錄與訊息補上標的、舊欄與舊索引消失、第二次不改任何東西。

---

## 7. Extensibility & Handoff Notes

- **Most likely next requirement:** 同時持倉數量上限（「最多同時 N 個標的有倉」），其次是個別標的覆寫部位規劃。
- **Where it lands:** 同時持倉上限落在 `RecordRound` 排委託前——此時機器人與所有監聽標的都已鎖住讀進來，`StrategyBotWatchedSymbolDomain` 的持倉方向就是答案；加一個整台規則（例如 `StrategyBotRunStateDomain.AdmitsNewPosition(symbol)`）即可，不必改一輪的流程。個別覆寫則在 `StrategyBotWatchedSymbol` 加欄位、在 `composeRoundMessage` 選用 `watchedSymbol` 的規劃取代 `botDto.PositionPlan`。
- **How to add it:** 只動 domain 規則與一個判斷點；資料已在聚合裡。
- **Patterns applied & why:** 聚合（機器人＋監聽標的同一個 repository、同一個鎖），因為兩者的一致性（「全部標的停擺 → 整台停擺」、「一輪一次寫」）必須在同一個交易內成立。
- **Do not hardcode:** 標的上限 `StrategyBotWatchedSymbolLimit`、每標的保留輪數 `strategyBotRememberedRunCount`。
- **Known debt / deferred:** 移除標的時不清它的舊執行紀錄（每個標的至多 50 筆，屬有界的殘留）。

---

## 8. Traceability

| PRD Scenario | Fulfilled by |
| :--- | :--- |
| US-01 建立盯兩個標的的現貨機器人 / 只盯一個 / 正好 10 個 / 超過 10 個 / 沒有任何標的 / 大小寫重複 | `StrategyBotDomain`（`NewStrategyBotDomain` 標的清單規則）+ `StrategyBotRepository.Save` |
| US-01 合約標的不在追蹤名單 / 槓桿超過其中一個標的 | `StrategyBotService.settle` 對每個標的 `ContractStrategyBotMarketDomain.Admit` |
| US-02 增加一個標的保留狀態 / 移除空手標的 / 移除後再加回從空白 | `StrategyBotRepository.Save`（同步子列，留下的不動、移除的刪掉） |
| US-02 移除有持倉 / 有沒做完委託 | `StrategyBotWatchedSymbolDomain.RequireRemovable` + `ContractAutoOrderRepository.ExistsUnsettled`，在 `UpdateStrategyBot` 交易內 |
| US-02 執行中不能修改 | `StrategyBotRunStateDomain.RequireEditable`（不變） |
| US-03 只有信號變了的標的送 / 兩個都送 / 衝突不影響另一個 | `StrategyBotRunApplication.playWatchedSymbolRound` + `StrategyBotService.DecideRound(watchedSymbol)` + `StrategyBotWatchedSymbolDomain.RoundFinished` |
| US-03 合約標的行情停了只跳過那一個 | `playWatchedSymbolRound` 的 `RequireCurrentMarket` → `watchedSymbol` outcome `skipped` |
| US-03 每個標的各記一筆執行紀錄 | `RecordRound` 逐標的 `Append` |
| US-03 只看某一個標的 | `StrategyBotRunRecordRepository.FindLatestByBot(…, symbol)` + controller `?symbol=` |
| US-03 每個標的各記得 50 輪 | `StrategyBotRunRecordRepository.Append` 依 `(bot, symbol)` 修剪 |
| US-04 每個標的各自用滿整份部位規劃 | `composeRoundMessage` 每個標的用 `botDto.PositionPlan`（不變的 `PositionPlanDomain`） |
| US-04 兩個標的各排一筆委託、不互相等待 | `RecordRound` 逐標的排委託 + `ContractAutoOrderRepository.FindDispatchCandidates`（每 `(bot, symbol)` 取頭） |
| US-04 別的標的不撤 / 同一個標的撤舊進場單 | `ContractAutoOrderRepository.ExistsUnsettledAfter(bot, symbol, id)` |
| US-04 平倉只平那個標的的持倉 | `ContractAutoOrderService.prepare`／`recordStep` 以委託標的讀寫 `StrategyBotWatchedSymbol` 持倉 |
| US-05 一個標的算不出來只停那一個（含通知） | `StrategyBotRoundFailureDomain`（標的範圍）+ `StrategyBotRoundOutcomeDomain.ApplyTo` + `StrategyBotLifecycleMessageDomain.WatchedSymbolHaltedText` |
| US-05 只盯一個標的時與今天相同 / 最後一個也停擺 | `StrategyBotRoundOutcomeDomain.ApplyTo`（全部標的停擺 → 整台停擺）+ `RecordRound` 只排整台通知 |
| US-05 交易策略找不到整台停擺 | `playRound` 交易策略讀取失敗 → 整台範圍 |
| US-05 Telegram 拒絕金鑰整台停擺 | `PendingMessageService`（不變，`Halt` 只動整台） |
| US-05 重新啟動恢復 | `StrategyBotRunStateDomain.Start` → `StrategyBotWatchedSymbolDomain.ClearedForStart` |
| US-06 完整帶過所有狀態 / 重複升級不變 / 舊紀錄歸給原標的 | `SchemaMigrator.moveBotSymbolsOntoWatchedSymbols` |
| US-07 查機器人看到每個標的的狀態 | `entities.StrategyBot.ToDto` → `StrategyBotWatchedSymbolDto` |
| US-07 刪除時任一標的有等成交的進場單就拒絕 | `ContractAutoOrderRepository.ExistsOutstandingEntry(bot)`（不變） |

---

## 9. Risks & Open Decisions

- **Risks / trade-offs:**
  - 一台盯 10 個標的時，一輪 10 個標的並行搶算式隔間；機器人輪次本來就優先，時限沿用一輪的時限——排不到的標的該輪以「算式隔間忙碌中」跳過（記持有），與今天單標的相同。
  - `UpdateStrategyBot` 的可否移除檢查與自動下單執行之間沒有共用鎖（執行不鎖機器人列）；但機器人已停止時不會排新的委託，而正在執行的委託在檢查時仍是「沒做完」，因此實務上不會漏擋。
  - 升級在一個交易內完成搬移與丟欄；資料量為「機器人台數」級，交易很短。
- **Open decisions (for implementation):** 無。
