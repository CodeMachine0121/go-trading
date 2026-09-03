# K 線即時跟盤 — Architecture Design

**Feature:** K 線即時跟盤
**Status:** Finalized
**PRD:** `PRD.md`（同一資料夾）
**Owner:** James Hsueh

---

## 1. Design Goal & Guiding Principle

系統第一次同時引入兩條「持續連著」的通道：**進來**的一條（跟著行情來源）與**出去**的一條
（推給觀看者）。兩條都是傳輸，兩條都不能讓 domain 認識。

指導原則只有三句：

1. **傳輸只有兩個端點知道。** 進來那條藏在一個 Proxy 實作裡，出去那條藏在一個 controller 裡。
   中間的每一層都只看得到「一根 K 線目前的樣子」與「這一份要不要送出去」。
2. **有數字的規則一律是 domain 的。** 「至多每十秒」「安靜超過多久算跟不動」
   「重試間隔怎麼拉長」全部是 PRD 明文寫下的業務規則，不是連線的調校參數——
   它們住在一個 domain model 上，可以用表格測試逐條釘死，不必真的連線。
3. **既有的寫入路徑一個字都不改。** 一根走完時的把關與存入，走的是攝取切片
   已經在用的那條路：`NewKCandleDomain(writeDto, now)` → `ToEntity()` → `Save`。
   本切片只是多一個呼叫它的人。

### 為什麼「出去那條」沒有 Proxy 介面

直覺上會想替「推給觀看者」也開一個 `IViewerProxy`。**不要。**

domain 不需要「推」的能力，它需要的是「把這一份交出去」。交出去用 Go 的 channel 就夠了——
channel 是語言原語，不是傳輸方式。domain service 交出一個
`<-chan dto.KCandleFollowUpdateDto`，誰拿去做什麼它不管；
controller 把每一份寫成一個事件送給瀏覽器。

這正好回答了「長生命週期的連線會不會違反 handler 只做 HTTP 轉換」：
**不會，因為它做的還是純轉換，只是做 N 次而不是一次。** handler 裡沒有任何業務判斷——
要不要送、什麼時候送、送不動了怎麼辦，全部在它拿到 channel 之前就決定完了。
它唯一多做的事是「請求結束時走人」，而那本來就是 HTTP 的事。

---

## 2. Change Scope

### 新增

| 層 | 檔案 | 為什麼存在 |
| :--- | :--- | :--- |
| domain/interface | `i_live_market_data_proxy.go` | 「持續跟著一個市場」這個能力的契約，以能力命名不綁供應商 |
| domain/models/vo | `live_k_candle_vo.go` | 來源送來的一根目前的樣子，加上「走完了沒」 |
| domain/models/dto | `k_candle_follow_update_dto.go` | 觀看者收到的唯一形狀 |
| domain/models/domains | `k_candle_follow_domain.go` | 一份跟盤隨時間演變的**全部規則**（節流、安靜判定、重試間隔） |
| domain/service | `k_candle_follow_service.go` | 共享訂閱的生命週期：誰在看、跟幾份、何時停 |
| application | `k_candle_follow_application.go` | 用例：一個觀看者要看某個交易標的 |
| controller | `k_candle_follow_controller.go` | 唯一知道推播長什麼樣子的地方 |
| infrastructure/marketdata | `binance_live_market_data_proxy.go` | 唯一知道來源的即時通道長什麼樣子的地方 |

### 修改

| 檔案 | 改什麼 |
| :--- | :--- |
| `cmd/server/config.go` | 三個新環境變數（見 §4） |
| `cmd/server/dependencies.go` | 組裝與路由；註冊跟盤服務的收尾 |
| `cmd/server/serve.go` | 關機時一併停掉跟盤服務 |
| `README.md`、`postman/` | 新的進入點與環境變數 |

### 刻意不動

- **`KCandleIngestionJob` / `KCandleIngestionService` / `KCandleIngestionDomain`** ——
  每五分鐘那一輪一行都不改。它負責的兩件事（沒人看時的每一根、來源事後的修正）
  跟盤本來就做不到，兩者是互補不是取代。
