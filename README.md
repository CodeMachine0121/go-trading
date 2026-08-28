# claude-rules

一組**與程式語言無關**的 coding 規範，給 [Claude Code](https://claude.com/claude-code) 讀。
拉進任何新專案，AI 就照同一套架構、命名、測試原則寫程式。

## 為什麼

每開一個新專案就要重講一次「entity 要乾淨」「介面別綁供應商」「不要開 Helper」很煩。
這個 repo 把這些規則寫死一次，之後複製過去就好。

規則只談**怎麼設計、怎麼命名、怎麼測**，不談產品、不綁語言。
大小寫、檔名格式、lint 工具一律 follow 各語言自身的慣例。

## 怎麼用

把兩個東西複製到新專案根目錄：

```bash
git clone https://github.com/<your-account>/claude-rules.git /tmp/claude-rules
cp -r /tmp/claude-rules/CLAUDE.md /tmp/claude-rules/.claude /path/to/your-project/
```

就這樣。Claude Code 啟動時會自動讀 `CLAUDE.md`，再依情境去翻對應的規則檔。

專案自己的產品知識（介紹、環境變數、API 路由、領域詞彙）**另外寫在專案文件裡**，
不要塞進 `CLAUDE.md`——它只負責指路。

## 內容

```
CLAUDE.md              情境 → 該讀哪份規則（Claude Code 的入口）
.claude/rules/
├── README.md          規則索引
├── architecture.md    Clean/Onion 分層、domain/models/ 結構、充血模型
├── naming.md          角色後綴、介面抽象命名、Proxy、檔名
├── code-style.md      型別、變數宣告、static helper、金額、錯誤處理
├── persistence.md     Code First、ORM schema sync、禁手寫 SQL
├── testing.md         只測業務行為、mocking 策略
└── background-jobs.md 背景 job 配置、管線輪次記錄
```

## 核心主張

| 主張 | 白話 |
| :--- | :--- |
| 依賴方向永遠指向 Domain | Domain 不認識 HTTP / ORM / 任何 SDK |
| Entity 乾淨，行為放 Domain Model | Entity 只有欄位；業務邏輯另外拉一個物件裝 |
| 行為住在它操作的資料旁邊 | 沒有 `private static`、沒有 `XxxHelper` 雜物櫃 |
| model 內沒有 `static` | 轉換寫在來源身上：`a.toB()` ✅　`B.fromA(a)` ❌ |
| 介面只抽象行為，不抽象資料 | `IPaymentProxy` ✅　`interface OrderSummary { ... }` ❌，資料一律 class |
| 介面綁「能力」，不綁「供應商」 | `IMapProxy` ✅　`IGoogleProxy` ❌ |
| 外部服務一律 `Proxy` 結尾 | 不用 Client / Gateway / Adapter |
| 一律 Code First | schema 由 code 定義，sync 交給 ORM |
| 測試只驗業務行為 | mock 只用套件 mock 介面，禁手寫 `FakeDbContext` |

規則不是教條。跟專案現實衝突時，**改規則、進 commit**，不要讓程式碼跟文件漂移。

## 客製

- 不需要的規則直接刪檔，記得同步改 `CLAUDE.md` 的情境對照表與 `.claude/rules/README.md`。
- 要加規則就開新檔案放進 `.claude/rules/`，並在那兩個地方各補一行。
- 團隊各自的 workflow（commit 節奏、分支策略）**刻意不放這裡**——因人而異，寫在專案自己的文件。
