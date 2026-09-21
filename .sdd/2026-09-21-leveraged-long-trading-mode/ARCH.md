# 槓桿做多這種交易模式 — Architecture Design

**Status:** Draft
**PRD:** `.sdd/2026-09-21-leveraged-long-trading-mode/PRD.md`
**Repos:** `go-trading`（後端）、`go-trading-frontend`（前端）、`go-trading-mcp`（助手工具目錄）

---

## 1. Design Goal & Guiding Principle

交易模式一直用**一個二值選擇**回答**兩個獨立問題**（做不做得了空、借不借得到錢），
於是第三種合法組合表達不出來。設計的核心是**不把那兩個問題合併得更緊，而是把它們分開問**。

`TradingModeDomain` 其實**早就把它們分開問了**——`CanGoShort()` 與 `CanUseLeverage()`
是兩個各自獨立的述詞，而且原始碼裡已經寫下了這一刀的預言：

> *It is a second question rather than a reading of `CanGoShort`, although the two currently
> answer alike. … a venue that lent against long-only positions would make them differ
> without either of these sentences becoming false.*
> — `internal/domain/models/domains/trading_mode_domain.go`

所以這不是一次結構改造，是**讓那兩個述詞第一次給出不同的答案**。
新增一個列舉值，兩個述詞照原本的語意各自回答，其餘的一切自然分流。

**指導原則**

1. **問能力，不問模式。** 任何一處需要分支的地方，一律問 `CanGoShort()` / `CanUseLeverage()`，
   **絕不新增 `== TradingModeSpot` 這類對特定模式的比對**。前端目前有一處是這樣寫的，
   這一刀順手把它改成問能力——否則第三種模式會悄悄落進錯的那一邊。
2. **一句話只有一個家。** 「借不到錢」這句拒絕，現在只有重演說得出來。
   機器人也要說同一句，所以把它搬到 `TradingModeDomain` 上，兩邊都去那裡拿。
3. **存的時候擋，跑的時候不擋。** 既有機器人不得被回頭作廢（PRD 明訂），
   所以這道關卡放在**存**這條路上，不放在每一輪都會走的那條路上。

---

## 2. Change Scope

### 後端 `go-trading`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `internal/domain/models/vo/trading_mode_vo.go` | 多一個列舉值 |
| 修改 | `internal/domain/models/domains/trading_mode_domain.go` | 可選清單多一項；三個述詞各自認得它；**新增拒絕句的唯一出處** |
| 修改 | `internal/domain/models/domains/backtest_leverage_domain.go` | 不再自己組那句話，改向 `TradingModeDomain` 要 |
| 修改 | `internal/domain/models/domains/strategy_bot_domain.go` | 收下交易模式，加上存檔時的槓桿關卡 |
| 修改 | `internal/domain/models/dto/strategy_bot_write_dto.go` | 多一個由 application 填的欄位（與既有的擁有者同一類） |
| 修改 | `internal/domain/service/strategy_bot_service.go` | 建立與修改兩條路把交易模式傳進 domain |
| 修改 | `internal/application/strategy_bot_application.go` | 擁有權關卡**不再丟掉它查到的那份交易策略**，改為回傳它的交易模式 |
| 修改 | `internal/domain/models/domains/strategy_bot_message_domain.go` | 「要不要印交易模式那一行」的條件換一個問法 |
| 修改 | `internal/application/assistantqueries/*` | 助手看得懂的說明從兩種變三種 |

### 前端 `go-trading-frontend`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `app/domain/models/vo/trading-mode-vo.ts` | 多一個字面量與一個排序位置 |
| 修改 | `app/domain/models/domains/trading-mode-domain.ts` | 多一組名稱與說明；**新增 `canUseLeverage()`** |
| 修改 | `app/domain/models/domains/backtest-leverage-domain.ts` | `=== 'spot'` 換成問能力 |

### 助手工具目錄 `go-trading-mcp`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `cmd/server/tool_catalog_strategy.go` | 工具說明列出的模式從兩種變三種 |

### 明確**不動**的東西

- **`BacktestLeverageDomain` 的每一條算法。** 強平距離、維持保證金率的預設與上限、
  不穿倉、止損與強平的先後——一行都不改。槓桿做多只是多一種**通得過那道門**的模式。
