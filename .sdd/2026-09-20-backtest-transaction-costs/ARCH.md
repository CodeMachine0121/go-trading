# 回測的交易成本 — Architecture Design

**切片**：`2026-09-20-backtest-transaction-costs`
**規格來源**：[PRD.md](PRD.md)（Acceptance Criteria 為 oracle）
**共識來源**：[BRIEF.md](BRIEF.md)
**範本切片**：`2026-09-18-backtest-exit-levels`（形狀幾乎一模一樣：可選的新參數、留白即不計、
一個新 Domain Model、一個新的成績單數字）

---

## 1. Design Goal & Guiding Principle

### 一句話

**成本是一個只會做算術的 Domain Model，它算出來的數字由既有的那兩條路徑（開倉、平倉）
去扣，而「付得起嗎」這個既有問題多吃一個參數。**

### 四個支配這一刀的決定

#### 決定一：成本模型**只做算術，不碰帳戶**

`BacktestTransactionCostsDomain` 拿兩個百分點，回答三個純算術問題：
可押上限是多少、這筆押注的進場成本是多少、這筆成交金額的出場成本是多少。

它**不知道**有帳戶、有現金、有部位。這讓每一條規則都能用一張數字表釘死，
也讓「錢什麼時候少」這件事只在帳戶那一個地方發生——與 `BacktestExitLevelsDomain`
只回答「兩個價位在哪」而不回答「有沒有碰到」是同一個切法。

**零值就是不計。** 費率兩個零 → 可押上限等於可用資金、兩筆成本都是零。
這不是一個旗標，是**算術本身**：`可用資金 ÷ (1 + 0)` 就是可用資金。
旗標會與費率互相矛盾，而沒有人說得出該信哪一個。

#### 決定二：出場成本掛在**倉位**身上，因為只有它知道成交金額

出場成本要乘的是**成交金額 ＝ 單位數 × 出場價**。

單位數只有 `BacktestPositionDomain` 有。而帳戶手上唯一現成的數字是
`ValueAt(exitPrice)`——那是「這一注值多少」，做多時剛好等於成交金額，
**做空時不等於**（放空一百單位、進場 100、出場 90：成交 9000，那注值 11000）。

在帳戶算成本，就會寫出一個**只有做空會錯、而且從數字上完全看不出來**的乘法。
所以成本的算術跟著倉位走，帳戶只負責把算好的數字扣掉。

#### 決定三：「付得起嗎」是**既有的**問題，不是新的

`PositionSizingDomain.StakeFor` 現在就同時回答「押多少」與「押得起嗎」。
成本沒有新增一道檢查，它**只是讓第二個答案多吃一個參數**：

```
押得起 ⟺ 押的金額 + 它的進場成本 ≤ 可用資金
        ⟺ 押的金額 ≤ 可用資金 ÷ (1 + 進場成本率)
        ⟺ 押的金額 ≤ 可押上限
```

三種模式因此共用**同一個**判斷，而不是每一種長出自己的邊界。
不計成本時可押上限就是可用資金本身——**既有的每一條規則與每一個測試一個字都不用改**。

`StakeFor` 收的是**成本模型本身**，不是一個算好的上限數字。
理由是第二個呼叫者：機器人的部位計畫傳 `BacktestTransactionCostsDomain{}`（零值），
而那行程式碼因此自己說出了「機器人不計成本」這個**刻意的**決定。
傳一個算好的數字會讓同一件事讀起來像巧合。

#### 決定四：估值維持毛額，**只有平倉才扣出場成本**

`ValueAt(price)` 一個字都不改。它是資金曲線每一點的來源，
而出場成本**還沒付**——預扣它等於讓曲線描述一件沒發生的事。

真正扣錢的只有 `settleOpenPosition`，它是**兩條出場路徑（訊號、價位）唯一共用的那條**。
既有註解已經寫明「兩條路徑都必須做完整三件事」；這一刀把三件變成四件，
而因為只有一份程式碼，不可能只補一邊。

---

## 2. Change Scope

### 新增（2 個檔案）

| 檔案 | 內容 |
|:---|:---|
| `internal/domain/models/domains/backtest_transaction_costs_domain.go` | `BacktestTransactionCostsDomain` ＋ `validatedCostPercentage` |
| `internal/domain/models/domains/tests/backtest_transaction_costs_domain_test.go` | 該 model 的單元測試 |
| `internal/domain/models/domains/tests/backtest_transaction_cost_simulation_test.go` | 走完整條模擬路徑的數字表（比照 `backtest_exit_simulation_test.go`） |

