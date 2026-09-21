# 只做空這種交易模式 — Architecture Design

**Status:** Draft
**PRD:** `.sdd/2026-09-21-short-only-trading-mode/PRD.md`
**Repos:** `go-trading`（後端）、`go-trading-frontend`（前端）、`go-trading-mcp`（助手工具目錄）

---

## 1. Design Goal & Guiding Principle

前一刀（槓桿做多）把交易模式拆成**兩個獨立的述詞**：
`CanGoShort()` 與 `CanUseLeverage()`，並立下原則「**問能力，不問模式**」。

**只做空把第一個述詞問壞了。**

`CanGoShort()` 這個名字暗示著一個二分：做得了空的是一種、做不了空的是另一種，
而「做不了空」就等於「只做多」。四種模式擺在一起，那個暗示就破了：

| 交易模式 | 做得了多嗎 | 做得了空嗎 | 借得到錢嗎 |
| :--- | :---: | :---: | :---: |
| 多空反手 | ✅ | ✅ | ✅ |
| 現貨 | ✅ | ❌ | ❌ |
| 槓桿做多 | ✅ | ❌ | ✅ |
| **只做空** | **❌** | **✅** | ✅ |

**「做得了多嗎」一直都是一個獨立的問題，只是在此之前它的答案永遠是「是」，
所以沒有人需要問出口。** 只做空是第一個答「否」的。

於是這一刀的核心是**補上那個從來沒被問出口的問題**：新增述詞 `CanGoLong()`。

補上之後，兩個既有的分支點都從「查表」變成「求值」：

```
TargetFor(買入)  = CanGoLong()  ? 持多 : 空手
TargetFor(賣出)  = CanGoShort() ? 持空 : 空手
結論動詞(買入)   = CanGoLong()  ? (CanGoShort() ? "做多" : "買入") : "出場"
結論動詞(賣出)   = CanGoShort() ? "做空" : "出場"
```

四種模式代進去，**每一格都落在 PRD 要求的答案上**，而且 `TargetFor` 裡那個
逐一列舉模式的 `switch` 整個消失——它本來就是在手寫這兩個述詞的真值表。

### 指導原則

1. **問能力，不問模式**（沿用前一刀）。這一刀把它推到底：
   `TargetFor` 與結論動詞都不再提及任何一個具體模式的名字。
2. **一句話只有一個家**（沿用前一刀）。結論動詞這條規則現在有**四種模式 × 三種信號**
   十二格，散在呼叫端就會有十二個機會寫錯。它搬進 `TradingModeDomain`。
3. **不認得的模式一律什麼都不做。** 既有原始碼刻意讓未知模式回「不變」而不是
   落進某一邊。補述詞之後這個性質**必須留著**——`CanGoLong()` 與 `CanGoShort()`
   同時為假就是那個情況，而它的答案仍然是「不變」，不是「兩邊都平掉」。

---

## 2. Change Scope

### 後端 `go-trading`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `internal/domain/models/vo/trading_mode_vo.go` | 多一個列舉值 `TradingModeShortOnly` |
| 修改 | `internal/domain/models/domains/trading_mode_domain.go` | 可選清單多一項；**新增 `CanGoLong()`**；`CanGoShort()` 與 `CanUseLeverage()` 認得新模式；`TargetFor` 改由兩個述詞求值；**新增 `HeadlineVerbFor()`** |
| 修改 | `internal/domain/models/domains/signal_domain.go` | **新增 `InWords()`**——信號自己的三個字，供結論動詞與各來源那幾行共用 |
| 修改 | `internal/domain/models/domains/strategy_bot_message_domain.go` | 結論動詞那一段改為向交易模式要一句話；`signalInWords` 改為向信號要 |
| 修改 | `internal/application/assistantqueries/*` | 助手看得懂的說明從三種變四種 |

### 前端 `go-trading-frontend`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `app/domain/models/vo/trading-mode-vo.ts` | 多一個字面量，接在清單最後 |
| 修改 | `app/domain/models/domains/trading-mode-domain.ts` | 多一組名稱與說明；借得到錢的名單多一項 |

### 助手工具目錄 `go-trading-mcp`

| 動作 | 元件 | 為什麼 |
| :--- | :--- | :--- |
| 修改 | `cmd/server/tool_catalog_strategy.go` | 建立與修改交易策略的說明，模式從三種變四種 |
| 修改 | `cmd/server/tool_catalog_automation.go` | 借得到錢的名單多一項 |
| 修改 | `cmd/server/tool_catalog.go` | 同上 |

