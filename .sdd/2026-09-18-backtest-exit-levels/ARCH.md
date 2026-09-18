# 回測的止損止盈 — Architecture Design

**切片**：`2026-09-18-backtest-exit-levels`
**依據**：[BRIEF.md](BRIEF.md)、[PRD.md](PRD.md)
**範圍**：`go-trading`（後端）。前端與 MCP 各自有自己的切片文件。

---

## 1. Design Goal & Guiding Principle

### 一句話

**一根 K 線多問一句「有沒有碰到出場價位」，而那句話問在信號之前。**

### 三個支配這一刀的決定

#### 決定一：「進場那一棒不檢查」用**順序**表達，不用比對

最容易寫出來的版本是：檢查時比一下 `entryTime` 是不是這一棒。
那個版本會在兩個地方壞掉——時區、以及同一秒開盤的兩根棒——
而且壞掉的時候不會報錯，只會多掃出場一筆。

真正的實作是把檢查放在**套用信號之前**：

```
第 N 棒： 1. 檢查出場價位   ← 此時第 N 棒的部位還不存在
          2. 套用第 N 棒的信號  ← 部位在這裡誕生
第 N+1 棒：1. 檢查出場價位   ← 它第一次被檢查
```

**那條規則不是一個判斷，它就是這個順序。** 沒有比對、沒有旗標、沒有
「這是不是進場那一棒」這個問題。實作裡找不到那條規則的程式碼，
因為它不是程式碼，它是排列。

#### 決定二：出場價位在**進場當下算一次**

與單位數同一個位置、同一個理由。每一棒重算等於每一棒做一次乘法，
而某一次乘法捨入得跟前一次不一樣的那天，一個沒有動過的部位會看起來在漂。

#### 決定三：止損出場與訊號出場走**同一條平倉路徑**

一筆已平倉交易的方向、單位數、損益、計價算法**完全相同**，
只有「成交在哪個價格」與「為什麼出場」不同。
所以這一刀**不新增第二條平倉路徑**，只給既有那一條多一個參數（原因）
與一個新的呼叫點。

兩條路徑的代價很具體：勝率與最大回落會在某一天只認得其中一條。

---

## 2. Change Scope

### 新增（4 個檔案）

| 檔案 | 角色 | 為什麼是新的 |
|:---|:---|:---|
| `domain/models/vo/trade_exit_reason_vo.go` | VO | 三個取值的列舉，純資料無行為 |
| `domain/models/vo/exit_prices_vo.go` | VO | 一個部位進場當下算好的兩個出場價位＋各自有沒有。純資料 |
| `domain/models/domains/backtest_exit_levels_domain.go` | Domain Model | 這一次重演的那兩個距離：驗證，以及**把價位放在進場價的哪一邊** |
| `domain/models/domains/tests/backtest_exit_levels_domain_test.go` | 測試 | 四個方向的價位各一組數字 |

### 修改（9 個檔案）

| 檔案 | 改什麼 |
|:---|:---|
| `domain/models/dto/backtest_request_dto.go` | 多兩個距離 |
| `domain/models/dto/trading_strategy_backtest_request_dto.go` | 多兩個距離，並在 `ToBacktestRequestDto` 帶過去 |
| `domain/models/domains/backtest_domain.go` | 驗證那兩個距離、持有出場價位、交給模擬 |
| `domain/models/domains/backtest_errors.go` | 多一個欄位名常數 |
| `domain/models/domains/backtest_position_domain.go` | 持有自己的出場價位；回答「這一棒把我掃出場了嗎」；平倉帶原因 |
| `domain/models/domains/backtest_account_domain.go` | 多一個「先檢查價位」的入口；平倉路徑抽成共用；數得出兩種出場筆數 |
| `domain/models/domains/backtest_simulation_domain.go` | 走訪多一步（在信號之前）；成績單多兩個數字 |
| `domain/models/vo/closed_trade_vo.go` | 多一個出場原因 |
| `domain/models/dto/closed_trade_dto.go`、`dto/backtest_summary_dto.go` | 對外多三個欄位 |
| `controller/models/backtest_request.go`、`controller/models/trading_strategy_backtest_request.go` | 收那兩個距離 |

