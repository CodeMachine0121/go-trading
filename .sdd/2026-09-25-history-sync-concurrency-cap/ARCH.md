# 歷史同步同時進行上限 — Architecture Design

**Status:** Confirmed
**Source PRD:** `.sdd/2026-09-25-history-sync-concurrency-cap/PRD.md`
**Tech context:** Go · Gin · GORM（PostgreSQL）· Clean / Onion Architecture（`.claude/rules/`）

---

## 1. Design Goal & Guiding Principle

- **In one sentence:** `KCandleIngestionService.StartHistorySyncFor` 與 `ContractKCandleIngestionService.StartHistorySyncFor` 在記下 `running` 輪次之前，先向各自的 run repository 數一次 `running` 列，交給 `KCandleHistorySyncCapacityDomain` 判斷；滿了回 `ErrKCandleHistorySyncCapacityReached`（controller 對映 `429`），不寫任何列、不啟動 goroutine。
- **Guiding principle:** **「幾趟算滿」只住在一個 Domain Model。** 兩個 service 只負責「數、問、記」，並把這三步圈在同一把 service 鎖裡。下一個要改上限規則的人（例如依市場分別計、或讓某種同步不佔名額）只改 `KCandleHistorySyncCapacityDomain` 與它收到的數字，不碰 runner 或 controller。

---

## 2. Change Scope

| Area | Action | What / Why |
| :--- | :--- | :--- |
| `domain/models/domains` | **Add** | `KCandleHistorySyncCapacityDomain` + 哨兵 `ErrKCandleHistorySyncCapacityReached` |
| `IKCandleHistorySyncRunRepository` / `IKCandleContractHistorySyncRunRepository` + 實作 | **Modify** | 各加 `CountRunning`：資料庫是進行中趟數的唯一真相 |
| `KCandleIngestionService` / `ContractKCandleIngestionService` | **Modify** | 建構子多收同時進行上限；加 `historySyncStartMutex`；`StartHistorySyncFor` 在同一把鎖裡「數 → 判斷 → 記下」 |
| `KCandleHistorySyncController` / `KCandleContractHistorySyncController` | **Modify** | `ErrKCandleHistorySyncCapacityReached` → `429` |
| `config.IngestionConfig` / `ContractIngestionConfig` | **Modify** | `HistorySyncMaxConcurrentSyncs`（`KCANDLE_HISTORY_SYNC_MAX_CONCURRENT_SYNCS`、`CONTRACT_KCANDLE_HISTORY_SYNC_MAX_CONCURRENT_SYNCS`，預設各 `2`） |
| `cmd/server/dependencies.go` | **Modify** | 只在兩個 service 建構子呼叫多傳一個參數；**路由不動**（另一條分支在改路由的登入保護） |
| `.env.example`、README、Postman、`go-trading-deploy` configmap | **Modify** | 新設定與 `429` 拒絕案例 |
| 標的登錄（`TradingSymbolService.AddToWatchlist` 等） | **Not touched** | 歷史同步從不登錄標的，`reachSymbolOnDemand` 遇到沒登錄的代號回 `ErrTradingSymbolNotRegistered`；登錄只經 `AddToWatchlist`（先 `LookUpSymbol` 確認上市）或啟動預設名單。已驗證，不需收緊 |
| 一趟一標的唯一索引、重啟收尾、runner | **Not touched** | 既有規則照舊；上限是額外的一道 |
| Schema | **Not touched** | 不加欄位、不加索引；`CountRunning` 走既有 `status` 欄位 |

---

## 3. New Classes / Modules

| Name | Kind | Responsibility (purpose) | Collaborators | Satisfies (PRD scenario) |
| :--- | :--- | :--- | :--- | :--- |
| `KCandleHistorySyncCapacityDomain` | Domain Model | 以同時進行上限判斷「目前 `running` 幾趟時還能不能再開一趟」。建構子把 `< 1` 的上限正規化成 `1`（至少放得進一趟，才不會因設定錯誤讓功能整個死掉；config 端已先把不成立的值換成預設）。`Admit(runningCount) error`：`runningCount >= 上限` 回包著 `ErrKCandleHistorySyncCapacityReached`、寫出上限的訊息 | — | US-01、US-02、US-04 |
| `ErrKCandleHistorySyncCapacityReached` | 哨兵錯誤 | 「整體太忙、稍後再試」；與 `ErrKCandleHistorySyncInProgress`（有人按了兩次）分開，controller 才分得出 `429` 與 `409` | — | US-02、US-03、US-05 |

> 深度檢查：service 端一次呼叫 `Admit`、一個錯誤分支；「怎樣算滿」與訊息全部藏在 domain 裡。

---

## 4. Modified Components

