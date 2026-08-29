# 📔 Ubiquitous Language Map

**Project:** go-trading
**Bounded Context:** K 線行情資料（K-Line Market Data）
**Maintainer:** James Hsueh
**Last Updated:** 2026-08-29

> 本文件只記錄**現在有效的詞彙**，不記錄變更歷史。詞彙不再使用就直接刪除該列。

---

## 1. Nouns & Concepts
*Records entities, value objects, attributes and their correspondence between code and real business.*

| Domain Term | Technical Name | User-Facing Label | Definition & Business Rules | Status |
| :--- | :--- | :--- | :--- | :--- |
| K 線 | `KCandle` | K 線 | 市場在一段固定時間內的價量摘要。一根固定涵蓋**五分鐘**，且必定屬於一個交易標的。以「交易標的 + 起始時間」唯一辨識 | Confirmed |
| 交易標的 | *(尚未實作)* | 交易標的 | 這根 K 線所描述的市場，例如 BTCUSDT。每根 K 線必屬於一個交易標的；**不得為空**；查詢時必須指定。名稱格式不另作限制 | Confirmed |
| 起始時間 | *(尚未實作)* | 起始時間 | 一根 K 線所涵蓋的五分鐘從何時開始。必須落在五分鐘刻度上；**不得指向未來**；一律以世界標準時間表示與儲存 | Confirmed |
| 開盤價 | `Open` | 開盤價 | 該五分鐘內第一筆成交的價格 | Confirmed |
| 最高價 | `High` | 最高價 | 該五分鐘內的最高成交價。**不得低於最低價** | Confirmed |
| 最低價 | `Low` | 最低價 | 該五分鐘內的最低成交價 | Confirmed |
| 收盤價 | `Close` | 收盤價 | 該五分鐘內最後一筆成交的價格 | Confirmed |
| 成交量 | `Volume` | 成交量 | 該五分鐘內成交的標的數量 | Confirmed |
| 成交額 | `QuoteVolume` | 成交額 | 該五分鐘內成交的計價金額 | Archeology |
| 主動買入量 | `TakerBuyBaseVolume` | 主動買入量 | 該五分鐘內由買方主動成交的標的數量 | Archeology |
| 主動買入額 | `TakerBuyQuoteVolume` | 主動買入額 | 該五分鐘內由買方主動成交的計價金額 | Archeology |
| 查詢區間 | *(尚未實作)* | 時間區間 | 查詢 K 線時給定的起訖時間。**起訖兩端都包含在內**；起訖本身不必落在五分鐘刻度上 | Confirmed |

---

## 2. Actions & Processes
*Records business operations, function logic, and their corresponding business actions.*

| Business Action | Technical Method | Trigger | Business Impact | Notes |
| :--- | :--- | :--- | :--- | :--- |
| 新增 K 線 | *(尚未實作)* | 使用者提供一根 K 線的資料 | 建立該根 K 線。若該交易標的在該起始時間上已有一根，**覆蓋**舊的而非新增第二根 | 起始時間不在五分鐘刻度、或最高價低於最低價，一律拒絕並說明原因 |
| 查詢 K 線 | *(尚未實作)* | 使用者指定交易標的與查詢區間 | 回傳該區間內的 K 線，依起始時間**由早到晚**排列 | 未指定交易標的、或結束時間早於開始時間則拒絕；區間內無資料回空清單（非錯誤）；超過單次查詢上限則拒絕並請縮小區間 |
| 讀取單一 K 線 | *(尚未實作)* | 使用者以交易標的與起始時間指名 | 回傳該根 K 線 | 不存在時回覆找不到 |
| 修改 K 線 | *(尚未實作)* | 使用者以交易標的與起始時間指名並提供新數字 | 更新該根 K 線的價格與成交數字 | **不得更換交易標的或起始時間**；不存在時回覆找不到 |
| 刪除 K 線 | *(尚未實作)* | 使用者以交易標的與起始時間指名 | 移除該根 K 線 | 不存在時回覆找不到 |

---

## 3. Ambiguities & Conflicts
*Records cases where the same technical term means different things in different modules, or multiple terms refer to the same concept.*

| Ambiguous Term | Meaning in Context A | Meaning in Context B | Resolution |
| :--- | :--- | :--- | :--- |
| 顆粒度 | 一根 K 線本身涵蓋多長時間 | 查詢起訖時間可以精確到多細 | 兩者分開：**K 線固定五分鐘一根**；**查詢起訖可精確到分鐘**，且不必對齊五分鐘刻度 |
| 「量」 | 成交的**標的數量**（成交量、主動買入量） | 成交的**計價金額**（成交額、主動買入額） | 中文一律以「量」指數量、「額」指金額，不混用 |

---

## 4. External & Enum Mapping
*Records magic numbers/strings in code and their real business meaning.*

| Category | Code Value / Key | Domain Label | Description |
| :--- | :--- | :--- | :--- |
| K 線長度 | 五分鐘 | K 線涵蓋時長 | 目前所有 K 線固定為五分鐘一根，尚無其他長度 |
| 起始時間刻度 | `:00, :05, :10, … :55` | 五分鐘刻度 | 起始時間唯一合法的取值；不在刻度上的資料一律拒絕 |
| 單次查詢上限 | `1000` | 單次查詢筆數上限 | 一次查詢最多回傳 1000 根，超過則拒絕並請縮小區間 |
| 價量下限 | `0` | 價量最小值 | 所有價格與成交數字皆不得為負數，違反則拒絕 |
| 時區 | 世界標準時間（UTC） | 時間基準 | 所有起始時間與查詢區間一律以此表示與儲存 |
| 交易標的範例 | `BTCUSDT`、`ETHUSDT` | 交易標的 | 目前僅作為範例，尚未定義名稱格式規則 |

---

## 維護原則

1. **只反映現況**——本文件不記錄變更歷史，詞彙不再使用就刪除該列。
2. **先進地圖，再進程式碼**——新增業務詞彙一律先寫進本文件，不得自創同義詞。
3. **`Archeology` 代表尚未與業務確認**，確認後改為 `Confirmed`。
