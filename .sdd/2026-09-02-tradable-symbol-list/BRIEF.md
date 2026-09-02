# 可查交易標的清單 — Requirements Brief

## Goal

現在要查行情，得先自己知道要查哪一檔——交易標的是一個要手打進去的名字。
打錯一個字母不會有人告訴你打錯了，只會查出「這段區間內沒有 K 線」，
而那句話跟「這檔真的沒資料」長得一模一樣。

這個切片讓系統能回答一個現在沒人問得到的問題：**它手上到底有哪幾檔的行情**。
有了這份清單，問的人就能從裡面挑，而不是憑記憶打字。

## Requirements

- 系統能列出**目前手上握有 K 線的每一個交易標的**。
- 清單依名稱**由小到大**排列，順序每次都一樣。
- 同一個交易標的不論有幾根 K 線，在清單上都**只出現一次**。
- 系統手上一根 K 線都沒有時，回覆**空的清單**，這不是錯誤。
- 這份清單反映的是**實際握有的資料**，不是「打算追蹤哪幾檔」——
  設定上要追蹤、但一根都還沒抓回來的交易標的，不在清單上。

## Examples (Specification by Example)

Each example lists **only** the data that affects the behavior — nothing more.

### Rule: 列出手上握有 K 線的每一個交易標的

| # | Given (only relevant data) | When | Then |
|---|---|---|---|
| 1 (happy) | 系統握有 BTCUSDT 的三根與 ETHUSDT 的一根 | 詢問有哪些交易標的 | 回覆 BTCUSDT 與 ETHUSDT 兩個 |
| 2 (boundary) | 系統握有 BTCUSDT 的一百根，沒有別的交易標的 | 詢問 | 只回覆 BTCUSDT 一個，不是一百個 |
| 3 (boundary) | 系統一根 K 線都沒有 | 詢問 | 回覆空的清單，不視為錯誤 |

### Rule: 順序固定，由小到大

| # | Given (only relevant data) | When | Then |
|---|---|---|---|
| 1 (happy) | 系統握有 SOLUSDT、BTCUSDT、ETHUSDT 的 K 線 | 詢問 | 依序回覆 BTCUSDT、ETHUSDT、SOLUSDT |
| 2 (boundary) | 同一批資料，再問一次 | 再次詢問 | 順序與上一次完全相同 |

### Rule: 清單說的是「有資料」，不是「打算追蹤」

| # | Given (only relevant data) | When | Then |
|---|---|---|---|
| 1 (exception) | 設定上要追蹤 BTCUSDT 與 XRPUSDT，但系統只握有 BTCUSDT 的 K 線 | 詢問 | 只回覆 BTCUSDT |
| 2 (boundary) | 某個交易標的的 K 線全部被刪光 | 詢問 | 它不再出現在清單上 |

## Out of Scope

- 讓使用者新增、修改或移除清單上的交易標的——清單是既有資料的投影，不是一份可以編輯的名單。
- 交易標的名稱的格式規則（長度、大小寫、允許哪些字元）。
- 每個交易標的附帶的其他資訊（有幾根、最新一根是什麼時候、目前價格）。
- 自動抓取的觀察清單——那是另一件事，且它不因這個切片而改變。
- 既有的查詢、新增、修改、刪除 K 線與指標計算，一律不動。

## Open Decisions

Items the PRD author should resolve:

- 無。

## Context / Background

- 這份清單是為了讓操作介面把「交易標的」從一個要手打的欄位變成一個可以挑的清單。
- 之所以取「實際握有資料」而不是「設定上要追蹤」：挑得到卻查不出東西的選項，
  比沒有這個選項更糟——使用者會以為是自己查錯。