### 一個字都不改

- 押多少的三種模式、交易模式、資金曲線、最大回落、勝率的算法。
- **既有的每一條回測測試。** 寫在這一刀之前的斷言是「沒有東西移位」的唯一證據；
  改了任何一條，這一刀就再也證明不了相容性。

---

## 3. New Classes / Modules

### `BacktestExitLevelsDomain`

```go
type BacktestExitLevelsDomain struct {
    stopLoss   decimal.Decimal   // 百分點；零＝不模擬
    takeProfit decimal.Decimal
}

func NewBacktestExitLevelsDomain(
    stopLoss decimal.Decimal, takeProfit decimal.Decimal,
) (BacktestExitLevelsDomain, error)

// PricesFrom 把兩個價位放在一個進場價的兩側。
func (d BacktestExitLevelsDomain) PricesFrom(
    direction vo.PositionDirectionVo, entryPrice decimal.Decimal,
) vo.ExitPricesVo
```

**零值＝不模擬任何出場**，與部位規劃同一個手法：沒有一個「要不要模擬」的旗標
可以跟那兩個數字互相矛盾。零距離就是沒有那個出場，而那個事實不可能自我矛盾。

**兩句拒絕的話直接沿用既有的 `validatedDistance`**（同一個 package，
機器人部位規劃上一刀寫的）。不複製，因為同一個 `150` 在兩張表單上必須
得到同一句話——而複製出來的第二份，會在第一次有人改措辭的那天分岔。

### `ExitPricesVo`

```go
type ExitPricesVo struct {
    StopLossPrice   decimal.Decimal
    HasStopLoss     bool
    TakeProfitPrice decimal.Decimal
    HasTakeProfit   bool
}
```

一個部位進場當下算好就不動的那兩個價位。純資料——
「碰到了沒有」要看方向，而方向是部位的。

### `TradeExitReasonVo`

```go
type TradeExitReasonVo string

const (
    TradeExitReasonSignal     TradeExitReasonVo = "signal"
    TradeExitReasonStopLoss   TradeExitReasonVo = "stopLoss"
    TradeExitReasonTakeProfit TradeExitReasonVo = "takeProfit"
)
```

---

## 4. Modified Components

### `BacktestPositionDomain` — 多持有兩個價位，多回答一個問題

```go
type BacktestPositionDomain struct {
    direction  vo.PositionDirectionVo
    entryTime  time.Time
    entryPrice decimal.Decimal
    stake      decimal.Decimal
    unitCount  decimal.Decimal
    exitPrices vo.ExitPricesVo   // ← 新增，與 unitCount 同一時刻算好
}

func NewBacktestPositionDomain(
    direction vo.PositionDirectionVo,
    entryTime time.Time,
    entryPrice decimal.Decimal,
    stake decimal.Decimal,
    exitLevels BacktestExitLevelsDomain,   // ← 新增
) (BacktestPositionDomain, bool)

// ExitOn 是這一棒逼出來的那一筆已平倉交易，如果它逼出了一筆。
func (p BacktestPositionDomain) ExitOn(
    kCandle vo.KCandleVo, exitTime time.Time,
) (vo.ClosedTradeVo, bool)

// ClosedAt 多一個原因參數
func (p BacktestPositionDomain) ClosedAt(
    exitTime time.Time, exitPrice decimal.Decimal, reason vo.TradeExitReasonVo,
) vo.ClosedTradeVo
```

**`ExitOn` 直接回傳一筆已平倉交易**，而不是「價格＋原因＋有沒有」三個回傳值。
理由：帳戶拿到它之後要做的三件事——記進明細、把錢加回可用資金、數一筆——
讀的都是這一個形狀上的欄位（`ExitPrice`、`ExitReason`）。
回傳三個值等於讓呼叫端自己把它們拼回一筆交易，而那是這裡已經會做的事。

**方向的推理出現在兩個地方**，各一行，刻意的：