### 明確**不動**的東西

- **`BacktestAccountDomain` 與整個倉位模擬。** 它讀的是 `TargetFor` 的答案，
  而「目標持空」「目標空手」這兩個答案它早就會處理——多空反手與現貨各自
  已經在用了。**它不必知道有第四種模式存在。**
- **`BacktestLeverageDomain` 的每一條算法。** 強平距離、維持保證金率、不穿倉、
  止損與強平的先後，一行都不改。只做空只是多一種**通得過那道門**的模式。
- **強平在進場價的哪一邊。** 既有邏輯照**倉位方向**決定，不照交易模式決定；
  只做空只有空倉，所以自動落在上方。**這是零改動就正確的一格。**
- **`PositionPlanDomain` 的止損止盈方向。** `suggestsShort` 讀的是
  `target == TargetPositionShort`，`TargetFor` 一改對，這裡自動跟著對。
- **`StrategyBotDomain` 存檔時的槓桿關卡。** 它問的是 `BorrowingRefusal()`，
  而只做空借得到錢，所以自動放行。**一行都不改。**
- **持久化與 schema。** 交易模式是字串欄位，多一個取值不動表結構。
- **既有三個列舉值、既有的預設值、既有的任何一則訊息措辭。**

---

## 3. New Classes / Modules

**這一刀不新增任何類別。** 與前一刀同一個結論，同一個理由：
四種模式是同一個概念的四個取值，不是四種各有自己資料與行為的東西。

新增的是**三個方法**，全部落在已經擁有那份資料的模型上：

| 方法 | 所屬 | 責任 | 為什麼在這裡 |
| :--- | :--- | :--- | :--- |
| `CanGoLong() bool` | `TradingModeDomain` | 這套規則做不做得了多 | 與 `CanGoShort()` 是同一個問題的另一半。它一直存在，只是答案一直是「是」 |
| `HeadlineVerbFor(SignalDomain) string` | `TradingModeDomain` | 這一輪的結論要人去做的那個動作 | 它需要 `CanGoLong()`、`CanGoShort()` 與信號本身——**前兩樣只有交易模式有** |
| `InWords() string` | `SignalDomain` | 信號自己的三個字：買入／賣出／持有 | 那是信號的詞彙，不是訊息的詞彙。訊息與交易模式都要用它，而**行為要住在它描述的資料旁邊** |

### 為什麼 `HeadlineVerbFor` 值得是一個方法

現在 `StrategyBotMessageDomain` 是這樣求出動詞的：

```go
headlineVerb := signalInWords(verdict)           // 1. 先抄信號的字
if tradingMode.CanGoShort() { ...改寫成做多／做空 } // 2. 再看能不能做空
else if tradingMode.TargetFor(...) == Flat { ...改寫成出場 } // 3. 再問一次目標
```

**呼叫端自己在排步驟，而且問了交易模式三次。** 那是淺介面的典型徵狀，
而這一刀正好會把它撐破：只做空必須在第 2 步就分岔成兩種答案。

搬進 `TradingModeDomain` 之後，呼叫端只剩一句：

```go
headlineVerb := tradingMode.HeadlineVerbFor(signal)
```

十二格真值表關在模型裡，呼叫端問一句話就拿到能用的答案。

### 取捨：為什麼不為「方向能力」開一個值物件

想過用一個 `TradeableDirectionsVo{Long, Short bool}` 把兩個述詞包起來。
**沒有做**，理由是它會讓呼叫端多一跳（`mode.Directions().CanGoLong()`）
卻換不到任何東西：那兩個布林沒有自己的行為，唯一讀它們的地方就是
`TargetFor` 與 `HeadlineVerbFor`，而那兩個都已經在 `TradingModeDomain` 身上。
**包起來只是把一個扁的東西變成兩層的扁東西。**

---

## 4. Modified Components

### 4.1 `vo.TradingModeVo`

```go
// TradingModeShortOnly is the mirror of leveragedLong: it only ever goes short.
// A sell opens a short, a buy closes back to cash, and a buy while flat does
// nothing at all. It may borrow — not as a convenience but because selling what
// you do not have means borrowing it first, so there is no version of this mode
// that does not.
TradingModeShortOnly TradingModeVo = "shortOnly"
```

加進 `selectableTradingModes` 的**最後一位**——拒絕句列出的順序就是這個順序，
既有三種在那句話裡一個字都不移位。