- **`KCandleRepository`** —— `Save` 已經是 `(symbol, open_time)` 上的 upsert，
  跟盤直接用它。**這正是兩個寫入者能共存的全部理由**，不需要鎖、不需要協調、
  也不需要誰讓誰：後寫的覆蓋先寫的，而兩邊寫的是同一根的同一批數字。
- **指標計算的每一條路徑** —— 進行中的那一根不進資料庫，計算讀的是資料庫，
  因此計算**完全不知道跟盤存在**。這不是靠額外的判斷達成的，是靠不存達成的。
- **既有的查詢、新增、修改、刪除 K 線。**

---

## 3. New Classes / Modules

### `ILiveMarketDataProxy`（domain/interface）

```go
type ILiveMarketDataProxy interface {
    FollowKCandles(
        executionContext context.Context, symbol string,
    ) (<-chan vo.LiveKCandleVo, error)
}
```

**一次連線，一個 channel。** 通道結束時 channel 關閉——這是「跟不動了」唯一的訊號，
呼叫端不必再去問狀態。

**它刻意不負責重連。** 重試間隔怎麼拉長、要不要放棄，是 PRD 白紙黑字的業務規則
（US-06.3），不是傳輸的調校參數。把重連塞進 Proxy，等於把一條可以用表格測試的規則
埋進一個只能靠真的斷線才測得到的地方。

### `LiveKCandleVo`（domain/models/vo）

來源送來的一根：交易標的、起始時間、五個價量數字，以及 `Closed bool`。

帶一個 `ToWriteDto()`，形狀與既有的 `MarketKCandleVo.ToWriteDto()` 一致——
**這是刻意的**：一根走完時，它就能原封不動走進既有的把關與存入路徑，
不必為跟盤另外寫一套規則檢查。規則只有一份，兩個入口共用。

### `KCandleFollowUpdateDto`（domain/models/dto）

觀看者收到的唯一形狀：

| 欄位 | 意義 |
| :--- | :--- |
| `Symbol` | 這是哪一個交易標的 |
| `Status` | `forming`（正在走的那一根）／ `closed`（這一根的最終樣子）／ `stalled`（跟不動了） |
| `KCandle` | 那一根的內容；`stalled` 時為零值 |

三種狀態是**觀看者能觀察到的全部**，與 PRD §5 的三種狀態一一對應。
`stalled` 是一種更新而不是一次錯誤——因為對看盤的人來說，
「即時停了」是一則必須送到的消息，不是一次失敗的請求。

### `KCandleFollowDomain`（domain/models/domains）★ 本切片的核心

**一份跟盤隨時間演變的全部規則。** 三個方法，三條 PRD 規則，各自可用表格測試：

| 方法 | 回答的問題 | 規則 |
| :--- | :--- | :--- |
| `Admit(live vo.LiveKCandleVo, now time.Time) bool` | 這一份現在要不要送出去？ | 走完的**一律送**；正在走的只在距離上次送出已滿更新間隔上限時才送 |
| `HasGoneQuiet(now time.Time) bool` | 這條通道是不是安靜地死了？ | 距離上次**收到**任何東西超過安靜門檻 |
| `NextRetryDelay() time.Duration` | 下一次重連等多久？ | 由一秒起逐次加倍，最長三十秒；成功連上即歸零 |

建構子把三個數字收進合理範圍（不大於零一律回到預設值），
因此**實例存在就代表這幾條規則是可用的**——與專案既有的建構子正規化慣例一致。

三個方法放同一個物件，是因為它們共用同一個變動理由：
**「一份跟盤該怎麼隨時間表現」**。分開會讓 service 自己去排序這三件事，
而那正是 shallow interface 的樣子。

### `KCandleFollowService`（domain/service）

**共享訂閱的生命週期。** 對外只有兩個方法：

```go
Watch(executionContext context.Context, symbol string) (<-chan dto.KCandleFollowUpdateDto, error)
Stop()
```

