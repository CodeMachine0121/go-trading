# Naming Conventions

與語言無關的命名規範。**大小寫、檔名格式一律 follow 該語言自身的慣例**，本檔只規定「名字怎麼取」。

## 通則

- **命名一律全名，避免縮寫**（`InstitutionalScore` 不寫 `InstScore`、`ConfidenceLevel` 不寫 `ConfLvl`）。
- 識別字的大小寫風格 follow 語言慣例（Go：匯出 `PascalCase` / 未匯出 `camelCase`；C#：`PascalCase` / `_camelCase`；TS：`camelCase`…）。
- DB 表 / 欄位用該 ORM 的預設慣例，名稱本身一樣要全名不縮寫。

## 各層角色固定後綴

| 角色 | 後綴 | 層 |
| :--- | :--- | :--- |
| Domain service | `Service` | domain |
| Application | `Application` | application |
| Controller | `Controller` | controller |
| Repository | `Repository` | infrastructure |
| Proxy | `Proxy` | infrastructure |

`Service` 後綴**僅**用於跨物件的編排；單一物件的計算放它自己的 Domain Model。

### 外部資源一律用 `Proxy`

**只要是透過 API 呼叫外部資源 / 服務，封裝它的物件一律以 `Proxy` 結尾。** 不論對方是 REST、gRPC、GraphQL、訊息佇列或第三方 SDK。

- **介面**用「做什麼事」命名：`I{能力}Proxy`，放 `domain/interface/`。
- **實作**用「誰來做」命名：`{Provider}{能力}Proxy`，放 infrastructure。
- **不使用 `Client`、`Gateway`、`Adapter`、`Api`、`Connector` 等同義後綴。**
- **一個外部資源一個 Proxy**：所有對該資源的呼叫都從這個 proxy 出去，其他層一律透過介面呼叫，不得在 application / domain 直接碰第三方 SDK。
- Proxy 負責把外部回傳的原始格式**正規化成 VO** 再往內傳，不把 wire 格式漏進 domain。

只有存取自家資料庫的持久化物件用 `Repository`；其餘外部資源都是 `Proxy`。

## 資料物件命名

| 類別 | 規則 | 位置 |
| :--- | :--- | :--- |
| **Entity**（乾淨 Data Model，只有欄位） | 以領域語彙命名，**不加任何後綴** | `domain/models/entities/` |
| **Domain Model**（行為所在地） | **`Domain` 後綴** | `domain/models/domains/` |
| **DTO**（service 回傳給 application 的純資料） | `Dto` 後綴 | `domain/models/dto/` |
| **VO**（不可變、無行為的值物件） | **`Vo` 後綴** | `domain/models/vo/` |
| **Request**（endpoint 接收的 body） | `Request` 後綴 | controller 同層 |

- querystring / route 參數**不立 struct/class**，直接在 handler 內逐一解析。
- 回應**直接回傳 DTO**，不另立 `Response` 型別。

### 四種 model 的後綴一眼可辨

同一個領域概念會同時存在四種形狀，後綴就是它們的身分證——看到名字就知道它屬於哪一層、能不能帶行為、能不能離開 domain：

```
Order           entity      只有欄位與持久化標註
OrderDomain     行為         由 entity 轉換而來，業務規則住在這裡
OrderDto        對外形狀      domain 交給 application 的唯一形狀
MoneyVo         值           不可變、無行為
```

**Entity 是唯一不帶後綴的**，因為它是那個領域概念的本體；其餘三種都是它的某種投影，各自加上後綴表明自己是什麼。

## 介面：以「能力」抽象命名，不以「供應商」命名

- `interface` 只用於「行為契約」，一律 `I` 前綴。
- **集中放在 `domain/interface/`，一個介面一個檔案。**
- **實作檔內不得宣告任何 `interface`。**
- 不使用「port」一詞或資料夾。

### 介面只抽象「行為」，絕不抽象「資料」

**`interface` 的唯一用途是抽象業務層的 method——repository、proxy 這類「做什麼事」的契約。**
**資料一律用 `class` 定義，不論它是 entity、domain model、DTO、VO 還是 service 的參數。**

