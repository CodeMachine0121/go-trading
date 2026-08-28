# Code Style（語言無關原則）

大小寫、格式、lint 設定 follow 各語言自身的官方慣例與工具。本檔只規定跨語言都成立的原則。

## 型別要明確

- **禁止用「任意型別」當資料多型手段**（`any` / `object` / `interface{}` / `dynamic` / `Map<String, Object>`）。
- 需要多型時用**具名介面或明確的聯合型別**。
- 唯一可接受的情境：泛型型別參數，以及對接第三方 SDK 時它自己要求的鬆散結構（如 JSON schema 定義），且需侷限在 infrastructure 層內。

## 變數宣告當下即賦值

禁止「先宣告、後賦值」。

```
// ❌
var total
total = calculate()

// ✅
total = calculate()
```

例外：序列化 / binding 的 decode 目標（需要先有一個空殼給框架填）。

## 行為掛在物件上

- 業務計算一律是 **Domain Model 的 method**，不是散落的靜態工具函式。
- **model 內禁止任何 `static` method**（entity / domain model / DTO / VO 皆然），含看似無害的工廠與轉換。建構用建構子，轉換用來源身上的 `toXxx()`——詳見 [architecture.md](architecture.md)。
- **禁止 `private static` method**：它的參數屬於誰，就把 method 搬進誰裡面，改用自身 property 運算。
- **`private` method 只有被 2 個以上 public method 共用時才留**，否則直接 inline。
- 詳細做法與範例見 [architecture.md](architecture.md) 的「禁止 private / private static method」。
- 例外：真正無狀態的轉換模組（parser、formatter）可用純函式，見下方 helper 門檻。

### 禁止 static helper class

**不准開 `XxxHelper` / `XxxUtils` / `XxxManager` 這類「一堆靜態方法的雜物櫃」。**

看到想寫 helper 的衝動，先照 [architecture.md](architecture.md) 的三步搬家：這個方法的參數屬於誰，就搬進誰裡面。多數 helper 的存在，只是因為行為沒有放回它的資料旁邊。

**真的不得已**才建立 helper 類別，門檻是**全部**符合：

1. 完全無狀態，且**不碰任何領域資料**（碰了就該是 Domain Model 的行為）
2. 沒有任何一個既有物件「擁有」這個行為——搬給誰都不合理
3. 純技術性轉換或框架黏合（格式解析、編碼轉換、字串正規化）

符合的話：

- **一律放 `utilities/` 資料夾**，不散落在各層。
- 命名照該語言慣例，但職責要具體（`DateFormatter` 優於 `CommonUtils`）。
- 不放任何業務規則。**業務規則進了 `utilities/` 就是設計錯了。**


## 金額用精確小數型別

- 價格、金額、停損等**金錢欄位一律用該語言的精確小數型別**（`decimal` / `BigDecimal` / `shopspring/decimal`），**禁用浮點數 float/double**。
- 非金額的權重、比例、分數可用浮點數。

## 錯誤處理

- 對外邊界（外部 API、DB）的錯誤要**包成領域可辨識的錯誤**（哨兵錯誤 / 具名例外），讓 controller 能對映狀態碼，而不是把底層錯誤原封丟出去。
