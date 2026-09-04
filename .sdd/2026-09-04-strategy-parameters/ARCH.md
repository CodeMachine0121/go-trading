# 策略自己的旋鈕 — Architecture Design

**Feature:** 策略自己的旋鈕
**Status:** Finalized
**PRD:** `PRD.md`（同一資料夾）
**Owner:** James Hsueh

---

## 1. Design Goal & Guiding Principle

三個問題必須正面回答，其餘都是它們的後果：

1. **兩種參數怎麼放進同一份清單，而不用空介面。**
2. **算式怎麼取用參數，而取用一個沒宣告的名字時失敗要怪對人。**
3. **「要拿幾根」的推導屬於誰。**

指導原則：**參數不是兩種型別，是一個數字加上一種讀法。**
這句話決定了下面每一個設計選擇。

---

## 2. 核心型別設計：一個數字，一種讀法

直覺的做法是讓值是「整數或浮點數」，於是需要一個能裝兩者的位置——那就是空介面，規範明文禁止，
而且它會把「這是哪一種」這件事從宣告推遲到讀取的那一刻。

**但這裡根本沒有兩種型別。** 使用者填的是**一個數字**；
不同的是**系統怎麼讀它**：回看根數要當成整數拿去數根數，數值就是個數字。

```go
type StrategyParameter struct {
    ID           uint
    StrategyID   uint    `gorm:"not null;index"`
    Name         string  `gorm:"size:64;not null"`
    Kind         string  `gorm:"size:32;not null"`  // lookbackCount ／ number
    DefaultValue float64 `gorm:"not null"`
}
```

- **值只有一個欄位，型別具體**：`float64`。沒有空介面，也沒有「兩個欄位其中一個永遠沒意義」。
- **回看根數存在 `float64` 裡不會失真**：整數在 `float64` 中精確到 2^53，
  而回看根數的上限是單次查詢上限（一千）——差了十兆倍。
- **`Kind` 是宣告的一部分**，不是讀取時才猜的。`StrategyParameterKindDomain` 負責正規化與拒絕，
  形狀比照既有的 `IndicatorResultTypeDomain`（同樣是「宣告的種類，非法即拒絕」）。

**金額仍然用精確小數，這裡不是金額。** 期數與倍數是純數值，
既有規範對 `float64` 的限制是針對價格與金額，本切片沒有觸及它們。

---

## 3. Change Scope

### 新增

| 層 | 檔案 | 為什麼存在 |
| :--- | :--- | :--- |
| entities | `strategy_parameter.go` | 一個參數的本體形狀（見上） |
| domains | `strategy_parameter_kind_domain.go` | 「這是哪一種」的正規化與拒絕 |
| domains | `strategy_parameters_domain.go` | **本切片的核心**：一整份參數的所有規則 |
| dto | `strategy_parameter_dto.go` | 參數交給 application 的形狀 |
| vo | `strategy_parameter_value_vo.go` | 這一次執行給的一個值（名稱＋數字） |
| domains | `indicator_parameter_errors.go` | 兩個哨兵錯誤（見 §5） |

### 修改

| 檔案 | 改什麼 |
| :--- | :--- |
| `entities/strategy.go` | 多一個 `Parameters []StrategyParameter`（GORM 一對多） |
| `persistence/strategy_repository.go` | 讀時 `Preload`、寫時整份取代 |
| `dto/strategy_write_dto.go`／`strategy_dto.go` | 帶上參數 |
| `domains/strategy_domain.go` | 建構時驗證整份參數 |
| `dto/indicator_calculation_request_dto.go` | 多帶參數宣告與這一次的值 |
| `domains/indicator_calculation_domain.go` | 「要拿幾根」的推導（見 §6） |
| `interface/i_indicator_script_proxy.go`＋`script/yaegi_...` | 把參數送進算式（見 §5） |
| `controller/strategy_controller.go`／`indicator_calculation_controller.go` | 請求形狀 |
| `persistence/schema_migrator.go` | 新表 |

### 刻意不動

- **進入點的形狀 `func Calculate(data []indicator.KCandle) map[string]<值形狀>` 一個字都不改。**
  使用者已同意既有策略可以壞掉，但**沒有必要壞**——參數走符號注入這條路
  （`indicator.Data` 已經是這樣進去的），因此**既有策略全部繼續可用**。
  被允許破壞，不等於應該破壞。
- **「湊不滿就整次拒絕」與單次查詢上限**：沿用。
- **彙總刻度、算到哪一刻**：沿用，它們仍屬於這一次執行。

---

## 4. `StrategyParametersDomain` — 一整份參數的所有規則

**它是這個切片唯一需要被讀懂的物件。** 對外只回答問題，不外洩那份清單：

```go
NewStrategyParametersDomain(parameters []entities.StrategyParameter) (StrategyParametersDomain, error)

// 這一次執行：把給的值套上去，回傳一份「已經定案」的參數
Applying(values []vo.StrategyParameterValueVo) (StrategyParametersDomain, error)

MaximumLookbackCount() int          // 沒有任何回看根數時為 0
LookbackCountOf(name string) (int, bool)
NumberOf(name string) (float64, bool)
ToDtos() []dto.StrategyParameterDto
```

