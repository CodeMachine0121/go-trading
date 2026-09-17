# 指名一段歷史同步回來 — Architecture Design

---

## 1. Design Goal & Guiding Principle

**只多一個窗口與一條寫入規則，其餘一律沿用。**

把 K 線抓回來這件事，從窗口以下已經整條做好了：問來源（來源自己分頁）、
逐根判、存下去、寫報告。三條路（定時一輪、開機回補、手動補缺口）的差別**只有**
交給來源的那個時間窗。

所以這個切片加的是**第四個窗口**：從現在往回推 N 天，不看既有資料——
那是唯一填得了中間破洞的問法。

它另外加一條**寫入規則**：這一趟只寫沒有的，已經有的原封不動。那是四條路裡唯一
不覆蓋的一條，而其餘三條**必須**繼續覆蓋——定時那一輪收的是剛走完那一分鐘的
K 線，下一輪會在來源把數字定下來之後再收一次，不覆蓋會把每一根都凍在最粗糙的
那個版本。

它不新增報告形狀、不碰來源。

第二條原則：**既有那一支一個字都不動。** `POST /k-candles/backfill` 的身分是
「回溯長度由系統決定」，那句話寫在它的請求物件上。在它身上加一個選填欄位會讓它
變成半真的；兩條路各自說得完整，比一條路說一半好。

---

## 2. Change Scope

| 層 | 動作 |
| :--- | :--- |
| Domain model | 新增 `KCandleHistoryLookbackDomain`；`KCandleIngestionDomain` 多一個窗口 |
| Interface / Repository | `IKCandleRepository` 多一個「沒有才寫」 |
| DTO | 新增 `KCandleHistorySyncDto`（application → domain 的輸入形狀） |
| Domain service | `KCandleIngestionService` 多一個公開用例 |
| Application | `KCandleIngestionApplication` 多一個 method |
| Controller | 新增 `KCandleHistorySyncController` + `KCandleHistorySyncRequest` |
| Config | 新增一個上限 |
| 組裝根 | 註冊一條路由 |
| Postman | 新增一組請求 |

---

## 3. New Classes / Modules

| 型別 | 檔案 | 職責 |
| :--- | :--- | :--- |
| `KCandleHistoryLookbackDomain` | `internal/domain/models/domains/k_candle_history_lookback_domain.go` | 回溯天數：建構子判 1..上限，並換算成時間長度 |
| `KCandleHistorySyncDto` | `internal/domain/models/dto/k_candle_history_sync_dto.go` | service 收的那組參數（標的 + 天數） |
| `KCandleHistorySyncController` | `internal/controller/k_candle_history_sync_controller.go` | `POST /k-candles/history` |
| `KCandleHistorySyncRequest` | `internal/controller/models/k_candle_history_sync_request.go` | 那支 endpoint 的 body |

### 為什麼回溯天數要一個 Domain Model

它不是一個數字，是一個**有條件的**數字：至少一天、不超過上限、而且超過時的拒絕
要說得出上限是多少。建構子裡判、判過才拿得到值，是這個專案對「有規則的值」的既有做法
（`AssistantAskDomain`、`BacktestInitialCapitalDomain` 都是這樣）。

上限由外面給進建構子，因為它是設定而不是領域常識。

### 為什麼 service 收 DTO 而不是兩個參數

`naming.md`：「Service 收的參數用 DTO」。既有的 `RunBackfillFor` 收一個字串是因為
它只有一個；這一支有兩個，而第三個（例如起訖時刻）遲早會有人想加。

---

## 4. Modified Components

### `KCandleIngestionDomain`

多一個窗口：

```
HistoryWindow(symbol, market, lookback) → KCandleFetchWindowVo
    起點 = 對齊（現在 − lookback）
    終點 = 最後一根走完的一分鐘
```

**與 `BackfillWindow` 的唯一差別，就是它不看既有資料。** `BackfillWindow` 會把起點
往後推到「已存最新那一根之後」，而那個起點**跨不回中間的破洞**；這一個不推，
所以問得到整段，洞才補得起來。

### `IKCandleRepository` 多一個 `SaveIfAbsent`

**第二個方法而不是 `Save` 上的一個旗標**，因為那是兩種意圖而不是一種意圖加一個設定：
自動那幾輪收的是還在成形的 K 線、本來就打算等來源定案後再蓋掉；補洞則是絕不碰
已經有的。

**判斷交給資料庫**（同一個開盤時刻就什麼都不做），不是先讀一次再寫。兩個範圍重疊的
同步同時在跑時，先讀再寫會雙雙認定某一分鐘不存在、雙雙寫下去，而後寫的那個
正好蓋掉這條規則要保護的東西。