### 修改（14 個檔案）

| 檔案 | 改什麼 |
|:---|:---|
| `domains/backtest_position_domain.go` | 多持有成本模型與自己的進場成本；`ClosedAt` 算出場成本並把損益改成淨額 |
| `domains/backtest_account_domain.go` | 多持有成本模型；開倉多扣進場成本、平倉多扣出場成本；多一個累計成本的讀數 |
| `domains/position_sizing_domain.go` | `StakeFor` 多收成本模型，用可押上限回答「押得起嗎」 |
| `domains/position_plan_domain.go` | 呼叫端傳零值成本模型（機器人維持現狀，且讓這件事寫在程式碼上） |
| `domains/backtest_simulation_domain.go` | 多持有成本模型並傳給帳戶；成績單多一個累計成本 |
| `domains/backtest_domain.go` | 建構成本模型、把拒絕指到新的欄位名、傳進模擬 |
| `domains/backtest_errors.go` | 多一個欄位名常數 `transactionCosts` |
| `vo/closed_trade_vo.go` | 多兩個成本欄位；`Profit` 語意改為淨額 |
| `dto/closed_trade_dto.go` | 多兩個成本欄位 |
| `dto/backtest_summary_dto.go` | 多一個累計成本 |
| `dto/backtest_request_dto.go` | 多兩個費率 |
| `dto/trading_strategy_backtest_request_dto.go` | 多兩個費率，並在轉接處帶過去 |
| `controller/models/backtest_request.go` | 多兩個費率 |
| `controller/models/trading_strategy_backtest_request.go` | 多兩個費率 |
| `application/assistantqueries/trading_strategy_backtest_assistant_query.go` | 多兩個費率**與兩個出場距離**（後者是補齊先前的遺漏） |
| `postman/go-trading.postman_collection.json` | 兩個重演請求各補兩格 |

### 一個字都不改

| 元件 | 為什麼 |
|:---|:---|
| `BacktestExitLevelsDomain`、`ExitPricesVo`、`TradeExitReasonVo` | 成本在「哪一棒出場、以什麼價位出場」決定**之後**才發生，不影響任何一個出場判斷 |
| `BacktestPositionDomain.ValueAt` / `ProfitAt` | 估值維持毛額（決定四）。`ProfitAt` 是價差，成本在 `ClosedAt` 才扣 |
| `BacktestEquityCurveDomain` | 它收到的數字自然變了，它的規則沒變 |
| `TradingStrategyBacktestDomain` | 它把每一件關於錢的事都委派給 `BacktestDomain`。**兩種重演一字不差是這個委派的結果，不是抄兩遍的結果** |
| `TradingModeDomain`、`SignalDomain` | 與成本無關 |
| 機器人的建議部位（`PositionPlanDomain` 的算法） | 刻意不動（PRD Out of Scope）；只有呼叫 `StakeFor` 那一行多一個零值參數 |

---

## 3. New Classes / Modules

### `BacktestTransactionCostsDomain`

```
內部狀態
  entryCostPercentage   decimal   零＝進場不收費
  exitCostPercentage    decimal   建構後已解析（留白時等於進場）

建構
  NewBacktestTransactionCostsDomain(entry, exit) (model, error)
    1. 兩格各自驗證（負 → 拒絕；> 100 → 拒絕；正好 100 → 放行）
       驗證看的是**宣告的原值**，所以 101 與 -1 在沿用之前就被擋下
    2. 出場為零 → 沿用進場        ← 正規化寫在建構子裡，符合既有慣例

行為（全部是純算術）
  MaximumStakeFrom(availableCash) → availableCash ÷ (1 + entry/100)
  EntryCostFor(stake)             → stake × entry/100
  ExitCostFor(tradedNotional)     → |tradedNotional × exit/100|
```

**為什麼 `ExitCostFor` 取絕對值。** 成本永遠是支出（PRD R-6）。
成交金額由 K 線的價格算出來，而那是外部餵進來的浮點數；一個負價格會讓成本變成一筆收入，
那是一種**會讓成績單變好看、又完全不會報錯**的錯。取絕對值是一行，讓這件事不可能發生。

**為什麼零值可用。** `MaximumStakeFrom` 在零值時是 `cash ÷ 1`，兩個 `CostFor` 都回零。
既有的每一條路徑傳零值進來，行為與這一刀之前**逐位元相同**。

