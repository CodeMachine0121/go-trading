# 粗刻度 K 線序列先彙總好再給 — Architecture Design

**Status:** Confirmed
**Source PRD:** `.sdd/2026-10-18-coarse-chart-series-summaries/PRD.md`
**Tech context:** Go · Gin · GORM（Postgres，Code First）· Clean Architecture（Controller → Application → Domain ← Infrastructure）

---

## 1. Design Goal & Guiding Principle

- **In one sentence:** 彙總刻度為 `1h`/`4h`/`1d` 的序列，把查詢區間切成「頭、尾兩段用一分鐘 K 線，中間整格整格取留存彙總」，缺的天一天一天補；任何一分鐘寫入在同一個交易內作廢那一天。
- **Guiding principle:** **留存彙總與一分鐘 K 線同一種形狀**。留存彙總讀回來時轉成 `entities.KCandle` / `entities.KCandleContract`，餵給**原本的** `KCandleSeriesDomain` / `KCandleContractSeriesDomain` 合併——現行合併規則對「先合成小時、再合成天」是結合的（開取最早、收取最晚、極值、加總；「不報」只在全不報時成立；合約價格線只要缺一份就缺），所以「一字不差」由構造保證，而不是由第二套合併碼去追。

---

## 2. Change Scope

| Area | Action | What / Why |
| :--- | :--- | :--- |
| `entities.KCandleSummaryDay` / `KCandleSummary`、`KCandleContractSummaryDay` / `KCandleContractSummary` | **Add** | 一天的彙總狀態（版本號、是否已彙總）與那一天的小時／日留存彙總；現貨與合約各一套（同一個代號在兩個市場是兩檔） |
| `IKCandleSummaryDayRepository` / `IKCandleContractSummaryDayRepository` + persistence 實作 | **Add** | 以「一天」為聚合讀寫：讀一段的彙總狀態與留存彙總（同一快照）、條件式留存一天 |
| `KCandleSummaryPlanDomain` | **Add** | 一次序列要求怎麼切：頭尾分鐘段、中間留存彙總段、彙總粒度、哪些天缺 |
| `KCandleSummaryDayDomain` / `KCandleContractSummaryDayDomain` | **Add** | 用一天的一分鐘 K 線合出那一天的小時與日留存彙總（重用現行合併） |
| `kCandleSummarizedSeriesReader` / `kCandleContractSummarizedSeriesReader`（service 旁） | **Add** | 照計畫讀、缺的天一天一天補，交回一串「來源 K 線」 |
| `KCandleSummaryService` + `KCandleSummaryApplication` + `KCandleSummaryJob` | **Add** | 值班時每五分鐘補最近四十八小時走完的天 |
| `KCandleService.GetKCandleSeries` / `KCandleContractService.GetKCandleContractSeries` | **Modify** | 粗刻度改走 reader；細刻度原樣 |
| `KCandleRepository` / `KCandleContractRepository` 的寫入方法 | **Modify** | 寫入與作廢放在同一個交易 |
| `SchemaMigrator`、`config`、`dependencies.go` | **Modify** | 註冊新 entity、新增 `KCANDLE_SUMMARY_INTERVAL_MINUTES`、組裝 |
| 指標計算、回測、策略機器人、交易日誌、`FindLatestBefore` | **Not touched** | PRD Out of Scope；它們照舊逐分鐘合併 |
| `KCandleSeriesDomain` / `KCandleContractSeriesDomain` / `KCandleBucketDomain` 的合併規則 | **Not touched** | 刻意重用，保證一字不差 |
| 控制器、回應形狀、驗證與筆數上限 | **Not touched** | 回應不變 |

---

## 3. New Classes / Modules

