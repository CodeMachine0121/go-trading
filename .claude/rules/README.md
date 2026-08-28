# Rules

**與程式語言無關**的實作規範（架構、命名、風格、測試、流程），不含任何產品／業務知識。
大小寫、檔名格式、lint 工具一律 follow 各語言自身的慣例；本目錄只規定「怎麼設計、怎麼命名、怎麼測」。

產品知識（專案介紹、環境變數、API 路由、領域詞彙）請另外寫在專案自己的文件，不要放進本目錄，也不要放進 `CLAUDE.md`——`CLAUDE.md` 只負責指路：什麼情境該讀哪一份規則。

| 檔案 | 內容 |
| :--- | :--- |
| [architecture.md](architecture.md) | Clean / Onion 分層、依賴方向、呼叫鏈、`domain/models/` 結構、**Entity 乾淨 + 行為放 Domain Model**、**Domain Model 不是介面抽象**、**model 內禁 static（含 `fromXxx` 轉換）**、禁 private static（搬去參數所屬的 module） |
| [naming.md](naming.md) | 命名規範：角色後綴、entity / `Domain` / `Dto` / `Vo` / `Request` 四種 model 後綴、**介面只抽象行為不抽象資料**、介面以**能力抽象**命名（`I` 前綴）、外部資源一律 `Proxy` 結尾、檔名對齊主型別 |
| [code-style.md](code-style.md) | 跨語言原則：禁任意型別、宣告即賦值、行為掛物件、**禁 static helper class（不得已才放 `utilities/`）**、金額用精確小數 |
| [persistence.md](persistence.md) | **一律 Code First**、schema sync 交給 ORM、禁手寫 SQL/DDL、一 entity 一 repository |
| [testing.md](testing.md) | **只測業務行為**、**只用 mocking 套件 mock 介面（禁手寫 Fake）**、測試力度放大 |
| [background-jobs.md](background-jobs.md) | 背景 job 配置、`IBackgroundJob`、管線輪次記錄機制 |