| 誰 | 推理什麼 |
|:---|:---|
| `BacktestExitLevelsDomain.PricesFrom` | 價位**放在哪一邊**（做多止損在下） |
| `BacktestPositionDomain.ExitOn` | 什麼**碰得到**它（在下面的，用最低價碰） |

兩者都要看方向，但它們是兩個不同的問題，而第二個必須在部位身上——
部位是唯一同時握著方向與那兩個價位的東西。兩處各留一行註解指向對方。

### `BacktestAccountDomain` — 多一個入口，平倉路徑抽成共用

```go
func NewBacktestAccountDomain(
    initialCapital decimal.Decimal,
    positionSizing PositionSizingDomain,
    tradingMode TradingModeDomain,
    exitLevels BacktestExitLevelsDomain,   // ← 新增，開倉時交給部位
) *BacktestAccountDomain

// ApplyExitLevels 在這一棒的信號之前問一次：這一棒把手上那一注掃出場了嗎。
func (a *BacktestAccountDomain) ApplyExitLevels(kCandle vo.KCandleVo, candleTime time.Time)

// ExitCountFor 是以某個原因結束的筆數。
func (a *BacktestAccountDomain) ExitCountFor(reason vo.TradeExitReasonVo) int
```

**兩個出場筆數不是兩個計數器，而是從明細數出來的。**
與勝率同一個手法：多兩個欄位就是多兩個會跟明細不一致的機會，
而不一致的那一天沒有人說得出該相信哪一個。

**平倉那四行（記明細、加回現金、放掉部位）現在有兩個公開呼叫者**
（`Apply` 與 `ApplyExitLevels`），所以抽成一個私有 helper 是符合門檻的
——不是為了看起來整齊，是因為兩邊漏掉其中一行的後果是錢憑空出現或消失。

### `BacktestSimulationDomain` — 走訪多一步

```go
for candleIndex, inputKCandle := range backtestSimulationDomain.inputKCandles {
    fillPrice := decimal.NewFromFloat(inputKCandle.Close)
    candleTime := time.Unix(inputKCandle.OpenTimeUnixSeconds, 0).UTC()

    // 先問價位，再問信號。這個順序就是「進場那一棒不檢查」那條規則本身：
    // 這一棒開的部位在下一行才誕生，所以它第一次被檢查是下一棒。
    account.ApplyExitLevels(inputKCandle, candleTime)
    account.Apply(backtestSimulationDomain.signals[candleIndex], candleTime, fillPrice)

    equityCurve.Record(candleTime, account.EquityAt(fillPrice))
}
```

成績單多兩個數字，讀自 `account.ExitCountFor(...)`。

### 那兩個距離怎麼走到模擬

```
BacktestRequest（HTTP body）
  └─ ToRequestDto ──▶ BacktestRequestDto（多兩格）
                        │
TradingStrategyBacktestRequestDto（多兩格）
  └─ ToBacktestRequestDto ──┘      ← 兩種重演在這裡合流，之後只有一條路
                        │
                        ▼
                  NewBacktestDomain
                    └─ NewBacktestExitLevelsDomain（驗證）
                         └─ 失敗 → BacktestValidationFailure(BacktestExitLevelsField, 句子)
                        │
                        ▼
                  NewBacktestSimulationDomain
                        │
                        ▼
                  NewBacktestAccountDomain
                        │
                        ▼（開倉時）
                  NewBacktestPositionDomain → PricesFrom → ExitPricesVo
```

**兩種重演在 `ToBacktestRequestDto` 就合流了**，所以
「腳本重演與交易策略重演行為一字不差」不是兩處各寫一次的結果，
而是它們之後只走一條路。

### 拒絕指向哪一格

```go
const BacktestExitLevelsField = "exitLevels"
```

**一個名字蓋住兩格**，句子本身說出是止損還是止盈。
與既有的 `BacktestTimeRangeField` 同一個判斷（它一個名字蓋住兩個時刻加一個刻度）。
多一個常數只是把句子裡已經有的那個詞再說一次，
而代價是前端要多維護一組對照。

