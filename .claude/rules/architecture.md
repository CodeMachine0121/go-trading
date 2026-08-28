# Architecture — Clean / Onion Architecture

**依賴方向一律指向 Domain（核心）；Domain 不依賴任何人。** 與語言、框架無關。

```
   Controller ───▶ Application ───▶ Domain ◀─── Infrastructure
   (HTTP/API)       (use cases)     (核心)        (Repository/Proxy 實作)
                                      ▲
                    domain/interface/ 放所有對外介面（一介面一檔）
```

## 各層職責

- **Domain（核心）**：models（entity / domain model / dto / vo）、領域計算邏輯、Domain Service，以及**所有對外介面**。**Domain 不 import 任何其他層**（不認識 HTTP、LLM SDK、ORM…）。
- **Application**：依賴 Domain，呼叫 Domain Service 編排用例，拿回 **DTO**（全程不碰 entity / domain model）。
- **Controller**：依賴 Application，只做請求 / 回應轉換與參數 binding。
  - 允許的例外：controller 可 import domain service 的**哨兵錯誤**做狀態碼對映（400 / 502 分流）。
- **Infrastructure**：Repository（持久化）與 Proxy（外部 API 呼叫）的**實作**，實作 domain 介面（DIP——細節依賴抽象）。
- **組裝根**：唯一知道所有具體型別的地方，負責 DI 組裝、設定讀取、路由註冊。

## Domain 內部結構

domain 底下**統一用一個 `models/` 資料夾**收納所有資料/領域物件，再依種類細分：

```
domain/
├── models/
│   ├── entities/     乾淨的資料模型（Data Model），只有欄位、沒有業務邏輯
│   ├── domains/      Domain Model：業務行為的所在地（充血模型）
│   ├── dto/          domain 對 application 的唯一回傳形狀
│   └── vo/           value object：不可變純資料、無行為
├── service/          Domain Service：application 的唯一呼叫入口，一檔一 service
└── interface/        repository / proxy 介面，一介面一檔（mocks 放子資料夾）
```

## 充血模型：Entity 乾淨，行為放 Domain Model

**我們走充血模型，但行為不寫在 Entity 上。**

| 角色 | 定位 | 內容 |
| :--- | :--- | :--- |
| **Entity**（`models/entities/`） | 乾淨的 **Data Model** | 只有欄位、持久化對應（ORM 標註）。**不放任何業務邏輯、不放計算 method** |
| **Domain Model**（`models/domains/`） | 業務行為的所在地 | 針對某個 entity（或一組 entity）拉出的領域物件，**所有計算、驗證、狀態轉換的 function 都放這裡** |

**Domain Model 是一個由 entity 轉換而來的普通 class，不是介面抽象。**

- **不為 Domain Model 定義介面，也不用介面做繼承。** 它沒有多個實作要替換——它就是那個領域概念本身。需要不同行為時，那是不同的 Domain Model，不是同一個介面的兩種實作。
- **Domain Model 檔內不得宣告任何 `interface`**（這條本來就適用於所有實例檔，見 [naming.md](naming.md)）。參數過多時，把參數群組抽成 **VO**（domain 內部）或 **DTO**（跨 service 邊界），不是抽成 interface。
- 由 entity 進來的路徑一律是 `entity.toDomain()`；要產生 entity 時一律是 `domainModel.toEntity()`。

- 需要對 entity 做任何領域操作時，**另外替它拉一個 Domain Model**，由 Domain Model 持有 entity 資料並提供行為。
- Domain Model 以建構子建立，**建構子內做正規化 / clamp**（非法 enum → 安全預設值、數值 → 合理範圍）。
- **禁止散落的 package-level / static 計算函式當「工具類」（壞味道）**——行為要掛在 Domain Model 上。
- **entity 與 domain model 絕不直接回傳給 application**，一律先轉成 DTO。

## 禁止 private / private static method——把行為搬回它該待的地方