```
✅ interface IPaymentProxy { createCheckout(...) }   ← 行為契約，有多個實作要替換
❌ interface OrderSummary { orderId, amount }        ← 這是資料，該用 class
✅ class OrderSummaryDto { 建構子可驗證、可帶 toXxx() 轉換 }
```

判準很簡單：**它有沒有「多個實作要替換」？** 沒有就不該是 interface。一份資料形狀永遠只有一種樣子，抽成 interface 只是把 class 少寫了一個字，換來的是它從此無法帶建構子驗證、無法帶 `toXxx()` 轉換、也無法在執行期辨認型別。

**Service 接收的物件一律用 DTO。** 需要一組參數時，把它們封裝成 `XxxDto`，不要抽 interface，也不要直接用行內的匿名物件 / 字典型別當參數。DTO 因此是雙向的——它既是 domain 交給 application 的回傳形狀，也是 application 交給 domain 的輸入形狀。

### 核心原則

**介面命名要 focus 在「這個能力是什麼」，而不是「現在是誰在提供」。**

供應商（Google、Amazon、SendGrid…）只是眾多可替換實作的其中之一。介面若綁死供應商，換一家就得多開一個介面，抽象就失效了。

```
❌ IGoogleProxy   ← 綁死供應商，來了 Amazon 就得再開 IAmazonProxy
✅ IMapProxy      ← 綁定能力，GoogleMapProxy / AmazonMapProxy 都能實作同一個介面
```

| 介面（能力，抽象） | 實作（供應商，具體） |
| :--- | :--- |
| `IMapProxy` | `GoogleMapProxy`、`AmazonMapProxy` |
| `IObjectStorageProxy` | `AmazonObjectStorageProxy`、`AzureObjectStorageProxy` |
| `IEmailProxy` | `SendGridEmailProxy`、`SesEmailProxy` |
| `IPaymentProxy` | `StripePaymentProxy`、`NewebPaymentProxy` |

判斷方式：**如果明天換一家供應商，介面名字要不用改**——改到了就代表抽象層次不對。

### 同樣原則套用到所有角色

介面命名一律 follow **該角色自身的命名規則**，只是把具體實作細節抽掉：

| 角色 | 介面 | 實作 |
| :--- | :--- | :--- |
| Application | `I{用例}Application` | `{用例}Application` |
| Repository | `I{聚合}Repository` | `{聚合}Repository`（不叫 `ISqlOrderRepository`） |
| Proxy | `I{能力}Proxy` | `{Provider}{能力}Proxy` |
| Background job | `IBackgroundJob` | `{工作}Job` |

Repository 的抽象維度是**聚合 / 資料**（`IOrderRecordRepository`），不是資料庫技術；Application 的抽象維度是**用例**，不是實作手段。

## 具體實作：純角色名 + 必要的辨識前綴

實作**不帶 `I`**，也**不帶技術前綴**：

- `IOrderRecordRepository` 的實作是 `OrderRecordRepository`（**不是** `EfOrderRecordRepository` / `GormOrderRecordRepository`）

唯一可加的前綴是**供應商名**，且僅在該介面確實會有多個供應商實作時：

- `IMapProxy` → `GoogleMapProxy`、`AmazonMapProxy` ✅（供應商是有意義的區分）
- `IMapProxy` → `MapHttpProxy` ❌（HTTP 是傳輸細節，不是區分維度）

若目前只有單一供應商且短期不會換，實作可直接用純角色名（`PaymentProxy`），日後要接第二家時再加供應商前綴。

## 檔名

**檔名格式 follow 語言慣例**（Go：`snake_case.go`；C#：`PascalCase.cs`；TS：依專案既有風格），但一律遵守：

- **檔名必須對齊其主要型別 / 符號**，一檔一主角。
- 介面檔獨立成檔，檔名帶 `I` 前綴（依語言慣例轉寫，如 `i_payment_proxy.go` / `IPaymentProxy.cs`）。
- 實作檔用實作本身的名字（純角色名，或帶供應商前綴）。
- mock 檔由 mocking 工具產生時，沿用工具的預設命名，不手改。

## 業務詞彙

**不得自創同義詞。** 若專案有維護通用語言地圖（UL-MAP / glossary），新增業務詞彙一律先進地圖，再進程式碼。