### 4.2 `TradingModeDomain`

```go
// CanGoLong is whether these rules may hold a position that gains when the price
// rises.
//
// It reads as though it could never be false, and until short-only it never was —
// which is exactly why it was not written down. Asking it out loud is what stops
// a mode that cannot go long from being told to "做多".
func (d TradingModeDomain) CanGoLong() bool {
    return d.value == vo.TradingModeLongShort ||
        d.value == vo.TradingModeSpot ||
        d.value == vo.TradingModeLeveragedLong
}

func (d TradingModeDomain) CanGoShort() bool {
    return d.value == vo.TradingModeLongShort || d.value == vo.TradingModeShortOnly
}

func (d TradingModeDomain) CanUseLeverage() bool {
    return d.value == vo.TradingModeLongShort ||
        d.value == vo.TradingModeLeveragedLong ||
        d.value == vo.TradingModeShortOnly
}
```

`TargetFor` 改寫為：

```go
func (d TradingModeDomain) TargetFor(signal SignalDomain) vo.TargetPositionVo {
    // Neither direction is a mode this does not recognise — a zero value, or a new
    // one added to the selectable set and forgotten in the predicates above. It asks
    // for nothing at all, so the replay makes no trades. That is the loud failure:
    // falling through to flat would quietly close positions a mode nobody meant to
    // replay, and the report card would look entirely plausible.
    if !d.CanGoLong() && !d.CanGoShort() {
        return vo.TargetPositionUnchanged
    }

    switch signal.Value() {
    case vo.SignalBuy:
        if d.CanGoLong() {
            return vo.TargetPositionLong
        }
        return vo.TargetPositionFlat
    case vo.SignalSell:
        if d.CanGoShort() {
            return vo.TargetPositionShort
        }
        return vo.TargetPositionFlat
    }

    return vo.TargetPositionUnchanged
}
```

**既有三種模式的答案逐格不變**（PRD 要求），新模式自然落位。

`HeadlineVerbFor` 新增：

```go
// HeadlineVerbFor is what this round asks its reader to go and do.
//
// A conclusion is read as an instruction, and the signal's own word is not always
// one the reader can carry out: told 賣出 by rules that cannot short, they ask what
// they are meant to be selling; told 買入 by rules that cannot go long, they ask the
// same question. 出場 is the one wording every reader can act on — there is a
// position, or there is not.
//
// It is one method rather than a branch at the call site because the table is four
// modes by three signals, and twelve cells written out where they are read is twelve
// chances to write one of them backwards — a mistake nothing would report, because
// 做多 is a perfectly ordinary word to find in a message.
func (d TradingModeDomain) HeadlineVerbFor(signal SignalDomain) string {
    // A mode this does not recognise keeps the signal's own vocabulary. A message is
    // the last place to invent a direction.
    if !d.CanGoLong() && !d.CanGoShort() {
        return signal.InWords()
    }

    switch signal.Value() {
    case vo.SignalBuy:
        if !d.CanGoLong() {
            return "出場"
        }
        if d.CanGoShort() {
            return "做多"
        }
        return "買入"
    case vo.SignalSell:
        if d.CanGoShort() {
            return "做空"
        }
        return "出場"
    }

    return signal.InWords()
}
```

**四種模式代進去：**

| 交易模式 | 買入 | 賣出 |
| :--- | :--- | :--- |
| 多空反手 | 做多 | 做空 |
| 現貨 | 買入 | 出場 |
| 槓桿做多 | 買入 | 出場 |
| **只做空** | **出場** | **做空** |

前三列與這一刀之前**逐字相同**。

### 4.3 `SignalDomain.InWords()`

把既有 `StrategyBotMessageDomain.signalInWords` 整個搬過來，
一字不改（含「認不得的值原樣寫出」那條）。原處改為呼叫它。

搬家的理由是 Feature Envy：那個方法的參數全部來自信號，
而它現在有了第二個呼叫端（`HeadlineVerbFor`）。

### 4.4 `StrategyBotMessageDomain`

結論動詞那三步換成一句 `headlineVerb := d.tradingMode.HeadlineVerbFor(signal)`。

**「要不要印交易模式那一行」的條件不動**：`CanGoShort() || CanUseLeverage()`。
只做空兩者皆真 → 印（PRD 要求）；既有三種的答案逐格不變。

### 4.5 前端