`Watch` 是**唯一**的入口，它一次回答了四個問題：這個交易標的有人在跟了嗎、
沒有的話開始跟、把這位觀看者加進去、他走的時候怎麼收尾。
呼叫端不必依序做四件事——這是 deep module 的判準。

內部：`map[string]*symbolFollow` 加一把 mutex。每個 `symbolFollow` 有自己的 goroutine，
跑「連線 → 讀 → 問 `KCandleFollowDomain` 要不要送 → 送給每個觀看者 →
走完的話存入」這個迴圈；連線結束就依 `NextRetryDelay()` 等待後重連。

觀看者的離開靠**他自己的 context**：`Watch` 收到的 context 結束時，
把他從那份跟盤移除；移除後沒人了，就取消那份跟盤的 goroutine 並從 map 刪掉。
**「連線斷掉即視為離開」因此不需要任何額外的判定**——它就是 context 結束。

### `KCandleFollowApplication`（application）

一個方法 `WatchKCandles(ctx, symbol)`，把 domain service 的 channel 原樣交給 controller。
薄，但它是分層要求的那一層：controller 不得認識 domain service。

### `KCandleFollowController`（controller）

唯一知道推播格式的地方。設定推播用的標頭、`for range` 那個 channel、
每收到一份就寫一個事件並 flush、channel 關閉或請求結束就返回。
**沒有任何 `if` 在判斷業務條件**——它拿到什麼就寫什麼。

### `BinanceLiveMarketDataProxy`（infrastructure/marketdata）

唯一知道來源即時通道長什麼樣子的地方：連線、訂閱、把 wire 格式正規化成 `LiveKCandleVo`、
通道結束時關閉 channel。與既有的 `BinanceMarketDataProxy` 並存——
兩者是同一個供應商的兩種取得方式，不是彼此的替代品。

### Depth check（deep-module 診斷）

| 診斷 | 結果 |
| :--- | :--- |
| 呼叫端需要自己排步驟嗎？ | 否。`Watch` 一次做完加入／開跟／收尾 |
| 介面名稱有 And／Then 嗎？ | 無 |
| 參數會不斷長大嗎？ | `Watch(ctx, symbol)` 兩個；日後要跟更粗的刻度會變三個，屆時收成 VO |
| 有沒有兩個類別共用一個變動理由？ | 節流／安靜／重試三條規則已合併在 `KCandleFollowDomain` |
| domain 認識傳輸嗎？ | 不認識。進來是 channel、出去是 channel |

---

## 4. Modified Components

### 新增的環境變數

| 變數 | 預設 | 用途 |
| :--- | :--- | :--- |
| `LIVE_UPDATE_INTERVAL_CEILING_SECONDS` | `10` | 更新間隔上限；不大於零時回到預設值 |
| `LIVE_FEED_QUIET_TIMEOUT_SECONDS` | `30` | 多久沒收到任何東西就當成跟不動 |
| `LIVE_FEED_MAX_RETRY_DELAY_SECONDS` | `30` | 重連間隔加倍的上限 |

安靜門檻取三十秒，是因為**誤判的代價只是白重連一次，漏判的代價是使用者盯著停格的圖**
（PRD 已定案：寧可誤判）。實務上這條通道即使毫無成交也會持續送出目前的樣子，
因此三十秒的安靜幾乎必然代表通道已死——但這條規則本身不依賴那個事實，
換一個來源只要調整這個數字。

### `dependencies.go` / `serve.go`

跟盤服務是**本專案第一個長生命週期的 domain service**（見 §8）。
它在組裝根建立、註冊進關機流程，關機時 `Stop()` 讓每一份跟盤收尾、每一個觀看者的 channel 關閉。

---

## 5. Component Relationships

```
瀏覽器 ──推播連線──▶ KCandleFollowController
                          │
                          ▼
                   KCandleFollowApplication
                          │
                          ▼
                   KCandleFollowService ────────┐
                    │        │                  │
                    │        ▼                  ▼
                    │  KCandleFollowDomain   IKCandleRepository.Save
                    │  （送不送／死沒死／等多久）   （走完的那一根，upsert）
                    ▼
             ILiveMarketDataProxy
                    ▲
                    │（實作）
        BinanceLiveMarketDataProxy ──持續連線──▶ 行情來源
```