- **`PositionPlanDomain`。** 它不收交易模式（理由見 §3 的取捨）。
- **`BacktestAccountDomain` / 倉位模擬。** 它讀的是 `TargetFor` 的答案，
  而槓桿做多的答案與現貨逐字相同，所以它不必知道有第三種模式存在。
- **持久化與 schema。** 交易模式本來就是字串欄位，多一個取值不動表結構。
- **既有的兩個列舉值、既有的預設值、既有的任何一則訊息措辭。**

---

## 3. New Classes / Modules

**這一刀不新增任何類別。** 這是設計的結論，不是省略。

三種模式是同一個概念的三個取值，不是三種各有自己資料與行為的東西；
為它開一個 `LeveragedLongTradingModeDomain` 會讓 `TradingModeDomain`
從「一個知道自己能做什麼的值」變成「一個要去問別人的殼」，
而它現在那三個述詞（`CanGoShort` / `CanUseLeverage` / `TargetFor`）
正是**淺介面的反面**：呼叫端問一句話就拿到能用的答案，不必自己組步驟。

唯一新增的是 `TradingModeDomain` 上的**一個方法**：

| 方法 | 責任 | 為什麼在這裡 |
| :--- | :--- | :--- |
| `BorrowingRefusal() error` | 這套規則借不到錢時，那一句拒絕；借得到時回 `nil` | 它同時需要 `CanUseLeverage()`（該不該拒）與 `InWords()`（拒絕句裡那個詞），**兩樣都只有 `TradingModeDomain` 有**。放在別處就得把這個模型整個傳過去，那正是 Feature Envy |

**取捨：這道關卡為什麼不放進 `PositionPlanDomain`。**

直覺上它該放那裡——`NewBacktestLeverageDomain` 就是收下交易模式、在建構子裡擋的，
比照辦理最一致。但 `PositionPlanDomain` 有**兩個**建構點：

1. `StrategyBotDomain` 存一台機器人時（`strategy_bot_domain.go:109`）
2. `StrategyBotService` **每一輪**算建議部位時（`strategy_bot_service.go:342`）

把關卡放進建構子，第 2 條路會讓**既有那台現貨 1.8 倍的機器人每一輪都失敗**，
直接違反 PRD「既有機器人照常跑」。而「跑的時候放行、存的時候擋」這種
分情況的建構子，等於一個建構子有兩種意思——那比多一個呼叫點糟。

所以關卡放在 `StrategyBotDomain`，它的型別註解本來就寫著
*"holds one strategy bot **as it is being saved** and guarantees its own invariants"*。
**這是一條關於「存一台機器人」的規則**，它待在對的地方。

---

## 4. Modified Components

### 4.1 `vo.TradingModeVo`

```go
// TradingModeLeveragedLong never goes short — a sell closes back to cash, exactly as
// spot does — but it may put on a position worth more than the money behind it.
TradingModeLeveragedLong TradingModeVo = "leveragedLong"
```

### 4.2 `TradingModeDomain`

| 成員 | 改動 |
| :--- | :--- |
| `selectableTradingModes` | 追加 `TradingModeLeveragedLong`（**接在最後**，既有兩個不移位；這份順序就是拒絕句裡列出的順序，也是前端選項的順序） |
| `CanGoShort()` | **一個字不改**——它只在 `longShort` 時為真，新模式自然落在假的那一邊 |
| `CanUseLeverage()` | `longShort` **或** `leveragedLong` 為真。**這是那兩個述詞第一次給出不同的答案** |
| `TargetFor()` | `switch` 多一個 `case`：賣出時回 `TargetPositionFlat`，與 `spot` 同一行 |
| `InWords()` | `switch` 多一個 `case`：`"槓桿做多"` |
| `BorrowingRefusal()` | **新增**（見 §3） |

`TargetFor` 與 `InWords` 的 `switch` 目前都刻意**不寫 `default` 分支**（每一個模式都被點名），
新模式必須各自補上一行——這是既有設計故意留下的提醒機制，照它走。

### 4.3 `BacktestLeverageDomain`

那句 `fmt.Errorf("%s交易模式開不了槓桿——…", tradingMode.InWords())` 收掉，
改成：