| Component | Current role | Change needed |
| :--- | :--- | :--- |
| `I…HistorySyncRunRepository.CountRunning` | — | `Model(&entity).Where(status = running).Count(&count)`，錯誤包成 `count running … history sync runs: %w` |
| `KCandleIngestionService.StartHistorySyncFor` | 驗證 → 切段 → `Save` → `go runner` | 驗證與切段不變；`Save` 換成 `recordRunningHistorySync(ctx, run)`：拿 `historySyncStartMutex` → `CountRunning` → `capacity.Admit` → `Save` → `defer` 還鎖。這是只被一個 public method 用的 private method，但屬於「圈住資源持有範圍」的例外：內聯會在三個 `return` 前手動還鎖 |
| `ContractKCandleIngestionService.StartHistorySyncFor` | 同上 | 同上 |
| 兩個 controller 的 `StartSymbolHistorySync` | 400/404/409/502 分流 | 在 409 之前加 `ErrKCandleHistorySyncCapacityReached → 429` |

---

## 5. Component Relationships

```mermaid
flowchart TD
    Controller[KCandleHistorySyncController / KCandleContractHistorySyncController] --> Application[KCandle(Contract)IngestionApplication]
    Application --> Service[KCandleIngestionService / ContractKCandleIngestionService]
    Service -->|lookback, symbol first| Lookback[KCandleHistoryLookbackDomain]
    Service -->|under historySyncStartMutex| Repository[(I…HistorySyncRunRepository: CountRunning, Save)]
    Service --> Capacity[KCandleHistorySyncCapacityDomain.Admit]
    Service -->|only when admitted| Runner[history sync runner goroutine]
```

---

## 6. Extensibility & Handoff Notes

- **Most likely next requirement:** 多副本部署，或依市場（加密貨幣現貨 vs 台股）分別設上限。
- **Where it lands:**
  - 多副本：`recordRunningHistorySync` 的鎖只封得住同一個 process。屆時把「數 + 記」改成資料庫內一次決定（例如 `SELECT … FOR UPDATE` 於一張名額列、或 advisory lock），介面仍是 `Admit` 前後那一段，service 的呼叫端不變。
  - 依市場：`CountRunning` 加一個市場條件、`KCandleHistorySyncCapacityDomain` 收一組上限；runner 與 controller 不動。
- **Patterns applied & why:** Domain Model 承載規則（rules 要求行為住在 domain model）；service 鎖是資源範圍，不是業務規則。
- **Do not hardcode:** 上限一律來自設定。
- **Known debt / deferred:** 同一標的已在跑且上限也滿時，回 `429` 而非 `409`（數在寫之前）。兩者都叫人等，且那一趟本來就算在滿的裡面，所以接受。

---

## 7. Traceability

| PRD Scenario | Fulfilled by |
| :--- | :--- |
| 沒有任何進行中的同步時照常開跑 / 最後一個空位照常開跑 | `KCandleHistorySyncCapacityDomain.Admit`（`count < limit`）+ service |
| 剛好滿了就拒絕 / 超過上限一樣拒絕 | `Admit`（`count >= limit`）；service 在 `Save` 前返回，不啟動 runner |
| 其中一趟結束後就有空位 | `CountRunning` 只數 `running` |
| 合約滿了，現貨照常開跑 / 合約滿了就拒絕合約 | 兩個 service 各自一份 repository 與上限 |
| 沒設定時用預設值 / 設定不成立時用預設值 | `config.Load`（`positiveIntWithDefault`） |
| 上限 1 時只容一趟 | `Admit` + domain 正規化 |
| 回溯天數不合法先判 / 沒登錄過的代號先判 | `StartHistorySyncFor` 的驗證順序（lookback、`reachSymbolOnDemand` 在數之前） |
| 同一標的已在跑且上限已滿時回答已滿 | 數在 `Save`（唯一索引）之前 |
| 兩個要求搶最後一個空位 | `historySyncStartMutex` 圈住「數 → 判斷 → 記下」 |
| 讀不到進行中趟數 | `CountRunning` 錯誤原樣回傳（controller → `502`），不 `Save` |
| 「太忙」回應 | controller `429` |

---

## 8. Risks & Open Decisions

- **狀態碼：`429` 而非 `409`。** `409` 已用於「同一標的已在跑」（衝突於特定資源，對方應去看那一趟）；上限是整體容量、稍後自然會空、同一個要求原封不動重送就會成功——這正是 `429 Too Many Requests` 的語意（RFC 6585：使用者在一段時間內送出太多請求；也常用於並行上限）。不選 `503`：服務本身健康，其他端點照常。不回 `Retry-After`：一趟多久結束無法預估，給一個數字是在說謊。
- **一份還是兩份上限：兩份。** 現貨與合約打不同 venue、用不同 pacer（`venuePacers.cryptoSpot`/`taiwanStock` vs `cryptoContract`），設定本來就分兩組（`KCANDLE_…` / `CONTRACT_KCANDLE_…`）。現貨那一份涵蓋加密貨幣現貨與台股（同一個 service、同一張 run 表）；台股額度較小（Fugle 55/分），但預設 2 趟仍由 pacer 節奏保護，不另分。
- **競態：在 process 內封住。** go-trading 部署為單一副本、`strategy: Recreate`（`go-trading-deploy/go-trading/deployment.yaml`），新舊 pod 不會同時跑，所以一把 service 鎖即可完全排除「兩個都看到還有空位」。代價是持鎖期間含兩次 DB 往返（數、寫），發起同步本來就少，可以接受。
- **Open decisions:** 無。