### `KCandleIngestionService`

多一個公開用例 `SyncHistoryFor`，以及一個說明「這一趟怎麼寫」的 `kCandleWriteRule`
（覆蓋／保留）。四條路裡只有歷史同步是保留。

它與 `RunBackfillFor` **互不呼叫**（同一個 service 的公開用例互不呼叫），
但兩者共用同一段私有流程：

```
SyncHistoryFor(dto)
  ├─ 判代號          （與 RunBackfillFor 同一段私有 helper）
  ├─ 判回溯天數      【新】KCandleHistoryLookbackDomain
  ├─ 查登錄          （同上）
  ├─ 放掉休市判斷    （同上）
  └─ ingestSymbols(…, HistoryWindow)   ← 只有這一行不同
```

前四步是 `RunBackfillFor` 已經有的，且都只被這兩個公開用例用到——**兩個以上**，
所以抽成一個私有 helper 是對的（`architecture.md` 的門檻）。

### Controller

自己一支，理由與 `KCandleBackfillController` 當初分出來的一樣：它交出去的不是一根
K 線，而是一次取數。它與回補分開，是因為兩者的規則不同——一個讓你說多久，一個不讓。

狀態碼沿用既有那一支的對映，多一個回溯天數的 400。

---

## 5. Component Relationships

```
KCandleHistorySyncController
        │  KCandleHistorySyncRequest → KCandleHistorySyncDto
        ▼
KCandleIngestionApplication.SyncSymbolHistory
        ▼
KCandleIngestionService.SyncHistoryFor
        ├─ KCandleHistoryLookbackDomain      （判天數）
        ├─ KCandleIngestionDomain.HistoryWindow（算窗口）
        └─ ingestSymbols                      （既有：問來源、判、存、寫報告）
                 ├─ IMarketDataProxy          （既有，自己分頁）
                 └─ IKCandleRepository        （既有）
```

依賴方向不變。**來源、存入、報告三者一個字都沒動。**

---

## 6. Extensibility & Handoff Notes

- 想改成「指定起訖時刻」時，落點是再一個窗口方法，加上 DTO 多兩個欄位。
  其餘照樣不用動——這就是「差別只有窗口」的用處。
- 想做「重抓一段並修正它」時，落點只有一個字：把那條寫入規則換成覆蓋。
  窗口、關卡、報告一律照舊。
- 真的要抓一年時，該做的是**邊抓邊存**（把窗口切成一天一段，存完再抓下一段），
  而不是把上限調高。現在整段候在記憶體裡，那是上限存在的理由。
- 要做成非同步時，`pipeline_run`／助手那套「先留紀錄再寫回」是現成的樣子。
  這一版刻意不做——沒有畫面在等。

---

## 7. Traceability

| AC | 落在哪 |
| :--- | :--- |
| US-01 抓回指定的那一段 | `HistoryWindow` + `SyncHistoryFor` |
| US-01 問整段（填得了破洞） | `HistoryWindow` 不看既有資料 |
| US-01 已經有的不覆蓋 | `kCandleWriteRule` 的保留那一邊 + `SaveIfAbsent` |
| US-01 粒度固定 | 請求物件上沒有那個欄位 |
| US-02 代號說不通／沒登錄 | 沿用 `RunBackfillFor` 的那兩道，抽成共用私有 helper |
| US-02 回溯天數說不通 | `KCandleHistoryLookbackDomain` 建構子 |
| US-03 邊界上的 1 天與上限 | 同上 |
| US-04 報告一字不差 | 共用 `ingestSymbols`，沒有第二種報告 |

---

## 8. Risks & Open Decisions

| 風險 | 緩解 |
| :--- | :--- |
| 一次九十天要好幾分鐘 | 上限封頂；要更長時先做邊抓邊存 |
| 問整段對來源打很多次，即使資料已經齊全 | 這是填得了破洞的代價；手動工具，按的人知道自己在做什麼 |
| 這條寫入規則被誤用到自動那幾輪 | 四條路各自在呼叫處明寫用哪一種；定時那一輪有一條測試釘住它仍然覆蓋 |
| 十三萬根候在記憶體 | 同上限；這是上限真正在封的東西 |

### Open decisions（交給實作）

- 路徑 `POST /k-candles/history`。不叫 `/sync`，因為它與隔壁的 `/backfill` 意思太近，
  而兩者的差別正是這一支的重點：**你說要多少歷史**。