| Name | Kind | Responsibility (purpose) | Collaborators | Satisfies (PRD scenario) |
| :--- | :--- | :--- | :--- | :--- |
| `KCandleSummaryDay`（entity，表 `KCandleSummaryDays`） | Entity（聚合根） | 一檔一天的彙總狀態：`Symbol`、`Day`、`Revision`（每次作廢 +1）、`Summarized`；唯一 `(symbol, day)` | — | 作廢、合併中被改動不留存、只補缺的那幾天 |
| `KCandleSummary`（表 `KCandleSummaries`） | Entity（聚合成員） | 一格留存彙總：`Symbol`、`Interval`（`1h`/`1d`）、`OpenTime` 與 `KCandle` 相同的八個數字；唯一 `(symbol, interval, open_time)`；`ToKCandle()` 轉回一分鐘 K 線形狀 | — | 粗刻度取留存彙總、一字不差 |
| `KCandleContractSummaryDay` / `KCandleContractSummary` | Entity | 同上，合約版；後者帶 `KCandleContract` 的全部 21 個數字，`ToKCandleContract()` | — | 合約各情境 |
| `IKCandleSummaryDayRepository` / `IKCandleContractSummaryDayRepository` | 介面 | `FindDaysInRange(ctx, symbol, interval, startTime, endTime) ([]KCandleSummaryDay, error)`：一個唯讀 Repeatable Read 交易內取 `[dayOf(start), end)` 的每天狀態，已彙總的天附上該粒度、落在 `[start, end)` 的留存彙總；`SaveSummarized(ctx, day) (bool, error)`：只在那一天的版本號仍等於 `day.Revision`（或仍不存在）時，一個交易內標記已彙總並寫入該天全部留存彙總，否則回 `false` 什麼都不寫 | GORM | 同一快照不漏格、合併中被改動不留存 |
| `KCandleSummaryPlanDomain` | Domain Model | 由 `KCandleSeriesQueryDomain` 與 now 建出：`UsesSummaries()`（1h/4h/1d）；`SummaryInterval()`（1h→1h、4h→1h、1d→1d）；`SummarizedStretch()` = `[alignUp(start), min(BucketStart(end), todayStart))`；`MinuteStretches()` 頭 `[start, summarizedStart)`、尾 `[summarizedEnd, end]`（各附讀取上限 = 分鐘數 + 1）；`DaysToSummarize(days)` 列出區段內沒有已彙總狀態的每一天與它目前的版本號 | `AggregationIntervalDomain`、`KCandleQueryDomain` | 今天／這一小時當場合併、細刻度照舊、只補缺的那幾天 |
| `KCandleSummaryDayDomain` | Domain Model | 由 symbol、day、revision、那一天的一分鐘 K 線建出；`ToEntity()` 以 `KCandleSeriesDomain`（1h、1d）合出 24 格內的小時留存彙總與 1 格日留存彙總；沒有分鐘就沒有格子 | `KCandleSeriesDomain`、`KCandleBucketDomain` | 一字不差、沒有分鐘的那一天不出現 |
| `KCandleContractSummaryDayDomain` | Domain Model | 同上，以 `KCandleContractSeriesDomain` 合併 | `KCandleContractSeriesDomain` | 合約缺指數價格／溢價指數 |
| `kCandleSummarizedSeriesReader`（`service/` 旁、不加後綴） | 執行物件 | `read(ctx, planDomain) ([]entities.KCandle, error)`：讀頭尾分鐘段、`FindDaysInRange`、缺的天逐一 `summarizeDay`（讀那一天 ≤1440 根 → `KCandleSummaryDayDomain` → `SaveSummarized`），回傳頭段分鐘＋留存彙總（轉成 KCandle）＋尾段分鐘；`summarizeDays(ctx, symbol, days)` 供背景那一輪用 | 分鐘 repo、彙總 repo | 一天一天補、補到一半失敗已補好的保留 |
| `kCandleContractSummarizedSeriesReader` | 執行物件 | 同上，合約版 | 合約分鐘 repo、合約彙總 repo | 同上 |
| `KCandleSummaryService` | Domain Service | `SummarizeRecentDays(ctx) error`：觀察清單現貨與合約追蹤名單，對 `[todayStart−48h, todayStart)` 內未彙總的天逐一補 | 兩個 reader、`ITradingSymbolRepository`、`IContractTradingSymbolRepository`、`IClockProxy` | 背景那一輪各情境 |
| `KCandleSummaryApplication` | Application | `RunRound(ctx) error` | `KCandleSummaryService` | 同上 |
| `KCandleSummaryJob` | Background Job | `repeatingRound` + `IsLeader()`；失敗記一行 log | Application、`JobLeadershipApplication` | 只由值班那一處、停用時看圖結果不變 |
| `kCandleSummaryInvalidation[TDay, TSummary]`（persistence 內） | 持久化組件 | `invalidate(tx, symbol, days)`：每一天 upsert 狀態 `revision = revision + 1, summarized = false`，並刪除那一天的全部留存彙總 | GORM tx | 一分鐘 K 線一變即作廢、同時成立 |

---

## 4. Modified Components