**`validatedCostPercentage` 是 package-level 函式**，比照同一個 package 既有的
`validatedDistance` / `movedBy` / `reachedBy`。它必須在建構完成**之前**跑，
所以掛不到 model 的 method 上；兩格共用一份，是為了讓同一個 101 在兩格得到同一句話。
措辭與 `validatedDistance` **刻意不同**：距離超過 100 的問題是「價格會變成負數」，
費率超過 100 的問題是「成本不會超過成交金額本身」——兩句話講的是兩件事。

---

## 4. Modified Components

### `BacktestPositionDomain` — 多知道兩件事

```
多持有
  transactionCosts  成本模型（與 exitLevels 對稱：進場當下拿到，一路跟著這一注）
  entryCost         這一注**已經付掉**的進場成本（建構時算一次，之後不再重算）

建構子多一個參數
  NewBacktestPositionDomain(direction, entryTime, entryPrice, stake, exitLevels, transactionCosts)
    entryCost = transactionCosts.EntryCostFor(stake)

多一個讀數
  EntryCost() → 帳戶開倉時要扣的那個數字，也是累計成本要算進去的那一筆

ClosedAt 多做兩件事
  exitCost = transactionCosts.ExitCostFor(unitCount × exitPrice)
  Profit   = ProfitAt(exitPrice) − entryCost − exitCost     ← 淨額
  回傳的 ClosedTradeVo 帶上 EntryCost 與 ExitCost
```

**進場成本只算一次並記下來**，理由與單位數一模一樣：重算一次就是多一個會與前一次不一致的除法。
它也是「已經付掉」這個事實的唯一載體——累計成本要問未平倉那一注付過多少，問的就是它。

**出場成本在 `ClosedAt` 內聯算**，不抽 private helper：只有這一個呼叫點，
而規則說只被一個公開方法用到的 private 一律內聯。帳戶要用到它時，
讀的是 `ClosedTradeVo.ExitCost`——那個值已經在那張已完成的單子上了。

### `BacktestAccountDomain` — 兩條既有路徑各多扣一筆

```
多持有  transactionCosts

Apply（開倉那一段）
  stake, canStake = positionSizing.StakeFor(availableCash, transactionCosts)
  if !canStake → 跳過（既有規則，一字不改）
  openedPosition, isOpened = NewBacktestPositionDomain(..., transactionCosts)
  availableCash −= stake + openedPosition.EntryCost()      ← 多減一筆

settleOpenPosition（唯一的平倉路徑）
  availableCash += openPosition.ValueAt(exitPrice) − closedTrade.ExitCost   ← 多減一筆

多一個讀數
  TotalTransactionCost()
    = Σ(已平倉交易的 EntryCost + ExitCost)
    + (還開著的話) openPosition.EntryCost()
```

**累計成本從清單上數出來，不另外累加一個計數器。** 與 `ExitCountFor`、`WinRate`
同一個理由：計數器是同一個事實的第二個住處，哪一天它與清單對不起來，
沒有人說得出該信哪一個。

### `PositionSizingDomain.StakeFor` — 第二個答案多吃一個參數

```
StakeFor(availableCash, transactionCosts) (stake, canStake)

  maximumStake = transactionCosts.MaximumStakeFrom(availableCash)

  固定金額 → stake = 那個數
  全押     → stake = maximumStake          ← 全押的意思：錢全部變成「這筆交易」
  百分比   → stake = availableCash × v/100  ← 意思不變：可用資金的幾成

  canStake = stake > 0 且 stake ≤ maximumStake
```

**三種模式一個判斷。** 全押恆成立（它押的就是上限本身）；
另外兩種模式使用者講的是一個明確的數字，那個數字連同它的成本付不起，就跳過這次。

**百分比填 100 與全押從此不同**，而這是對的：全押沒有數字、它會自己縮到付得起；
百分比 100 是使用者**打出來的一個數字**，付不起它與它的成本就跳過——
與固定金額正好等於可用資金完全一致。

### `BacktestDomain` — 多建一個 model、多一個欄位名

```
transactionCosts, err = NewBacktestTransactionCostsDomain(
        requestDto.EntryCostPercentage, requestDto.ExitCostPercentage)
if err != nil → BacktestValidationFailure(BacktestTransactionCostsField, err.Error())
```

拒絕指向**一個**欄位名 `transactionCosts`，由句子本身說出是進場那格還是出場那格——
與 `exitLevels` 一字不差的判斷：這兩格在每一個畫面上都是併排填的一組，
第二個常數只會重複一個句子已經說過的詞，還讓每個呼叫端多一份對照要維護。

