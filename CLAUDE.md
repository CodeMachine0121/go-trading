# CLAUDE.md

本檔只做一件事：**告訴你在什麼情境下該去讀 `.claude/rules/` 裡的哪一份規範。**

規則本身與程式語言、框架、產品無關；大小寫、檔名格式、lint 工具一律 follow 各語言自身的慣例。
產品知識（專案介紹、環境變數、API 路由、領域詞彙）請寫在本檔以外的專案文件，不要塞進這裡。

## 動工前必讀

任何時候要寫或改程式碼，先讀這兩份：

- [.claude/rules/architecture.md](.claude/rules/architecture.md) — 東西該放哪一層、哪個資料夾
- [.claude/rules/naming.md](.claude/rules/naming.md) — 東西該叫什麼名字

## 情境對照表

| 我正在做的事 | 去讀 |
| :--- | :--- |
| 決定一段程式碼要放哪一層 / 哪個資料夾 | [architecture.md](.claude/rules/architecture.md) |
| 新增 entity、domain model、DTO、VO | [architecture.md](.claude/rules/architecture.md)、[naming.md](.claude/rules/naming.md) |
| 想把業務邏輯寫進 entity | [architecture.md](.claude/rules/architecture.md)（Entity 保持乾淨，行為放 Domain Model） |
| 出現 `private static` 或只被一處使用的 `private` method | [architecture.md](.claude/rules/architecture.md)（三步搬家 / inline 門檻） |
| 想在 model 上開 `static`（工廠、`fromXxx` 轉換） | [architecture.md](.claude/rules/architecture.md)（model 內一律不得有 static；轉換寫在來源的 `toXxx()`） |
| 幫 Domain Model 或 VO 命名 | [naming.md](.claude/rules/naming.md)（`Domain` / `Vo` 後綴） |
| 想為 Domain Model 定介面或做繼承 | [architecture.md](.claude/rules/architecture.md)（Domain Model 是由 entity 轉換而來的普通 class，不是介面抽象） |
| 想用 `interface` 描述一份資料 | [naming.md](.claude/rules/naming.md)（介面只抽象行為；資料一律 class） |
| Service 要收一組參數 | [naming.md](.claude/rules/naming.md)（封裝成 DTO，不抽介面、不用行內物件型別） |
| 想開 `XxxHelper`、`XxxUtils` 靜態工具類 | [code-style.md](.claude/rules/code-style.md)（原則禁止；不得已才放 `utilities/`） |
| 幫任何類別 / 介面 / 檔案命名 | [naming.md](.claude/rules/naming.md) |
| 要串接外部 API / 第三方服務 | [naming.md](.claude/rules/naming.md)（一律 `Proxy`；介面用能力抽象命名，不綁供應商） |
| 定義介面（interface） | [naming.md](.claude/rules/naming.md)（`I` 前綴、一介面一檔、以能力命名） |
| 存取資料庫、改資料表結構、寫 migration | [persistence.md](.claude/rules/persistence.md)（一律 Code First、schema sync 交給 ORM） |
| 想手寫 SQL 字串 | [persistence.md](.claude/rules/persistence.md)（禁止） |
| 寫測試、需要 mock 東西 | [testing.md](.claude/rules/testing.md)（只測業務行為；只用 mocking 套件 mock 介面，禁手寫 Fake） |
| 寫排程 / 背景工作 | [background-jobs.md](.claude/rules/background-jobs.md) |
| 選型別、宣告變數、處理金額、包錯誤 | [code-style.md](.claude/rules/code-style.md) |

完整索引見 [.claude/rules/README.md](.claude/rules/README.md)。

## 不可妥協的幾條

即使沒讀完全部規則，這幾條一律成立：

1. **依賴方向永遠指向 Domain**，Domain 不認識 HTTP / ORM / 任何 SDK。
2. **Entity 是乾淨的 Data Model**，業務行為放 Domain Model。
3. **行為住在它操作的資料旁邊**——沒有 `private static`、沒有 static helper class。
4. **model 內沒有任何 `static`**。轉換一律寫在來源身上：`a.toB()`，不是 `B.fromA(a)`。
5. **Domain Model 不是介面抽象**，是由 entity 轉換而來的普通 class；`Domain` / `Vo` 後綴一眼可辨。
6. **`interface` 只抽象行為，不抽象資料**。所有 data model 一律 `class`；Service 收的參數用 DTO。
7. **介面以「能力」命名，不以「供應商」命名。**
8. **一律 Code First**，schema 交給 ORM，不手寫 SQL / DDL。
9. **測試只驗業務行為**，mock 只用套件 mock 介面。