```go
if borrowingRefusal := tradingMode.BorrowingRefusal(); borrowingRefusal != nil {
    return BacktestLeverageDomain{}, borrowingRefusal
}
```

行為**逐字不變**（現貨被拒時說的還是同一句話），變的只有那句話住在哪裡。

### 4.4 `dto.StrategyBotWriteDto`

多一個欄位 `TradingMode string`。

**它與既有的 `OwnerID` 是同一類東西**：都不是請求本文裡的欄位，
而是 application 從當下的情境填進來的（`OwnerID` 來自登入者，
`TradingMode` 來自這台機器人指名的那份交易策略）。
這個先例已經存在，所以不必為它另立一個輸入型別。

### 4.5 `StrategyBotDomain`

建構子簽章不變（仍然只收 `writeDto`），交易模式從 `writeDto.TradingMode` 讀。
在**既有的 `NewPositionPlanDomain` 那一段之後**加上關卡：

```go
// 借不借得到錢，是那份規則的事，不是這台機器的事——所以問它，而不是自己判斷。
// 建議一個它所引用的規則做不到的槓桿，做出來的是一台跑得起來、卻怎麼樣都
// 重演不出來的機器人，而那正是這一刀要消滅的狀態。
if positionPlan.IsBorrowed() {
    if refusal := tradingMode.BorrowingRefusal(); refusal != nil {
        return StrategyBotDomain{}, fmt.Errorf("%w: %s", ErrStrategyBotValidation, refusal)
    }
}
```

需要 `PositionPlanDomain` 回答「這組建議有沒有借錢」。
`BacktestLeverageDomain` 已經有一個同名同義的 `IsBorrowed()`，**沿用那個名字**，
在 `PositionPlanDomain` 上補一個——兩個模型對同一個問題用同一個詞。

`tradingMode` 由 `NewTradingModeDomain(writeDto.TradingMode)` 產生；
它回的錯誤沿用既有的 `ErrStrategyBotValidation` 包法。

### 4.6 `StrategyBotApplication`

`requireOwnedTradingStrategy` 目前把查到的交易策略丟掉（`_, findError := …`）。
改成回傳它的交易模式：

```go
func (…) tradingModeOfOwnedTradingStrategy(
    executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (string, error)
```

**擁有權那條規則一個字都不改**：查不到與別人的仍然回同一句「找不到」。
唯一的變化是它不再扔掉手上已經有的東西。
`tradingStrategyID == 0` 時回空字串——那一路由 `StrategyBotDomain` 既有的
「必須指名一份交易策略」擋下，順序不變。

### 4.7 `StrategyBotMessageDomain`

只改一個條件。目前：

```go
if strategyBotMessageDomain.tradingMode.CanGoShort() {   // 印交易模式那一行
```

改為：

```go
// 這一行的存在，是為了講出動詞沒講的事。現貨的「買入」就是拿現金去換，沒有第二種
// 讀法，所以它不印。另外兩種各有一件動詞說不出口的事——一個會反手做空，一個是
// 借錢開的倉、會被強制平倉——而那兩件事都足以改變他該用什麼心情去執行。
if strategyBotMessageDomain.tradingMode.CanGoShort() ||
    strategyBotMessageDomain.tradingMode.CanUseLeverage() {
```

**動詞那一段（第 97 行附近）一個字都不改**：它問的是 `CanGoShort()`，
而槓桿做多做不了空，自動落在 `買入／出場` 那一邊——正是 PRD 要的。

### 4.8 前端

| 檔案 | 改動 |
| :--- | :--- |
| `trading-mode-vo.ts` | 字面量聯合加 `'leveragedLong'`；`TRADING_MODES` 追加在最後；`DEFAULT_TRADING_MODE` **不動** |
| `trading-mode-domain.ts` | `TRADING_MODE_DESCRIPTIONS` 多一組（label `槓桿做多`）；**新增 `canUseLeverage(): boolean`**，與後端同名同義 |
| `backtest-leverage-domain.ts` | 刪掉 `SPOT_TRADING_MODE` 常數與 `=== SPOT_TRADING_MODE` 比對，改成 `new TradingModeDomain(mode).canUseLeverage()`。**拒絕的字串維持逐字相同** |