**建構子就把整份驗證完**：名稱不得為空白、去除前後空白、同一份內不得重複、
種類二選一、回看根數必須大於零。**實例存在就代表這一份是可用的**，
與專案既有的建構子正規化慣例一致。

`Applying` 是**唯一**的套值入口，它一次做完四件事——每個名字都有被宣告嗎、
沒給的補上預設值、給的值在它那一種的範圍內嗎、產生一份新的定案參數。
呼叫端因此不必自己排這四步，也不可能漏掉其中一步。

**為什麼是一份而不是一個個。** 「不得重複」「最大的回看根數」都是**整份**的性質，
單獨一個參數答不出來。把它們放在單一參數上，呼叫端就得自己蒐集再比對——那正是要避免的。

---

## 5. 算式怎麼取用參數，失敗怎麼怪對人

### 注入兩個具名函式

`Data` 現在是以符號注入進 yaegi 的，參數走同一條路：

```go
scriptSymbols[scriptDataPackage] = {
    "KCandle":       ...,
    "Data":          ...,
    "LookbackCount": reflect.ValueOf(func(name string) int { ... }),
    "Number":        reflect.ValueOf(func(name string) float64 { ... }),
}
```

算式因此這樣寫，**沒有型別斷言、沒有空介面**：

```go
period := indicator.LookbackCount("期數")   // int，可以直接拿去切片
factor := indicator.Number("倍數")          // float64
```

### 名字對不上時，怎麼一路正確地報出來

這是本切片最容易做錯的地方：**yaegi 把算式裡的任何失敗都變成「算式執行失敗」**，
於是「你把參數改了名」會被說成「你的算式壞了」——**怪錯人**，而 PRD 明文要求不能這樣。

做法：注入的那兩個函式**閉包捕獲一個記錄器**。取用一個沒宣告的名字時，
它**記下那個名字並 panic**：

- **panic 而不是回傳零**：回傳零會讓迴圈變成「看過去零根」，
  算出來仍是一串看起來正常的數字，而使用者會拿它去做決定。
  更糟的是 `data[i-0+1:]` 這類寫法會讓它跑滿整個執行允許時間才失敗。
- **panic 由 yaegi 收成一個錯誤**，proxy 因此不會整個倒掉。

proxy 拿到錯誤之後，**先問記錄器**：

```
有記到名字 → ErrIndicatorParameterNotDeclared，訊息指名是哪一個
沒記到     → 照舊 ErrIndicatorScriptFailed
```

**記錄器優先於錯誤內容本身**，所以判斷不依賴 yaegi 的錯誤訊息長什麼樣子——
那是別人的實作細節，隨版本會變。

---

## 6. 「要拿幾根」的推導屬於誰

屬於 **`IndicatorCalculationDomain`**：它已經是「要幾根、算到哪一刻、讀到哪裡為止」
這幾件事的唯一歸屬地（`SourceCandleLimit`、`ReadCutoff` 都在它身上）。

```
InputCandleCount() = 呼叫端要看的根數 + MaximumLookbackCount() − 1
```

沒有任何回看根數時 `MaximumLookbackCount()` 是 0，於是它就等於呼叫端要看的根數——
**同一條式子涵蓋兩種情形，不需要分支**。

`CandleCount` 這個既有欄位的意思因此變了：從「餵給算式幾根」變成
「**我要幾格有值**」。這是刻意的——它現在對應的是使用者說得出口的那件事。
超過單次查詢上限的檢查對象是推導後的結果，沿用既有規則。

---

## 7. 存放方式：獨立一張表，但沒有自己的 repository

**選了關聯表，不是把整份參數塞成一欄。**

塞成一欄（例如一段序列化文字）看似省事，代價是**每一條規則都從資料裡消失**：
「回看根數必須大於零」只剩程式碼知道，任何人打開資料庫看到的是一團字。
關聯表讓每個欄位都有型別、有名字、看得懂。

**但它沒有自己的 repository**，`StrategyRepository` 讀時 `Preload`、寫時整份取代。

### 刻意偏離既有規範

規範寫著「一個 entity 對應一個 repository」。這裡不遵守，理由是：
**參數沒有自己的生命**——它不會被單獨查詢、單獨建立、單獨刪除，
它整份屬於一支策略，跟著策略生、跟著策略死。給它一個 repository，
等於允許有人在策略不知情的情況下改動它的一部分，
而「同一份內名稱不得重複」這條規則屆時沒有任何地方守得住。

規範那句話的用意是「一個聚合的讀寫只有一個入口」，本設計正是那個用意：
**策略就是那個入口。**

---

## 8. Extensibility & Handoff Notes

### 最可能的下一個需求

**「我想要一個是非的旋鈕」**（例如「要不要把成交量算進去」）。使用者已說先不做。

**誠實地說：這個接縫不是免費的。**

