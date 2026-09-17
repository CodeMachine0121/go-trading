# 指名一段歷史同步回來 — Architecture Design

---

## 1. Design Goal & Guiding Principle

**兩件事：一段一段地抓，還有不要讓人等。**

把 K 線抓回來這件事，從窗口以下已經整條做好了：問來源（來源自己分頁）、
逐根判、存下去、寫報告。三條既有的路（定時一輪、開機回補、手動補缺口）
的差別**只有**交給來源的那個時間窗。

這個切片原本也只想加**第四個窗口**：從現在往回推 N 天，不看既有資料——
那是唯一填得了中間破洞的問法。九十天以內，那樣就夠了。

四年不夠。約兩百萬根、幾千次來源往返、十幾分鐘到幾小時。整段一次抓，
記憶體握著兩百萬根等最後一頁；同步回答，一條連線得開十幾分鐘。
兩件事都不是調參數能解決的。

所以歷史同步**不走 `ingestSymbols`**，自己走一條：

1. **切成一天一段，抓一段存一段。** 記憶體是平的，斷掉時前面幾段已經存好。
2. **已經齊全的那幾段連問都不問。** 長區間裡幾乎每一段都是。
3. **先記輪次、回輪次、背景抓。** 沒有哪一條連線值得開十幾分鐘。
4. **照來源的節奏打。** 切段之後請求變密，而來源是按請求數算額度的。
   **一個來源一份節奏**，所有打到那個來源的 proxy 共用——額度是照來源算的，
   不是照問什麼問題算的。
5. **一個標的同時只跑一趟。** 兩趟同時走同一個標的，會互相搶同一份來源額度、
   寫同一批列，而且誰都不會比較早結束。

它另外有一條**寫入規則**：這一趟只寫沒有的。那是四條路裡唯一不覆蓋的一條，
而其餘三條**必須**繼續覆蓋——定時那一輪收的是剛走完那一分鐘的 K 線，
下一輪會在來源把數字定下來之後再收一次，不覆蓋會把每一根都凍在最粗糙的那個版本。
這條規則現在是**結構上**分開的：歷史同步根本不走那三條路的程式碼，
所以沒有任何旗標可以被傳錯。

第二條原則：**既有那一支一個字都不動。** `POST /k-candles/backfill` 的身分是
「回溯長度由系統決定」，那句話寫在它的請求物件上。

---

## 2. Change Scope

| 層 | 動作 |
| :--- | :--- |
| Domain model | 新增 `KCandleHistoryLookbackDomain`；`KCandleIngestionDomain` 多一個窗口與一個切段；`MarketDomain` 多「這段時間開得出幾根」與「這個市場的時區」 |
| Entity | 新增 `KCandleHistorySyncRun`（歷史同步輪次） |
| VO | 新增 `KCandleHistorySyncRunStatusVo` |
| Interface / Repository | `IKCandleRepository` 多「沒有才寫」「整批沒有才寫」「這段有幾根」；新增 `IKCandleHistorySyncRunRepository` |
| DTO | 新增 `KCandleHistorySyncDto`（輸入）與 `KCandleHistorySyncRunDto`（輸出）；報告 DTO 多「跳過幾根」與「名單被截斷了沒」 |
| Domain service | `KCandleIngestionService` 多三個公開用例；新增 `kCandleHistorySyncRunner`（背景推動者） |
| Application | `KCandleIngestionApplication` 多三個 method |
| Controller | 新增 `KCandleHistorySyncController`（兩條路由）+ `KCandleHistorySyncRequest` |
| Infrastructure | 新增 `RequestPacer`；在組裝根一個來源建一份，行情 proxy 與代號查詢 proxy 共用 |
| Config | 新增回溯上限與兩個來源節奏上限 |
| 組裝根 | 兩條路由、一個 repository、啟動時掃殘留輪次 |
| Postman | 新增兩組請求 |

---

## 3. New Classes / Modules