前端的 `TradingModeDomain` 目前只負責「那一句話要有一個家」；
加上 `canUseLeverage()` 讓它同時成為「那一條規則要有一個家」，
與後端同一個模型擔同樣兩件事的形狀一致。

**機器人表單不自行複製那道關卡**：它照既有做法呈現伺服器回來的拒絕訊息。
規則只有一個權威，前端複製第二份只會多一個會漂移的地方
（回測那一份是既有決定，不在這一刀的範圍內動它）。

### 4.9 助手工具目錄

`tool_catalog_strategy.go` 裡列舉模式的兩處說明文字改為三種，
並講清楚第三種的用途（**合約帳戶只做多、要上槓桿**），
好讓助手在使用者說「我在合約只做多」時挑得到它而不是退而求其次挑現貨。

---

## 5. Component Relationships

```
存一台機器人
  Controller
    └─▶ StrategyBotApplication
          ├─▶ tradingModeOfOwnedTradingStrategy()  ← 既有的擁有權關卡，改成不丟掉查到的東西
          │     └─▶ TradingStrategyService.GetTradingStrategy()
          └─▶ StrategyBotService.Create / Update(writeDto{…, TradingMode})
                └─▶ NewStrategyBotDomain(writeDto)
                      ├─▶ NewPositionPlanDomain(writeDto.PositionPlan)   ← 不收交易模式
                      │     └─▶ IsBorrowed()                             ← 新增
                      └─▶ NewTradingModeDomain(writeDto.TradingMode)
                            └─▶ BorrowingRefusal()   ★ 那句話的唯一出處

發動重演
  BacktestDomain
    └─▶ NewBacktestLeverageDomain(multiplier, mmr, tradingMode)
          └─▶ tradingMode.BorrowingRefusal()   ★ 同一個出處，同一句話

機器人送訊息（每一輪）
  StrategyBotService
    ├─▶ NewPositionPlanDomain(round.PositionPlanSettings)   ← 不擋，既有機器人照常跑
    └─▶ StrategyBotMessageDomain
          ├─▶ CanGoShort()                      → 動詞：做多／做空 vs 買入／出場
          └─▶ CanGoShort() || CanUseLeverage()  → 要不要印交易模式那一行
```

★ 標的是這一刀唯一的新匯流點：**兩條路第一次去同一個地方拿同一句話。**

---

## 6. Extensibility & Handoff Notes

### 最可能的下一個需求，以及它會打在哪裡

**「還有第四種帳戶」不會發生**——兩個布林問題只有三種組合成立
（做得了空卻借不到錢不存在，放空本來就要先借）。
這是這個設計敢用列舉而不用策略物件的理由：**取值集合是封閉的**。

真正會來的下一個需求是**第三個地方也要問「借不借得到錢」**。
資金費率、真的去下單、風險預檢——每一個都會再問一次同一句話。
那個縫已經留好了：`TradingModeDomain.BorrowingRefusal()`。
**新的呼叫端去那裡拿，不要自己寫第二句。**

### 留給下一位的三條線

1. **永遠問能力，不要問模式。** 看到 `== TradingModeSpot` 或 `=== 'spot'` 就是紅燈——
   那種寫法在這一刀之前是對的（只有兩種模式，問哪一個等於問能力），
   從現在起它會把第三種模式分到錯的那一邊，而且**不會報錯**。
   前端那一處就是這樣被找出來的。
2. **不寫 `default` 分支。** `TargetFor` 與 `InWords` 的 `switch` 刻意點名每一個模式，
   漏掉的那一個會以自己的存放字串冒出來——讀起來很怪，但是真的。
   加上 `default` 會讓它安靜地變成別的模式。
3. **「存的時候擋、跑的時候不擋」是刻意的。** 若哪天覺得該一併擋住每一輪，
   請先確認現存資料裡沒有跑著違反的組合——現在就有兩台。

### 既有那兩台機器人的去路

它們引用現貨交易策略、建議 1.8 倍，正在跑。這一刀之後：
照常跑 → 使用者停掉它 → 改那份交易策略成槓桿做多 → 再存機器人 → 通過。
**這條路是通的**，因為「執行中的機器人改不動」與「被引用中的交易策略改不動」
這兩條既有規則，剛好把順序排成了唯一正確的那一種。