### 執行順序 — 一個觀看者跟上

1. controller 收到請求，交給 application，拿回一個 channel。
2. service 查 map：沒人跟過就開一份，並啟動它的 goroutine；有人跟過就加入。
3. 該份跟盤把**目前進行中的那一根**（若有）立刻送給這位新觀看者。
4. controller 開始 `for range`，每一份寫成一個事件。

### 執行順序 — 市場有動靜

1. proxy 的 channel 送來一根 `LiveKCandleVo`。
2. 記下「收到了」（供 `HasGoneQuiet` 判斷）。
3. 問 `Admit`：不送就丟掉，送就分給每一個觀看者。
4. 若 `Closed`：走既有把關路徑 `NewKCandleDomain → ToEntity → Save`；
   違規則留下紀錄、不存，**跟盤本身繼續**。

### 執行順序 — 跟不動

1. proxy 的 channel 關閉，或 `HasGoneQuiet` 為真 → 主動切斷。
2. 送 `stalled` 給每一個觀看者。
3. 等 `NextRetryDelay()`，重連；連上就把間隔歸零，並重新送出目前的樣子。
4. **不放棄。** 觀看者全部離開才會停。

---

## 6. Extensibility & Handoff Notes

### 最可能的下一個需求

**「我想跟一小時一根的走勢」**——這是最可能的下一個要求，因為畫面本來就能看五種刻度，
而跟盤目前只送最細的那一種。

它會打在三個地方，而且**都是加欄位不是改結構**：
`LiveKCandleVo` 多一個刻度、`Watch` 的參數從兩個變三個（屆時收成一個
`KCandleFollowKeyVo`）、registry 的 key 從 `symbol` 變成那個 VO。
`KCandleFollowDomain` 的三條規則**一條都不用改**——它們與 K 線多粗無關。

**現在不預先做那個 key VO**：多一個只有一個欄位的型別，換來的是今天每一行都多繞一層。
接縫已經在對的位置（規則與傳輸都各自獨立），需求真的來時改三處，一個下午的事。

**「我想跟成交明細／掛單簿」**——這會是**另一個** Proxy 介面與另一個 follow service，
不是把 `ILiveMarketDataProxy` 撐大。它們唯一共用的是「共享訂閱 + 節流」這個形狀，
真的出現第二個時再抽共用，不要現在猜。

### 給下一個接手的人

- **不要把重連搬進 Proxy。** 它現在在 domain，是因為那三個數字是 PRD 寫下的業務規則，
  搬進 Proxy 就再也不能用表格測試了。
- **不要讓進行中的那一根進資料庫。** 這條規則有兩份文件背書（本切片與攝取切片），
  而且指標計算的正確性完全建立在它之上。
- **不要為了「效率」把每五分鐘那一輪關掉。** 它補的是沒人看時的每一根，
  以及來源事後的修正——兩件跟盤做不到的事。

---

## 7. Traceability