| 型別 | 檔案 | 職責 |
| :--- | :--- | :--- |
| `KCandleHistoryLookbackDomain` | `domain/models/domains/k_candle_history_lookback_domain.go` | 回溯天數：建構子判 1..上限，並換算成時間長度 |
| `KCandleHistorySyncRun` | `domain/models/entities/k_candle_history_sync_run.go` | 一趟歷史同步本身：要抓什麼、走到哪、收集到什麼 |
| `KCandleHistorySyncRunStatusVo` | `domain/models/vo/` | `running` / `succeeded` / `failed`，讀不懂的一律當 `failed` |
| `KCandleHistorySyncDto` / `KCandleHistorySyncRunDto` | `domain/models/dto/` | service 收的那組參數；service 交出去的那趟輪次 |
| `IKCandleHistorySyncRunRepository` | `domain/interface/` | 輪次的讀寫（`Save` / `FindOne` / `FailAllRunning`） |
| `kCandleHistorySyncRunner` | `domain/service/k_candle_history_sync_runner.go` | 背景推動那一趟，並把進度寫回去 |
| `RequestPacer` | `infrastructure/marketdata/request_pacer.go` | 把一個來源壓在它允許的節奏上 |
| `KCandleHistorySyncController` | `controller/` | `POST /k-candles/history`、`GET /k-candles/history/:id` |

### 為什麼回溯天數要一個 Domain Model

它不是一個數字，是一個**有條件的**數字：至少一天、不超過上限、而且超過時的拒絕
要說得出上限是多少。建構子裡判、判過才拿得到值，是這個專案對「有規則的值」的既有做法。

上限由外面給進建構子，因為它是設定而不是領域常識。

### 為什麼輪次是 entity 而不是記憶體裡的一張表

那份工作**活得比請求久**。它不在佇列裡、不在誰的 map 裡——這一筆就是那份工作。
所以有人來問時查得到，重啟時掃得到。這與助手回答那一套是同一個形狀，
理由也是同一個：先寫下來的那一列，是讓「還在跑」這件事看得見的唯一辦法。

### 為什麼推動者不是 Domain Model

它拿的是**執行**：一個背景 context、一份要寫回去的位置、一串還沒走完的段。
把那些拿掉之後不剩任何領域概念。所以它住在 service 旁邊、不加後綴，
規則仍然在 domain model 與 service 裡，它只是推動並記下結果。

### 為什麼節奏守在 proxy 裡，而且一個來源只有一份

只有那裡知道**一次呼叫會變成幾次請求**。上面看到的是「給我這一段」，
下面可能是一次，也可能是三千次；而來源是按請求數算額度的。

而**額度是照來源算的，不是照問什麼問題算的**：問「這個代號上市了嗎」與問
「給我這一天的 K 線」花的是同一份。所以節奏在組裝根建立、發給每一支打到那個
來源的 proxy——一支一份的話，兩支加起來就會跑到任何一支都不被允許的速度。

### 為什麼一個標的同時只跑一趟

兩趟同時走同一個標的，會互相搶同一份來源額度、寫同一批列，而且誰都不會比較早結束。
在這種長度下，多按一次滑鼠就是幾小時的額度花兩次。

**由資料庫決定，不是先讀再寫**：同時到的兩個請求會雙雙看到那個標的沒人在跑。
用的是一條只蓋住 `running` 那幾列的唯一索引——與「一段對話同時只寫一則回答」
同一個手法。被擋下來回 `409` 而不是 `502`：那是有人按了兩次，不是故障。

---

## 4. Modified Components

### `KCandleIngestionDomain`

多一個窗口與一個切段：

```
HistoryWindow(symbol, market, lookback) → KCandleFetchWindowVo
    起點 = 對齊（現在 − lookback）
    終點 = 最後一根走完的一分鐘

HistoryChunks(symbol, market, lookback) → []KCandleFetchWindowVo
    把上面那一段切成一天一段（與粗粒度彙總的最粗一格同寬）
```