### 那兩個費率怎麼走到模擬

```
重演一支策略腳本
  BacktestRequest（controller）
    └→ BacktestRequestDto ─────────────────────┐
                                               ├→ BacktestDomain
重演一份交易策略                                  │     └→ BacktestTransactionCostsDomain
  TradingStrategyBacktestRequest（controller）  │           └→ BacktestSimulationDomain
    └→ TradingStrategyBacktestRequestDto        │                 └→ BacktestAccountDomain
          └→ .ToBacktestRequestDto() ───────────┤                       └→ BacktestPositionDomain
                                               │
助手發動的重演                                    │
  tradingStrategyBacktestAssistantArguments     │
    └→ TradingStrategyBacktestRequestDto ───────┘
          └→ .ToBacktestRequestDto()
```

三個入口、一條轉接，**匯流之後只有一條路**。
`ToBacktestRequestDto` 是唯一會被遺忘而且不會編譯失敗的地方——
漏掉那兩行，交易策略重演會安靜地讀成「不計成本」。它自己的註解已經寫明這個風險。

**助手那條同時補上止損距離與止盈距離**：它現在連那兩格都沒有。
那不是刻意的取捨（它的註解逐條解釋了為什麼沒有刻度、沒有腳本、沒有交易模式，
卻對出場距離隻字未提），而出場距離恰好是交易策略**不會**說的東西。
三個入口只要有一個少幾格，同樣的設定經由不同路徑就會跑出不同的成績單。

---

## 5. Component Relationships

```
BacktestSimulationDomain
   │  持有 transactionCosts，交給帳戶
   ▼
BacktestAccountDomain ──────────────┐
   │  開倉：扣 stake + 進場成本        │ 問「押得起嗎」
   │  平倉：加 ValueAt − 出場成本      ▼
   │                          PositionSizingDomain
   │                                 │ 問「可押上限」
   ▼                                 ▼
BacktestPositionDomain ────▶ BacktestTransactionCostsDomain
      算自己的進場成本                （純算術，不認識任何人）
      算平倉那一刻的出場成本
      交出淨額損益
```

**依賴方向全部朝向那個只會算術的 model。** 它不 import 任何東西，
也因此每一條成本規則都能用一張數字表單獨釘死，不必啟動一次模擬。

---

## 6. Traceability

| AC | 落在哪裡 |
|:---|:---|
| AC-01.1 / 01.8 | `NewBacktestTransactionCostsDomain` 的驗證 |
| AC-01.2 | 零值成本模型；既有測試一條斷言都不改，就是這一條的迴歸網 |
| AC-01.3 | 建構子內「出場為零 → 沿用進場」 |
| AC-01.4 | 兩格各自獨立驗證，進場零即不收 |
| AC-01.5 | `ToBacktestRequestDto` 帶過去，之後單一路徑 |
| AC-01.6 / 01.7 | `validatedCostPercentage` |
| AC-02.1 | `Apply` 的 `availableCash −= stake + EntryCost()` |
| AC-02.2 | `settleOpenPosition` 的 `− closedTrade.ExitCost` |
| AC-02.3 | `StakeFor` 全押回 `maximumStake` |
| AC-02.4 | `StakeFor` 百分比仍乘 `availableCash` |
| AC-02.5 | `canStake` 的 `stake ≤ maximumStake` |
| AC-03.1 | `EntryCostFor(stake)` |
| AC-03.2 / 03.3 | `ClosedAt` 用 `unitCount × exitPrice`，不用 `ValueAt` |
| AC-03.4 | `ExitCostFor` 的絕對值 |
| AC-04.1〜04.4 | `ClosedAt` 的淨額；`ClosedTradeVo.IsWin` 讀它，勝率自動跟著變 |
| AC-04.5 | `ClosedTradeVo` / `ClosedTradeDto` 的兩個欄位 |
| AC-05.1〜05.3 | `TotalTransactionCost()` → `BacktestSummaryDto` |
| AC-06.1〜06.3 | `ValueAt` 未改動 |
| AC-06.4 / 06.5 | `TotalTransactionCost` 加未平倉那一注的 `EntryCost()`；交易明細未改動 |
| AC-07.1〜07.5 | 助手參數多四格，之後與其他兩個入口共用同一條路 |

---

## 7. Extensibility & Handoff Notes