充血模型的重點不是「把 method 塞進某個類別」，而是**讓行為住在它操作的資料旁邊**。類別裡出現 `private static`（或只被自己用的 `private`）method，幾乎都是行為放錯家的訊號。

### model 內一律不得有 `static` method

**entity、domain model、DTO、VO 一律不得有任何 `static` method**，不論公開或私有。model 是資料，不是工具箱。

最常見的違規是**用 static 做 model 之間的轉換**：

```
❌ OrderDomain.fromEntity(order)      ← 轉換掛在目標身上，還是個 static
❌ MoneyVo.fromString("299", "TWD")   ← 名為工廠，實為 static 工具
✅ order.toDomain()                   ← 轉換掛在來源身上，是 instance method
✅ new MoneyVo("299", "TWD")          ← 建構就用建構子，正規化寫在建構子內
```

**轉換的方向決定它住在哪裡：要從 A 變成 B，method 就寫在 A 身上，叫 `toB()`。** 因為轉換讀的是 A 的 property——寫在 B 上就得把 A 整個傳進去，那正是 Feature Envy。

需要在建立時做驗證或正規化（非法 enum → 安全預設值、數值 clamp、格式檢查），**一律寫在建構子裡**，不要為此開一個 static 工廠。

### `private static` 一律不留

看到 `private static`，**不要只是把權限改成 public**（那只會變成一個沒人從外面呼叫的假公開方法）。正確做法是三步：

1. **看它的參數來自哪一個 module** —— 參數是誰，行為就該屬於誰（Feature Envy）。
2. **把參數換成該 module 本身**，並把整個 method **搬進那個 module**。
3. 搬進去之後，method 內部**改用自身的 property 運算**，參數就消失了。

**例：entity → dto 的轉換**

```
❌ service 裡的 private static ToDto(entity)      ← 參數全部來自 entity，行為卻住在 service
✅ entity 自己的 ToDto() → 回傳一個新的 DTO       ← 由 entity 實例呼叫，內部讀自身 property
```

service 拿到 entity 後直接 `entity.ToDto()`，不再自己組裝。**這類「資料形狀轉換」不算業務邏輯**，放在 Entity 上不違反「Entity 保持乾淨」；真正的業務計算、驗證、狀態轉換仍然放 Domain Model。

### `private` method 也要先過門檻

同理，`private` method 不是免費的。抽出來之前先問：**有幾個 public method 在用它？**

| 被幾個 public method 使用 | 做法 |
| :--- | :--- |
| **2 個以上** | 有共用價值，可以留成 private helper |
| **只有 1 個** | **直接 inline 回原本的 method**，不要為了「看起來整齊」硬抽 |

抽 method 的理由只有「消除重複」與「行為屬於別人」，不是「這段太長」。太長是拆物件的訊號，不是拆 private method 的訊號。

## Domain Service 規則

- `Service` 後綴**僅**用於跨 model 的編排（取資料、轉 DTO、多物件 / 併發運算）；單一物件的計算放它自己的 Domain Model。
- **同一個 Domain Service 內的公開 use-case method 互不呼叫。** 需要跨方法編排時一律由 Application 層負責。私有 helper 不在此限。
- 併發編排（同時查多個來源）放 Domain Service，用該語言的標準併發原語。

## 呼叫鏈

```
Application → DomainService → repository/proxy 介面（impl 在 infra）
            → entity → 包成 Domain Model 執行行為 → 轉 DTO → 回傳 DTO
```

## 其他

- **不使用「port」一詞或資料夾**——對外介面一律稱「介面」，集中在 `domain/interface/`。
- 應用程式原始碼放在語言慣用的內部目錄（如 Go 的 `internal/`、.NET 的專案資料夾），**不對外暴露**。
- **`utilities/`**：唯一允許放 helper 類別的地方，且只收「不得已」的純技術性工具（判斷門檻見 [code-style.md](code-style.md)）。預設不該有東西進去。
