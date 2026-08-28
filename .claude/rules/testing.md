# Testing

## 只測業務邏輯行為

**測試的對象是業務行為，不是實作細節。**

- 驗證「給定輸入 → 得到什麼業務結果」，不驗證內部呼叫了哪些私有方法、欄位怎麼存。
- 不為框架、ORM、第三方 SDK 本身寫測試。
- 測試檔一律從**外部視角**（黑箱）import 受測目標，只測公開行為；重構內部實作時測試不該跟著壞。
- 一律 table-driven / 參數化測試，一個案例一組輸入輸出。

## Mocking：只 mock 介面，且只用 mocking 套件

- **一律用 mocking 套件對「介面」產生替身**（NSubstitute、Moq、mockery、Jest mock…）。
- **禁止手寫假物件**——不准出現 `FakeDbContext`、`InMemoryXxxRepository`、`StubPaymentProxy` 這類自己刻的假實作型別。
- 需要被 mock 的東西一定要先有介面（`I` 前綴，放 `domain/interface/`）。沒有介面就先補介面，不要為了測試改用具體型別。
- mock 只設在**最外層的邊界**：repository / proxy（DB、外部 API、時間、亂數）。

## 各層策略

| 層 | 策略 |
| :--- | :--- |
| **Domain Model** | 直接單元測試（業務行為的核心所在，行為都在這裡） |
| **Domain Service** | **不為它定介面**，以具體實例注入。跨物件編排較重的 service 另寫專屬測試，直接注入 mock 的 repository / proxy |
| **Application** | 注入**真實的 domain service（連帶真實 domain model）**，**只 mock 最外層的介面** |

Application 測試會**連帶測到 domain service 與 domain model**——這是刻意的「**測試力度放大**」，不要為了隔離而 mock domain service。

## 測試放置位置

**測試依語言慣例放置，但一律與受測 package/專案分離、用外部黑箱視角**：

- 語言支援同 repo 分層目錄的（如 Go）：各層底下開 `tests/` 子資料夾，測試檔用外部 `<pkg>_test` package，`testdata/` 一併放進去。
- 語言慣用獨立測試專案的（如 .NET / TS）：獨立測試專案，目錄結構鏡射受測專案。

檔名 / package 命名 follow 該語言慣例。