**與 `BackfillWindow` 的唯一差別，就是它不看既有資料。** `BackfillWindow` 會把起點
往後推到「已存最新那一根之後」，而那個起點**跨不回中間的破洞**。

切段寬度取「最粗的那一格」而不是隨便一個數字，因為系統已經有那個刻度，
而段邊界對齊刻度邊界時，每一段都是完整的一格——進度才說得出「第幾天」。

### `MarketDomain` 多 `TradingKCandleCountBetween`

「這個市場在這段時間裡開得出幾根」。**「已經齊全就不問」全靠它**：
手上有幾根，跟開得出幾根比一比。它與既有的 `TradingBucketCountBetween` 分開，
因為那一個在全年無休的市場上會少算一根（既有的已知差異，不在這個切片改）。

### `IKCandleRepository` 多三個

- `SaveIfAbsent` / `SaveAllIfAbsent`——**不覆蓋**。是第二個方法而不是 `Save` 上的旗標，
  因為那是兩種意圖而不是一種意圖加一個設定。**判斷交給資料庫**
  （`ON CONFLICT DO NOTHING`），不是先讀一次再寫：兩個範圍重疊的同步同時在跑時，
  先讀再寫會雙雙認定某一分鐘不存在、雙雙寫下去。
- `CountInRange`——「這段手上有幾根」，「已經齊全就不問」的那半。

整批寫是給歷史同步用的：一段一千多根，一根一句話是一千多次往返。
其餘三條路一次只有幾根，維持逐根寫（那也讓它們的「哪一根寫不進去」仍然指得出來）。

### `KCandleIngestionService`

多三個公開用例，並把共用的部分抽出來：

```
StartHistorySyncFor(dto, ceiling)
  ├─ 判回溯天數      KCandleHistoryLookbackDomain
  ├─ reachSymbolOnDemand   （與 RunBackfillFor 共用：判代號、查登錄、放掉休市判斷）
  ├─ 切段            HistoryChunks
  ├─ 記輪次（running + 總段數）  ← 記不下來就整個拒絕
  └─ go runner.run() ；回那筆輪次

GetHistorySyncRun(id)
FailInterruptedHistorySyncs()      ← 啟動時掃

（私有）syncSymbolHistory(symbol, chunks, recordProgress)
  逐段：收進交易時段 → 已齊全就跳過 → 照節奏問 → judge → SaveAllIfAbsent → 回報進度

（私有）judge(reported, domain)    ← 與 ingestSymbol 共用的「哪幾根可以存」
```

**歷史同步不經過 `ingestSymbols`**，而那是刻意的兩件事：這裡永遠只有一個標的，
而且從那裡通得到的「休市帳」絕不能看到這一趟——一段已經齊全的歷史同步會
「被問了卻什麼都沒收到」，那正是休市帳用來判定整個市場休市的訊號。

### Controller

兩條路由。`POST` 回 `202` 與輪次；`GET` 是那個編號真正有用的地方——
一個沒地方去的編號只是一張沒人看得懂的收據。

狀態碼：回溯天數／代號說不通 `400`、沒登錄 `404`、這個系統自己壞掉 `502`。
**來源不答話不在其中**：那是這一趟查到的事，寫在輪次的 `fetchFailureReason` 上。

---

## 5. Component Relationships

```
KCandleHistorySyncController
        │  KCandleHistorySyncRequest → KCandleHistorySyncDto
        ▼
KCandleIngestionApplication.StartSymbolHistorySync / GetSymbolHistorySync
        ▼
KCandleIngestionService.StartHistorySyncFor
        ├─ KCandleHistoryLookbackDomain          （判天數）
        ├─ KCandleIngestionDomain.HistoryChunks  （切段）
        ├─ IKCandleHistorySyncRunRepository      （記輪次）
        └─ go kCandleHistorySyncRunner.run()
                 └─ KCandleIngestionService.syncSymbolHistory（私有）
                        ├─ MarketDomain.TradingKCandleCountBetween（已齊全？）
                        ├─ IKCandleRepository.CountInRange
                        ├─ IMarketDataProxy.FetchKCandles → requestPacer
                        └─ IKCandleRepository.SaveAllIfAbsent
```