| PRD 情境 | 由誰滿足 |
| :--- | :--- |
| US-01.1 第一個觀看者開始跟盤 | `KCandleFollowService.Watch`（map 查無 → 開一份） |
| US-01.2 多人在看只跟一份 | `KCandleFollowService.Watch`（map 查有 → 加入） |
| US-01.3 還有人在看就繼續跟 | `symbolFollow` 的觀看者集合非空即不收 |
| US-01.4 最後一個離開就停 | 觀看者集合清空 → 取消 goroutine、從 map 移除 |
| US-01.5 不在觀察清單上也跟得動 | `Watch` 只認 symbol，完全不讀觀察清單 |
| US-01.6 換看另一個就換跟另一個 | 舊 context 結束 → 離開舊的；新的 `Watch` → 加入新的 |
| US-01.7 連線斷掉就算離開 | 請求 context 結束即離開，無額外判定 |
| US-02.1 剛跟上先給進行中的那一根 | `Watch` 回傳前先送出該份跟盤持有的最新一根 |
| US-02.2 尚無成交時不給、也不算失敗 | 該份跟盤還沒持有任何一根 → 不送，`Watch` 正常回傳 |
| US-02.3 送出的一律是五分鐘一根 | `BinanceLiveMarketDataProxy` 只訂閱五分鐘一根 |
| US-03.1 十秒內上百筆只送一次 | `KCandleFollowDomain.Admit` |
| US-03.2 未滿上限先不送 | 同上 |
| US-03.3 一根走完不等滿十秒 | 同上（`Closed` 一律 admit） |
| US-03.4 沒有變動就不送 | 沒有輸入就不會呼叫 `Admit` |
| US-04.1 進行中的那一根查不到 | 不呼叫 `Save`——靠不存達成，不靠額外判斷 |
| US-04.2 指標計算不使用它 | 同上；計算讀資料庫，資料庫沒有它 |
| US-04.3 走完之後查得到 | `Closed` 時走既有存入路徑 |
| US-05.1 走完的當下就存入 | `symbolFollow` 收到 `Closed` 即存 |
| US-05.2 事後修正照樣覆蓋 | `KCandleRepository.Save` 的 upsert（既有，不改） |
| US-05.3 沒人看時由自動抓取補上 | `KCandleIngestionJob`（既有，不改） |
| US-05.4 違規不存並留下紀錄 | `NewKCandleDomain` 回錯 → 記錄後 continue |
| US-06.1 中斷時明說已停止 | 送出 `stalled` 更新 |
| US-06.2 重新跟上給現在的樣子、不補播 | 重連後只送當下那一根；中斷期間的輸入從未被保留 |
| US-06.3 間隔逐次拉長、最長三十秒、不放棄 | `KCandleFollowDomain.NextRetryDelay` |
| US-06.4 中斷期間走完的不會遺失 | `KCandleIngestionJob`（既有，不改） |
| US-06.5 即時不能用時其他功能照常 | 跟盤獨立於既有每一條路徑；失敗只影響跟盤自己 |

---

## 8. Risks & Open Decisions

### 刻意偏離既有規範

**`KCandleFollowService` 是本專案第一個「有狀態且長生命週期」的 domain service。**
既有的 service 全部是無狀態的：進去、算完、回來。這一個持有一份註冊表、
一把鎖，以及每個被跟標的一個 goroutine。

**為什麼還是放在 domain service：** 它管的是「誰在看什麼、跟幾份、何時停」——
這是不折不扣的業務規則（PRD US-01 整段）。放進 application 會讓那一層開始持有狀態，
放進 controller 會讓它認識傳輸以外的事。它留在 domain，代價是這一層第一次出現生命週期，
換來的是規則沒有離開它該在的地方。

**測試上的代價：** 它不能像既有 service 那樣純函式式地測。
因此把**所有帶數字的規則抽進 `KCandleFollowDomain`**，讓那一部分回到表格測試；
service 本身只測生命週期（開／加入／離開／最後一個離開就停）。

### Risks / trade-offs

- **兩個寫入者，後寫的贏。** 跟盤存下正確值後，若自動抓取取回的是來源尚未修正完的數字，
  它會蓋掉。下一輪會再修正回來，因此影響是暫時的——這是 upsert 換來簡單性的代價，
  接受它。
- **安靜門檻是猜的。** 三十秒沒有理論依據，只有「誤判便宜、漏判昂貴」這個取捨。
  若日後出現頻繁的假重連，調大它即可；它是設定值不是常數，正是為此。
- **每個被跟的標的一個 goroutine 加一條外部連線。** 不設上限（PRD 已定案）。
  個人使用的系統同時在看的標的天生極少；真的撞到來源限制時，
  症狀是某些標的持續跟不動，走既有的中斷路徑，不會讓系統壞掉。

### Open decisions（留給實作階段解決）

- 推播的事件格式（每個事件的欄位命名）——屬於 controller 內部，
  與前端切片一起定案即可，不影響本設計的任何一層。
- 跟盤違規紀錄的落地方式（目前先與攝取切片一致，寫進 log）。