- 再多一種**數字**的種類（例如「百分比」）——**只要多一個常數與一個讀法**，
  值仍然是那個 `float64`，一行資料表都不用改。
- 但**是非不是數字**。它要嘛多一個欄位，要嘛把值改成文字再各自解讀——
  後者等於把型別推遲到讀取時，正是本設計拒絕的那條路。
  **屆時該做的是多一個欄位**，而不是把 `DefaultValue` 撐大成什麼都能裝。

把這件事寫下來，是為了讓下一個人不必重新推導一次，也不會因為
「反正 `Kind` 已經可以擴充」就把是非硬塞進一個數字欄位裡。

### 給下一個接手的人

- **不要把值改成一個什麼都能裝的型別。** 兩種讀法是宣告的一部分，
  不是讀取時才決定的事；改掉它，畫面就長不出正確的輸入框、驗證也沒地方成立。
- **不要把「名字對不上」併回算式執行失敗。** 那是使用者最容易犯、
  也最看不出來的錯，而它值得一個自己的說法。
- **不要給參數一個 repository。** 見 §7。

---

## 9. Traceability

| PRD 情境 | 由誰滿足 |
| :--- | :--- |
| US-01.1 帶著參數的策略被完整記住 | `Strategy.Parameters` 關聯 ＋ `StrategyRepository` 整份寫入 |
| US-01.2 讀回來時參數原樣還在 | `StrategyRepository` 讀時 `Preload` |
| US-01.3 一個參數都不宣告照常運作 | `StrategyParametersDomain` 允許空的一份；`MaximumLookbackCount()` 回 0 |
| US-01.4 名稱不得重複 | `NewStrategyParametersDomain` 的驗證 |
| US-01.5 名稱不得為空白 | 同上 |
| US-01.6 前後空白不予保留 | 同上（建構時正規化） |
| US-01.7 回看根數必須大於零 | 同上（依 `Kind` 分流的範圍檢查） |
| US-01.8 種類只有兩種 | `StrategyParameterKindDomain` |
| US-01.9 數值不限正負與小數 | 同上（數值那一種不設範圍） |
| US-02.1 要看的每一格都有值 | `IndicatorCalculationDomain.InputCandleCount()` |
| US-02.2 沒有回看根數就是那一段的根數 | 同上（最大值為 0） |
| US-02.3 好幾個回看根數只看最大的 | `StrategyParametersDomain.MaximumLookbackCount()` |
| US-02.4 只看一格也拿滿回看所需 | `InputCandleCount()` |
| US-02.5 超過上限整次拒絕 | 既有的上限檢查，對象改為推導後的結果 |
| US-03.1 回看根數取出來是整數 | 注入的 `LookbackCount` 回 `int` |
| US-03.2 這一次給的值蓋過預設值 | `StrategyParametersDomain.Applying` |
| US-03.3 取用沒宣告的名字就是這次失敗 | 記錄器 ＋ `ErrIndicatorParameterNotDeclared` |
| US-03.4 失敗說的是名字對不上 | 同上（記錄器優先於 yaegi 的錯誤內容） |
| US-04.1 給了值就用給的 | `Applying` |
| US-04.2 沒給值就用預設值 | 同上 |
| US-04.3 給了沒宣告的名稱整次拒絕 | 同上 |
| US-04.4 給的回看根數不合法整次拒絕 | 同上（依 `Kind` 分流的範圍檢查） |
| US-04.5 給了沒人取用的值不是錯誤 | 沒有任何地方去比對「算式用了哪些」——**靠不做那件事達成** |

---

## 10. Risks & Open Decisions

### 對既有測試與既有資料的影響

- **既有策略不會壞。** 進入點形狀未改，因此每一支既有算式都繼續可執行。
  被允許破壞而選擇不破壞，是因為符號注入這條路本來就更好：
  沒有參數的算式不必為此多帶一個永遠用不到的引數。
- **既有資料需要一張新表**，既有的策略列一個字都不用改（沒有參數就是沒有關聯列）。
- **既有測試**：策略的建立／修改／讀取／列出測試會因為形狀多一份參數而需要調整，
  但它們斷言的行為沒有一條改變。指標計算的測試會因為 `CandleCount` 的意思改變
  而需要重新表態——**那是刻意的行為變更**，見 §6。

### Risks / trade-offs

- **`float64` 裝整數。** 精確到 2^53，而上限是一千，不存在精度問題。
  但下一個人看到 `float64` 可能會擔心，所以理由必須寫在型別旁邊，不只寫在這裡。
- **panic 當作控制流。** 只用在「名字對不上」這一個情況，且被 yaegi 收成錯誤、
  由記錄器決定怎麼回報。它換來的是不必等滿執行允許時間才失敗。
- **「給了沒人取用的參數值不是錯誤」靠的是不做檢查。** 若日後有人加上
  「檢查算式用了哪些名字」的功能，這條會跟著改變，屆時要一併重新決定。

### Open decisions

- 無。BRIEF 的每一項都已在 PRD 定案。