依賴方向不變。節奏守在 infrastructure，輪次的規則在 domain。

---

## 6. Extensibility & Handoff Notes

- 想改成「指定起訖時刻」時，落點是再一個切段方法，加上 DTO 多兩個欄位。
- 想做「重抓一段並修正它」時，落點是 `SaveAllIfAbsent` 換成覆蓋版。
- 想讓一趟同步**可以取消**時，落點是輪次多一個狀態與一個取消訊號，
  推動者在每段之間檢查一次。這一版刻意不做——要停就重啟。
- 想讓「已齊全就不問」認得**休假日與上市前**時，落點是記下「問過了，那天本來就沒有」
  ——那是一個新的概念（一段被確認為空），而不是把期望根數算得更聰明：
  這個系統沒有行事曆，也不知道一檔標的何時上市，而來源知道。

---

## 7. Traceability

| AC | 落在哪 |
| :--- | :--- |
| US-01 抓回指定的那一段 | `HistoryChunks` + `syncSymbolHistory` |
| US-01 問整段（填得了破洞） | `HistoryWindow` 不看既有資料 |
| US-01 已經有的不覆蓋 | `SaveAllIfAbsent`（`ON CONFLICT DO NOTHING`） |
| US-01 已經齊全的不問 | `CountInRange` vs `TradingKCandleCountBetween` |
| US-01 抓得動四年 | `HistoryChunks` 切段 + 逐段存 + `requestPacer` |
| US-01 粒度固定 | 請求物件上沒有那個欄位 |
| US-02 代號／回溯天數說不通 | `reachSymbolOnDemand`、`KCandleHistoryLookbackDomain` |
| US-03 邊界上的 1 天與上限 | 同上 |
| US-04 交代它做了什麼 | 輪次的 `storedCount` / `skippedCount` / `fetchFailureReason` |
| US-05 不等我但看得到進度 | `KCandleHistorySyncRun` + `kCandleHistorySyncRunner` + `GET` |
| US-05 重啟掃殘留 | `FailAllRunning` + `main.go` 啟動時那一段 |

---

## 8. Risks & Open Decisions

| 風險 | 緩解 |
| :--- | :--- |
| 一趟四年要十幾分鐘到幾小時 | 先回輪次、背景抓、可查進度。不靠上限 |
| 來源額度 | 每個來源自己的節奏上限，且留餘裕——額度是跟系統其他呼叫共用的 |
| 進度寫得太頻繁 | 一段一次。相對於那一段幾百到幾千次照節奏發出的請求，是零頭；而只在收尾才出現的數字跟卡住的分不出來 |
| 這條寫入規則被誤用到自動那幾輪 | **結構上分開**：歷史同步不走 `ingestSymbols`，沒有旗標可以傳錯。另有一條測試釘住定時那一輪仍然覆蓋 |
| 壞掉的根太多，報告打不開 | 名單封頂 200 筆，`skippedCount` 照實算 |
| 同一個標的被開兩趟 | 只蓋住 `running` 的唯一索引，由資料庫擋，回 `409` |
| 收尾那一筆寫不進去，輪次永遠掛在 `running` | 收尾的寫入會重試（進度的不會——下一段馬上又寫一次） |
| 「已齊全就不問」認不出休假日與上市前 | **已知代價**。期望根數只認得星期與盤別，不認得國定假日，也不知道這檔何時上市，所以那幾段每跑一次就重問一次。它只會多打幾次，不會漏資料 |

### Open decisions（交給實作）

- 路徑 `POST /k-candles/history` 與 `GET /k-candles/history/:id`。
- 進度用**段數**而不是百分比或根數：段是這趟真正往前走的單位，
  而「第 11 段／共 30 段」不需要任何人先同意一個算法。
