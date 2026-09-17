# 助手拼交易策略並自己回測 — Architecture Design

---

## 1. Design Goal & Guiding Principle

**助手多出來的每一件事，都是既有用例的一層薄包裝。**

`IAssistantQuery` 這個介面存在的理由就是這個：助手能做什麼，等於系統給了它哪幾個
實作，沒有別的。所以這個切片**不動 domain 一個字**——不新增 entity、不新增
domain model、不新增 domain service、不改任何既有規則。

它只做三件事：

1. 新增五個 `IAssistantQuery` 實作，各自呼叫一個既有的 Application method。
2. 在組裝根註冊它們。
3. 把查詢次數上限的預設值由 8 改成 40。

**刪除交易策略不實作，這是設計本身。** 不是加一個 guard clause 擋下來——
是根本沒有那個實作可以被呼叫，因此也沒有任何人能不小心把 guard 拿掉。

---

## 2. Change Scope

| 層 | 動作 |
| :--- | :--- |
| Domain | **完全不動** |
| Application（`assistantqueries/`） | 新增 5 個檔案 + 1 個共用參數 schema 檔 |
| Controller | 不動 |
| Infrastructure | 不動 |
| Config | 1 個預設值 |
| 組裝根 | 註冊 5 筆 |

---

## 3. New Classes / Modules

| 型別 | 檔案 | 職責 |
| :--- | :--- | :--- |
| `TradingStrategyCreateAssistantQuery` | `internal/application/assistantqueries/trading_strategy_create_assistant_query.go` | `create_trading_strategy` → `TradingStrategyApplication.CreateTradingStrategy` |
| `TradingStrategyUpdateAssistantQuery` | `internal/application/assistantqueries/trading_strategy_update_assistant_query.go` | `update_trading_strategy` → `.UpdateTradingStrategy` |
| `TradingStrategyGetAssistantQuery` | `internal/application/assistantqueries/trading_strategy_get_assistant_query.go` | `get_trading_strategy` → `.GetTradingStrategy` |
| `TradingStrategyListAssistantQuery` | `internal/application/assistantqueries/trading_strategy_list_assistant_query.go` | `list_trading_strategies` → `.ListTradingStrategies` |
| `TradingStrategyBacktestAssistantQuery` | `internal/application/assistantqueries/trading_strategy_backtest_assistant_query.go` | `run_trading_strategy_backtest` → `TradingStrategyBacktestApplication.RunTradingStrategyBacktest` |
| `tradingStrategyWriteAssistantArguments`（未匯出） | `internal/application/assistantqueries/trading_strategy_write_assistant_arguments.go` | 拼一份與改一份共用的參數形狀 + JSON schema 常數 |

### 為什麼 write 參數獨立一檔

與策略腳本那一組一模一樣的理由：**拼一份與改一份收的內容完全相同**，
差別只有識別碼是不是必填，而那一句各自在自己的 `ArgumentSchema()` 裡講。
寫兩份的下場是有一天只改了一邊。

`ToWriteDto(id, ownerID)` 由呼叫它的那個能力決定「這是拼還是改」，
**助手自己沒有欄位可以指定擁有者**——與策略腳本同一條。

### 交回助手的形狀

| 能力 | 交回 |
| :--- | :--- |
| 拼／改／讀一份 | `TradingStrategyDto` 的 JSON，經 `renderedTradingStrategy` 統一 |
| 列出 | `{tradingStrategies:[{id,name,sourceLabels,aggregationIntervals}]}` 摘要 |
| 重演 | `BacktestResultDto` **去掉 `EquityCurve`** 之後的 JSON |

**資金曲線由一個獨立的摘要型別擋掉，不是靠 `json:"-"`。** 那個欄位在 HTTP 那條路
上是要回的，動它會傷到畫面。這裡宣告一個只給助手看的形狀，兩條路互不干涉。

---

## 4. Modified Components

| 元件 | 改動 |
| :--- | :--- |
| `cmd/server/dependencies.go` `buildAssistantQueries` | 多收 `*TradingStrategyApplication` 與 `*TradingStrategyBacktestApplication`，註冊 5 筆 |
| `internal/config/application_config.go` | `ASSISTANT_QUERY_LIMIT` 預設 `8` → `40`。**行為一個字都不改** |
| `.sdd/UL-MAP.md` | 「發動助手查詢」補上這五件事與「刪除交易策略辦不到」；查詢次數上限的預設值 |

---

## 5. Component Relationships

```
AssistantConversationService
   └─ []IAssistantQuery
        ├─ TradingStrategyCreateAssistantQuery ─┐
        ├─ TradingStrategyUpdateAssistantQuery ─┼─▶ TradingStrategyApplication
        ├─ TradingStrategyGetAssistantQuery ────┤      （既有，一條規則都不放寬）
        ├─ TradingStrategyListAssistantQuery ───┘
        └─ TradingStrategyBacktestAssistantQuery ──▶ TradingStrategyBacktestApplication
```

依賴方向不變：`assistantqueries` 在 application 層，往下呼叫同層的 Application，
再往下是 domain。**domain 不知道助手存在。**

### 拒絕怎麼走

Application 回的 error 由 `Run` 原封不動往上傳。`IAssistantQuery` 的契約已經寫明
「error 是被拒絕的理由，助手讀得到、可以據以改一改再試，不會終結整次回答」——
這裡什麼都不必做，照契約回就對了。

---

## 6. Extensibility & Handoff Notes

- **自動迭代那個切片**要的是同一組能力，只是驅動它們的不是一次對話而是一件背景工作。
  這五個實作**一個字都不用改**就能被那件工作重用。
- 「一圈約五次查詢」的估算寫在 BRIEF。實際跑過之後要調的是環境變數，不是程式碼。
- **助手可以改一份被執行中機器人引用的交易策略嗎？** 不行，既有規則擋下並說出是哪幾台。
  助手拿到那句話之後該做的是轉達，不是想辦法繞過——`Description()` 裡要講明白。

---

## 7. Traceability

| AC | 落在哪 |
| :--- | :--- |
| US-01 拼一份、歸給委託者 | `trading_strategy_create_assistant_query.go` + `ToWriteDto(0, viewerID)` |
| US-02 改／讀／列出 | update／get／list 三個檔 |
| US-03 刪不掉 | **沒有對應的實作**——這就是它的落點 |
| US-04 重演並讀得懂成績單 | `trading_strategy_backtest_assistant_query.go` + 去掉資金曲線的摘要型別 |
| US-05 被擋下來改一改再試 | `Run` 原封回傳 error；`IAssistantQuery` 既有契約 |
| US-06 一次回答跑好幾圈 | `ASSISTANT_QUERY_LIMIT` 預設值 |

---

## 8. Risks & Open Decisions

| 風險 | 緩解 |
| :--- | :--- |
| 助手拼出「幾乎全在打架」的策略而不自知 | 打架棒數一定交回；`Description()` 裡明講它代表什麼 |
| 40 次查詢全用在重演上會很慢 | 是環境變數，跑過再調 |
| 助手把回測結果當成未來保證 | `Description()` 裡明講「重演的是過去，不保證未來」 |

### Open decisions（交給實作）

- 列出交易策略的摘要要帶多少。建議帶識別碼、名稱、來源代號與彙總刻度——
  刻度帶著，助手一眼看得出哪一份重演不了，不必先讀完整份才發現。