---

## 5. Component Relationships

```
        ┌──────────────────────────────┐
        │ BacktestExitLevelsDomain     │  這一次的兩個距離
        │  · 驗證（沿用 validatedDistance）│
        │  · PricesFrom(方向, 進場價)   │  ← 價位放在哪一邊
        └──────────────┬───────────────┘
                       │ 開倉當下算一次
                       ▼
        ┌──────────────────────────────┐
        │ BacktestPositionDomain       │
        │  · exitPrices（不動）         │
        │  · ExitOn(K線) → 已平倉交易   │  ← 什麼碰得到它
        └──────────────┬───────────────┘
                       │
                       ▼
        ┌──────────────────────────────┐
        │ BacktestAccountDomain        │
        │  · ApplyExitLevels（先）      │
        │  · Apply（後，一行未改）       │
        │  · ExitCountFor（數明細）      │
        └──────────────────────────────┘
```

---

## 6. Traceability

| AC | 落在哪裡 |
|:---|:---|
| AC-01.1〜01.4（選填、留白不計） | `BacktestExitLevelsDomain` 零值；`PricesFrom` 對零距離不給價位 |
| AC-01.5（兩種重演一致） | `ToBacktestRequestDto` 合流，之後單一路徑 |
| AC-01.6〜01.8（拒絕與 100 允許） | `validatedDistance`（沿用，措辭不動） |
| AC-01.9（欄位名） | `BacktestExitLevelsField` |
| AC-02.1〜02.9（怎麼判、四個方向） | `PricesFrom` ＋ `ExitOn` |
| AC-03.1（進場那一棒不檢查） | **模擬走訪的順序**，非判斷 |
| AC-03.2（兩個都碰到算止損） | `ExitOn` 先問止損 |
| AC-03.3〜03.4（出場後信號照常判） | `ApplyExitLevels` 與 `Apply` 是兩次獨立呼叫 |
| AC-03.5（重新進場重算價位） | 新部位在建構時自己算 |
| AC-03.6（順序） | 同 AC-03.1 |
| AC-04.1〜04.5（原因與筆數） | `ClosedTradeVo.ExitReason`、`ExitCountFor` |
| AC-04.6（其餘數字一併反映） | 它們讀的是同一段歷史，不需要改 |

---

## 7. Extensibility & Handoff Notes

- **移動停損**進來時，動的是 `ExitPricesVo` 從「進場算一次」變成
  「每一棒可能往有利方向推一次」——那會讓它從 VO 變成一個 Domain Model。
  那一刀的第一個動作就是那個轉換，其餘不必動。
- **下一棒開盤成交**進來時，改的仍然只有模擬走訪裡那一行 `fillPrice`
  （既有註解已經標好了）。出場成交價不受影響——它成交在價位本身。
- **用細 K 線還原盤中順序**進來時，改的是 `ExitOn` 的參數（一根變一串）
  與「兩個都碰到算止損」那一行。那條規則屆時會有真正的答案，
  而它現在只在一個地方。

---

## 8. Appendix — 驗算

以初始資金 10000、全押、第一棒收 100 為基準（100 單位）：

| 情境 | 止損價 | 止盈價 | 第二棒 | 出場價 | 帳上 |
|:---|---:|---:|:---|---:|---:|
| 做多、止損 2% | 98 | — | 高 103 低 97 | **98** | **9800** |
| 做多、止損 2% | 98 | — | 高 103 低 98 | **98** | **9800** |
| 做多、止損 2% | 98 | — | 高 103 低 99 | 未出場 | — |
| 做多、止盈 5% | — | 105 | 高 106 低 99 | **105** | **10500** |
| 做多、兩者都設 | 98 | 105 | 高 106 低 97 | **98（止損勝）** | **9800** |
| 做多、都不設 | — | — | 高 103 低 97；第三棒收 110 | 未出場 | **11000** |
| 做空、止損 2% | 102 | — | 高 103 | **102** | **9800** |
| 做空、止盈 5% | — | 95 | 低 94 | **95** | **10500** |
