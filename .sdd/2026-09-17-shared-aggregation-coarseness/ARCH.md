# Architecture Design — 一份交易策略只看一種粗細

**PRD:** `.sdd/2026-09-17-shared-aggregation-coarseness/PRD.md`
**Status:** Implemented

---

## 1. Design Goal

把「一份交易策略只有一種彙總刻度」變成**一個地方說一次**的規則，
而不是存下去那一關與重演那一關各寫一次。

判準只有一條：**那句拒絕的措辭，兩道關卡必須一字不差。**
它們講的是同一件事；一旦有兩份文字，其中一份遲早會被改到而另一份不會。

---

## 2. Change Scope

### 新增

| 檔案 | 責任 |
| :--- | :--- |
| `internal/domain/models/domains/shared_aggregation_interval_domain.go` | 一組刻度共同的那一個，或一句說出它們有哪幾種的拒絕。**那句話只在這裡寫一次。** |

### 修改

| 檔案 | 改動 |
| :--- | :--- |
| `trading_strategy_signal_sources_domain.go` | 逐一驗完每個來源之後，多問一次：它們共用的是哪一個刻度。 |
| `trading_strategy_backtest_domain.go` | 原本的 package-level `sharedAggregationIntervalOf` 拿掉，改問同一個模型。 |

### 明確不動

- **儲存的形狀**：刻度仍然掛在每一個信號來源那一列上。
  搬去交易策略那一列會是第二次資料搬遷，而它買到的東西——
  「不可能不一致」——已經由這條不變量保證了。
- **讀取與刪除**：舊的不一致資料照樣讀得出來、刪得掉。
- **策略機器人那一端**的任何行為。
- **重演那一關**：它留著，因為舊資料的存在正是它不能拿掉的理由。

---

## 3. Key Decisions

### 為什麼是一個 Domain Model，不是一個共用函式

原本那段判斷是 `trading_strategy_backtest_domain.go` 裡的一個 package-level 函式。
第二個呼叫端出現的那一刻，它就必須有個家——而把它提升成「大家都看得到的工具函式」
只會讓它變成一個沒有主人的計算。

它操作的資料是**一組刻度**，所以它住在那組刻度旁邊。

### 它交出結果，不交出兩個半成品

模型只有一個回答方法，交出「共用的那一個」或「一句說明」，
而不是「它們一致嗎」加上「訊息是什麼」兩個分開的問題。
兩個方法就是兩個可以被配錯的半成品——最糟的那一種配法是
「它們不一致，而這裡沒有任何說明」。

### 錯誤的種類由呼叫端決定

同一句話在兩道關卡是兩種不同的失敗：存下去那一關是交易策略的驗證失敗，
重演那一關是回測的輸入驗證失敗，而且要指名是哪一格。

所以模型只負責**那句話**，兩個呼叫端各自把它包成自己的失敗種類。
這也是為什麼措辭能一致而狀態碼能不同。

### 為什麼不順手把舊資料改成一致

系統改使用者的資料，而使用者不會發現——那是比一句拒絕昂貴得多的事。
一份刻度混著的交易策略讀得出來、刪得掉，只有在他自己要改寫它的時候才被要求調整，
而那時他正看著那張表單。

---

## 4. 下一個需求會打在哪裡

最可能的下一個需求是**允許不同刻度並自動對齊**。

它會打在這個新模型上：那時「共用的那一個」會變成「對齊到哪一個」，
而拒絕變成一個選擇。兩個呼叫端因此不必各自改一次——
這正是把它收成一個模型換來的東西。

---

## 5. Traceability

| PRD 情境 | 由誰滿足 |
| :--- | :--- |
| 都看同一種就存得下來 | `TradingStrategySignalSourcesDomain` |
| 只有一個來源時一定一致 | `SharedAggregationIntervalDomain` |
| 不一致時存不下來，說出有哪幾種 | `SharedAggregationIntervalDomain` + `TradingStrategySignalSourcesDomain` |
| 三種刻度時三種都說得出來 | `SharedAggregationIntervalDomain` |
| 舊的讀得出來、原樣呈現 | 不動讀取路徑 |
| 原樣存回去被擋下 | `TradingStrategySignalSourcesDomain`（建立與改寫同一條路） |
| 重演那一關留著、措辭一致 | `TradingStrategyBacktestDomain` 問同一個模型 |