| Component | Current role | Change needed |
| :--- | :--- | :--- |
| `KCandleService.GetKCandleSeries` | 驗證 → `FindInRange(SourceCandleLimit)` → `SeriesOf` | 驗證後建 `KCandleSummaryPlanDomain`；`UsesSummaries()` 為真時 `reader.read` 取來源 K 線，否則照舊；最後一樣 `SeriesOf(...).ToDto()`。建構子多收 `IKCandleSummaryDayRepository` |
| `KCandleContractService.GetKCandleContractSeries` | 同上（合約） | 同上，用合約 reader，建構子多收 `IKCandleContractSummaryDayRepository` |
| `KCandleRepository.Save` / `SaveAllIfAbsent` / `Update` / `Delete` | 寫一分鐘 K 線 | 包進 `Transaction`：寫入成功後 `invalidate(tx, symbol, 涉及的天)`；`SaveAllIfAbsent` 與 `Update` / `Delete` 只在 `RowsAffected > 0` 時作廢 |
| `KCandleContractRepository` 同四個方法 | 寫合約一分鐘 K 線（含晚補指數／溢價） | 同上；`SaveAllIfAbsent` 的晚補 `DoUpdates` 計入 `RowsAffected`，所以晚補也作廢 |
| `SchemaMigrator.Migrate` | 註冊 entity | 加四個新 entity |
| `config.ApplicationConfig` | — | `KCandleSummary.Interval`：`KCANDLE_SUMMARY_INTERVAL_MINUTES`（預設 5，`jobIntervalWithDefault`，≤0 停用） |
| `cmd/server/dependencies.go` | 組裝 | 新 repo、service、application、job；兩個 series service 多注入彙總 repo |
| `.mockery.yaml` | mock 設定 | 加兩個新介面 |

**為什麼作廢放在 repository：** PRD 要求作廢與變動「同時成立」，而九條寫入路徑最後都落在這八個 repository 方法——放在這裡是唯一能以單一交易保證、且新增第十條寫入路徑也不會漏的位置。它是持久化一致性（等同一個觸發器），不是業務規則；作廢的業務含義（哪一天、版本號）仍記在 UL-MAP。

---

## 5. Component Relationships

```mermaid
flowchart TD
    Controller[KCandle(Contract)Controller.Get...Series] --> App[KCandle(Contract)Application]
    App --> Svc[KCandleService / KCandleContractService]
    Svc --> Plan[KCandleSummaryPlanDomain]
    Svc --> Reader[k(Contract)CandleSummarizedSeriesReader]
    Reader --> MinuteRepo[(IKCandle(Contract)Repository)]
    Reader --> SummaryRepo[(IKCandle(Contract)SummaryDayRepository)]
    Reader --> DayDomain[KCandle(Contract)SummaryDayDomain]
    Svc --> Series[KCandle(Contract)SeriesDomain 原樣合併]
    Job[KCandleSummaryJob] --> SumApp[KCandleSummaryApplication] --> SumSvc[KCandleSummaryService] --> Reader
    Writes[九條寫入路徑] --> MinuteRepo
    MinuteRepo -->|同一交易| Invalidate[kCandleSummaryInvalidation]
```

---

## 6. Extensibility & Handoff Notes

- **Most likely next requirement:** 回測、指標計算也改讀留存彙總；或讓背景那一輪把整段歷史補齊。
- **Where it lands:** reader 的 `read(ctx, planDomain)` 與粒度無關——回測只要用自己的區間建一個 `KCandleSummaryPlanDomain` 就能拿到同一串來源 K 線，後面照舊走 `SelectInputCandles`。整段補齊只是 `KCandleSummaryService` 換一個天數範圍。
- **How to add it:** 回測：在 `BacktestService` 注入 reader、改 `FindInRange` 那一行；新增粒度（例如週）：`KCandleSummaryPlanDomain.SummaryInterval()` 加一個對應、`KCandleSummaryDayDomain` 多合一種粒度。
- **Patterns applied & why:** 以「天」為單位的聚合＋樂觀版本號（作廢 +1、留存比對），處理並行寫入；cache-aside 但作廢在寫入交易內。
- **Do not hardcode:** 背景間隔走設定；四十八小時是 PRD 的規則，以具名常數放在 `KCandleSummaryService`。
- **Known debt / deferred:** 初次上線第一次往前接九百天仍要一天一天補（不中斷但慢）；之後每天由背景那一輪補新的一天。若之後要首屏也快，再讓背景那一輪往回補觀察清單的歷史。