- **每筆最低手續費**（台股 20 元）若日後要做：`BacktestTransactionCostsDomain`
  多兩個下限欄位、兩個 `CostFor` 各多一個 `Max`。**不要**改成可組合的成本模型階層——
  這個系統的入口是表單與工具參數，多型的成本物件在 JSON 上表達不出來，
  而且 `any` 是專案明文禁用的。
- **固定金額成本 / 按風險量計價**：同上，都是這個 model 多長幾個欄位的事，
  不需要動任何呼叫端。這正是把算術關進一個 model 的報酬。
- **滑價**若要做，落點**不在這裡**——它改的是「成交在什麼價位」，
  而成本改的是「成交之後付多少」。兩者放進同一個 model 會讓兩件事共用一個零值。
- **機器人的建議部位**若日後要預留成本：`position_plan_domain.go` 那一行把零值換掉即可，
  程式碼已經為那一天留了位置。

### 看過但**刻意不做**的兩個重構

**一、把四條「這次怎麼交易的規則」打包成一個參數物件。**
`BacktestDomain → Simulation → Account` 這條鏈現在各自帶著倉位大小、交易模式、出場價位、
交易成本四樣往下傳，上一刀加出場價位、這一刀加交易成本，兩次都把三個建構子拉寬一格。
下一條規則還會再來一次。

打包起來確實能讓那三個簽章從此不再變動，但換來的是一個帶四個 getter 的袋子：
帳戶還是得一個一個把它們要回來，耦合只是換了地方，沒有被吸收掉。
要讓它真的變成深模組，得由它自己回答「這一棒要開多大的倉」而不是交出零件——
那是重新設計帳戶怎麼開倉，是另一刀的工作量與另一刀的風險，不是這一刀的收尾。

**二、把 `validatedCostPercentage` 與 `validatedDistance` 併成一個。**
兩者的形狀一樣（不得為負、不得超過 100），但**理由不一樣**：距離超過 100 會讓價格變成負數，
費率超過 100 會收得比成交金額還多。併起來就得把句子參數化，
而拿到錯句子的人會跑去看錯的地方。兩個字面上像、意義上不同的規則，分開是對的。

**三、`settleOpenPosition` 同時讀倉位的估值與已完成交易的出場成本。**
一個「收回多少現金」的答案來自兩個物件，看起來像是該併。但併進倉位就得把出場成本再乘一次
（而這個 package 對「同一個數字算兩遍」有明確立場），併進交易則要從淨額反推回毛額。
這個形狀在這一刀之前就存在，這一刀只多了一個減號，維持原樣。

---

## 8. Appendix — 驗算

以下三組數字直接對應 PRD 的 Gherkin，實作完成後應逐位相符。
統一設定：初始資金 **10100**、全押、進場與出場成本率都是 **1**、第一棒收 **100** 說買入。

**開倉**
```
可押上限 = 10100 ÷ 1.01 = 10000
押注     = 10000          進場成本 = 10000 × 1% = 100
可用資金 = 10100 − 10000 − 100 = 0
單位數   = 10000 ÷ 100 = 100
那一棒的資金曲線點 = 0 + ValueAt(100) = 0 + 10000 = 10000   （初始 10100，立刻掉 100）
```

**做多、第三棒收 110 說賣出**
```
成交金額 = 100 × 110 = 11000      出場成本 = 110
ValueAt(110) = 10000 + 100×(110−100) = 11000
可用資金 = 0 + 11000 − 110 = 10890
這一筆損益 = 1000 − 100 − 110 = 790          （淨額，算贏）
累計成本 = 100 + 110 = 210
```

**做空、第一棒收 100 說賣出、第三棒收 90 說買入**
```
放空 100 單位，進場成本 100
成交金額 = 100 × 90 = 9000        出場成本 = 90     ← 若誤用 ValueAt 會算成 110
ValueAt(90) = 10000 + 100×(100−90) = 11000
可用資金 = 0 + 11000 − 90 = 10910
這一筆損益 = 1000 − 100 − 90 = 810
```

**原地打平（第三棒收 100 說賣出）**
```
成交金額 = 10000   出場成本 = 100
可用資金 = 0 + 10000 − 100 = 9900
這一筆損益 = 0 − 100 − 100 = −200            （不算贏）
最後剩 9900，初始 10100 —— 差的正好是那兩筆成本
```

**不填成本的同一段行情（迴歸網）**
```
可押上限 = 10100 ÷ 1 = 10100
押注 10100、單位數 101、第三棒收 110
ValueAt(110) = 10100 + 101×10 = 11110        （+10%，與這一刀之前一字不差）
```