```ts
export type TradingMode = 'longShort' | 'spot' | 'leveragedLong' | 'shortOnly'
export const TRADING_MODES = ['longShort', 'spot', 'leveragedLong', 'shortOnly'] as const
const BORROWING_TRADING_MODES = ['longShort', 'leveragedLong', 'shortOnly'] as const
```

說明那一句照既有規矩寫，說出它拿買入與賣出各做什麼：

> **只做空** — 只做空：賣出就開空倉，買入是平倉把錢收回來，之後空手，不做多。它借得到錢。

**前端不複製那張動詞真值表。** 畫面上不顯示結論動詞，那是訊息的事。

### 4.6 助手工具目錄

三個檔案裡列出模式的地方各加一句。措辭要點：
**使用者說「我只想做空」「這段時間看空」時給 `shortOnly`**，
而**不要**教助手用「`longShort` 加一個永遠不成立的買入條件」——
那正是這一刀要消滅的變通辦法。

---

## 5. The Axis of Change

**下一個需求最可能是第五種模式**（例如「兩邊都做、但不反手」：
做多平倉回現金、做空也平倉回現金）。

這一刀之後，那個需求的成本是：

1. 多一個列舉值；
2. 在三個述詞裡各填一格；
3. 前端多一個字面量與一句說明；助手目錄多一句。

**`TargetFor` 與 `HeadlineVerbFor` 都不用改**——它們已經是那三個述詞的函數，
新模式只是真值表上多一列，而那一列由述詞自己回答。

這正是把 `switch` 換成述詞求值換到的東西：**新增模式從「改邏輯」變成「填答案」。**

**縫在哪裡：** `CanGoLong()` / `CanGoShort()` / `CanUseLeverage()` 這三個述詞。
任何一處需要按模式分岔的新程式碼，一律問它們，**絕不比對列舉值**——
比對列舉值的地方，就是第五種模式會悄悄落錯邊的地方。

---

## 6. Traceability

| PRD Scenario | 由誰滿足 |
| :--- | :--- |
| US-01 全部（四種取值、預設、拒絕列出四種、既有不變） | `TradingModeVo`、`TradingModeDomain.NewTradingModeDomain` + `selectableTradingModes` |
| US-02 開空／忽略同向／買入平倉／空手買入無事／持有不動／明細全是做空 | `TradingModeDomain.TargetFor` → `BacktestAccountDomain.Apply`（後者不改） |
| US-02 與變通辦法跑出同一張成績單 | 同上——兩者產生同一串目標倉位 |
| US-03 借得到錢、槓桿留白／一／小於一 | `TradingModeDomain.CanUseLeverage` → `BacktestLeverageDomain`（後者不改） |
| US-03 強平在上方、止損先到 | 既有 `BacktestPositionDomain`（不改）——強平邊由倉位方向決定 |
| US-04 機器人存得起來／拒絕 | 既有 `StrategyBotDomain` + `BorrowingRefusal()`（不改） |
| US-05 做空／出場兩個動詞 | `TradingModeDomain.HeadlineVerbFor` |
| US-05 印交易模式那一行、保證金與名目、止損往上 | 既有 `StrategyBotMessageDomain` + `PositionPlanDomain`（皆不改） |
| US-05 各來源怎麼說照舊 | `SignalDomain.InWords()`（搬家，行為一字不差） |
| US-06 畫面四個選項、順序不動 | 前端 `trading-mode-vo.ts` + `trading-mode-domain.ts` |
| US-06 助手挑得到、不用變通辦法 | `go-trading-mcp` 三個工具目錄檔案 |
| US-06 重演策略腳本認得四種 | 與 US-01 同一個建構子——兩條路本來就共用它 |

---

## 7. Risks & Mitigations

| 風險 | 緩解 |
| :--- | :--- |
| **動詞寫反**（只做空的買入被寫成「做多」）。PRD 點名這是最大的風險，而且訊息看起來完全正常、不會報錯 | 十二格真值表收進 `HeadlineVerbFor` 一個方法，並為**四種模式 × 三種信號全部十二格**寫測試——不只測新模式那三格 |
| **漏改某一處的選項清單**（拒絕句、畫面、助手目錄） | Traceability 表逐條對照；`/contract` 階段再走一次 |
| **`TargetFor` 重寫時改壞既有三種** | 既有三種的九格全部有測試；重寫後這九格必須逐格不變 |
| **未知模式的「什麼都不做」被改成「平倉」** | 兩個述詞同時為假時明確回「不變」，並為零值寫測試 |