---

## 7. Traceability

| PRD Scenario | Fulfilled by |
| :--- | :--- |
| 九百天都已彙總好，日線很快回來 | `KCandleSummaryPlanDomain`（1d 取日留存彙總）＋ `kCandleContractSummarizedSeriesReader.read` ＋ `KCandleContractSummary.ToKCandleContract` ＋ `KCandleContractSeriesDomain` |
| 四小時由小時彙總合併而成 | `KCandleSummaryPlanDomain.SummaryInterval`（4h→1h）＋ 原樣 `SeriesOf` 合併 |
| 今天還在走的那一根當場合併 | `KCandleSummaryPlanDomain.MinuteStretches`（尾段 `[todayStart, end]`） |
| 今天的每一小時當場合併 | 同上（今天的小時全部由尾段分鐘合併） |
| 細刻度照舊當場合併 | `KCandleSummaryPlanDomain.UsesSummaries` 為假 → 原路徑 |
| 一天裡有一分鐘缺指數價格，日線的指數價格就是缺 | `KCandleContractSummaryDayDomain`（重用 `mergedPriceLine`） |
| 四小時格子裡有一小時缺溢價指數，那一格的溢價指數就是缺 | 小時留存彙總存「缺」→ 原樣 `KCandleContractSeriesDomain` 合併 |
| 市場不報的成交數字維持不報 | `KCandleSummaryDayDomain`（`OptionalFigureDomain`）＋ `NullDecimal` 欄位 |
| 只有部分分鐘報了的成交數字取有報的加總 | 同上 |
| 沒有任何一分鐘 K 線的那一天不會出現 | `KCandleSummaryDayDomain.ToEntity`（無分鐘無格子，狀態仍標已彙總） |
| 歷史同步補進一根，所屬的天與小時作廢 | `KCandleRepository.SaveAllIfAbsent` + `kCandleSummaryInvalidation` |
| 合約晚補指數價格，所屬的天與小時作廢 | `KCandleContractRepository.SaveAllIfAbsent` + `kCandleSummaryInvalidation` |
| 手動改寫一根，所屬的天與小時作廢 | `Update` + `kCandleSummaryInvalidation` |
| 手動刪除一根，所屬的天與小時作廢 | `Delete` + `kCandleSummaryInvalidation` |
| 例行抓取修正剛走完那一小時裡的一根 | `Save` + `kCandleSummaryInvalidation` |
| 一天最後一分鐘變了只影響那一天 | `kCandleSummaryInvalidation`（以 UTC 日界作廢） |
| 合併當中那一天被改動，這次結果不留下 | `SaveSummarized` 版本號比對 |
| 只補缺的那幾天 | `KCandleSummaryPlanDomain.DaysToSummarize` ＋ reader |
| 一天都還沒彙總好，也只一天一天處理 | reader 逐天 `summarizeDay`（每次讀 ≤1440 根） |
| 補到一半失敗，已補好的不必重做 | reader 每天各自 `SaveSummarized`；失敗即回傳錯誤 |
| 剛走完的一天連同它的每一小時被補好 | `KCandleSummaryService.SummarizeRecentDays` ＋ `KCandleSummaryDayDomain`（一天的日與 24 小時一起留存） |
| 剛走完的一天被補好 | `KCandleSummaryService.SummarizeRecentDays` |
| 背景那一輪停用時看圖結果不變 | reader 現場補（與 job 無關） |
| 不在任何名單上的標的不在背後補 | `KCandleSummaryService`（只走兩份名單） |

> 留存彙總以**天**為最小補齊單位：今天的小時一律由尾段分鐘合併（至多 1440 根，與細刻度同級成本）。以天為單位讓作廢、版本號與補齊都只有一種粒度。

---

## 8. Risks & Open Decisions

- **Risks / trade-offs:**
  - 每一次一分鐘寫入多兩句（狀態 upsert、刪留存彙總）。例行抓取每分鐘每檔 25 次，可接受；換來的是不可能漏作廢。
  - 初次往前接九百天約要逐天補九百次查詢（每次 ≤1440 根），慢但記憶體平坦。
  - 兩份相同要求同時補同一天：版本號相同時第二份 `SaveSummarized` 以「已彙總」為前提失敗回 `false`，但仍使用自己合出的結果回應；結果相同。
- **Open decisions (for implementation):** 無。
