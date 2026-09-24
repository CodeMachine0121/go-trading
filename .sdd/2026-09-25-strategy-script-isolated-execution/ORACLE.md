# Oracle — 策略腳本隔離執行

每一列的「預期」**只從 PRD 推導**，在任何實作存在之前寫下。
寫測試時斷言的期望值一律抄這裡，**不准從跑出來的結果回填**。

所有列都透過公開的 proxy（`YaegiIndicatorScriptProxy` / `YaegiContractIndicatorScriptProxy`）驗證：
隔間是 proxy 的內部細節，測試只看得到「給算式與輸入 → 得到結果或失敗」。

---

## A · 結果不變（US-01）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| A1 | 回傳最後收盤價；收盤 100、110、120 | Execute | `close = 120` | 一次指標計算結果不變 |
| A2 | 同上 | ExecuteForEachCandle | `100, 110, 120` | 逐根重演結果不變 |
| A3 | 自己數被跑幾次；三根 | ExecuteForEachCandle | `1, 2, 3` | 一次重演只讀一次算式 |
| A4 | 無 K 線 | ExecuteForEachCandle | 空結果、無錯誤 | 沒有 K 線的重演 |
| A5 | 合約：回傳最後一格資金費率 | Execute（合約 proxy） | 與輸入最後一格相同 | 現貨合約一視同仁 |

## B · 記憶體上限（US-02）— 只在 Linux 上斷言上限生效

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| B1 | 上限 512MB；算式一口氣要 16GB | Execute | `ErrIndicatorScriptFailed`，說法含「超出記憶體上限」與「512MB」 | 超過上限 |
| B2 | 上限 512MB；每根要 16GB；三根 | ExecuteForEachCandle | `ErrIndicatorScriptFailed` 含「超出記憶體上限」；結果為 nil | 重演第一根就超過 |
| B3 | 上限 512MB；配置約 50MB 後回傳 1 | Execute | `value = 1` | 上限內照常完成 |
| B4 | 甲要 16GB、乙回傳 120，同時執行 | 兩個 Execute 並行 | 甲含「超出記憶體上限」；乙 `close = 120` | 不影響另一支 |
| B5 | 剛有一次因超出上限失敗 | 立刻 Execute 回傳 120 的算式 | `close = 120` | 前一個隔間倒下後立刻再算 |
| B6 | 測試所在的行程本身 | B1 之後 | 測試行程仍活著（測試能繼續跑完即證明） | 交易服務照常運作 |

## C · 時間與收乾淨（US-03）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| C1 | 允許 500ms；無窮迴圈 | Execute | 含「未能算完」；耗時 ≥ 500ms 且遠小於 20 倍 | 算不完就中止 |
| C2 | 無窮迴圈；呼叫端 context 在 200ms 後取消 | Execute | 含「發動它的請求已經結束」 | 發動的人中途離開 |
| C3 | C1、C2 之後 | 檢查 | 不留下任何子行程（Wait 已回收；測試行程 goroutine 數不增長） | 收乾淨 |
| C4 | 允許 2s；每根很快；一千根 | ExecuteForEachCandle | 一千組結果 | 每根各自有完整額度 |

## D · 意外倒下是算式失敗（US-04）

| # | Given | When | Then（預期） | 出處 |
| :-- | :--- | :--- | :--- | :--- |
| D1 | 算式 `panic("boom")` | Execute | `ErrIndicatorScriptFailed`，含「算式執行失敗」 | 刻意引發錯誤 |
| D2 | 讀未宣告的 `period` | Execute / ExecuteForEachCandle | `ErrIndicatorParameterNotDeclared`，名稱 `period` | 未宣告參數指名回報 |
| D3 | `this is not go` | Execute | `ErrIndicatorScriptFailed`，含「算式無法解讀」 | 讀不懂的算式 |
| D4 | 算式 `println("noise")` 後正常回傳 120 | Execute | `close = 120` | 印出的文字不混進結果（PRD 邊界案例） |
| D5 | 子行程命令指向不存在的程式 | Execute | `ErrIndicatorScriptFailed`，含「算式執行失敗」 | 隔間還沒開始就倒下 |

> 既有的整套 script 測試（沙箱規矩、信號、參數、指標值種類）改走隔間後**全部維持綠燈**，即證明「其餘行為一字不差」。