---

## 7. Traceability

| PRD User Story | 場景涵蓋 | 由誰實現 |
| :--- | :--- | :--- |
| **US-01** 挑得到這個模式 | 存得起來 ×3、沒填、認不得的值、既有不被改、改成新模式 | `TradingModeVo`、`TradingModeDomain`（`selectableTradingModes`、建構子既有拒絕句） |
| **US-02** 倉位行為與現貨相同 | 開多、忽略、平倉回現金、空手賣出、無空倉、兩模式同分 | `TradingModeDomain.TargetFor()` 多一個 `case`；`BacktestAccountDomain` **不動** |
| **US-03** 只有借得到錢的才開得了槓桿 | 新模式接受、現貨仍拒、多空反手照舊、留白、填一、0.5 | `TradingModeDomain.CanUseLeverage()` / `BorrowingRefusal()`；`BacktestLeverageDomain` |
| **US-04** 強平算法一字不差 | 強平價、收回零、止損先、筆數、無槓桿為零、永遠在下方 | `BacktestLeverageDomain` **不動**（沿用）；「永遠在下方」由 `TargetFor` 只產出多倉保證 |
| **US-05** 機器人的槓桿受限於那份規則 | 現貨+1.8 拒、新模式+1.8 過、多空反手過、留白過、填一過、0.5 拒、既有照常跑、再存才擋、改完就過 | `StrategyBotDomain`（新關卡）、`PositionPlanDomain.IsBorrowed()`、`StrategyBotApplication`、`StrategyBotWriteDto` |
| **US-06** 訊息講出借錢這件事 | 買入/出場動詞、印模式行、印保證金名目、留白不印那兩行、現貨不變、多空反手不變、來源段不變 | `StrategyBotMessageDomain`（唯一改的是那個條件） |
| **US-07** 三條路都挑得到 | 畫面三選項、助手挑對、腳本重演指定得了、拒絕列出三種 | 前端 `trading-mode-vo.ts` / `trading-mode-domain.ts`；MCP `tool_catalog_strategy.go`；後端 `selectableTradingModes`（腳本重演那條路本來就讀它） |

---

## 8. Risks & Open Decisions

### 風險

| 風險 | 緩解 |
| :--- | :--- |
| **漏掉某一處「問模式而不是問能力」的寫法。** 症狀是新模式被當成現貨或多空反手處理，**不報錯** | 已知的一處（前端 `backtest-leverage-domain.ts`）列進範圍。實作時以 `TradingModeSpot` / `'spot'` 全文檢索兩個 repo，逐一確認每一處問的是能力 |
| **`switch` 漏補一個 `case`。** `TargetFor` 漏掉會讓新模式**完全不交易**（回 `Unchanged`） | US-02 的驗收條件會直接抓到；且既有設計刻意不寫 `default`，讓漏掉成為看得見的行為而非安靜的誤判 |
| **`StrategyBotWriteDto` 多一個 application 才填得出的欄位**，日後有人從 controller 直接填它 | 該欄位的註解比照既有的 `OwnerID` 寫清楚來源；`OwnerID` 的先例已經在那裡擋了一段時間 |
| **前端與後端的模式清單漂移。** 後端加了、前端沒加，使用者就挑不到 | US-07 把三條路各寫一條驗收條件 |

### Open Decisions（留給實作階段）

- **列舉的存放字串。** 本設計採 `"leveragedLong"`，與既有的 `"longShort"` 同一種拼法風格。
  一旦寫進資料就改不動了（既有資料沒有這個值，所以現在是唯一可以決定的時機）。
- **前端那三個選項的說明文字。** 槓桿做多與現貨在說明上只差「借不借得到錢」，
  兩句話要並排讀得出差別——這是文案，不是結構，實作時定。
- **助手說明裡要不要點名「永續合約」。** 詞彙上這個模式**不以場所命名**，
  但助手要在使用者說「我在幣安開合約」時對得上，說明文字裡把它當**例子**
  提一句是合理的（詞彙是詞彙，例子是例子）。
