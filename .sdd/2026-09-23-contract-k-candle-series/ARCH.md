# 合約 K 線的彙總序列 — Architecture Design

**Status:** Confirmed · **Source PRD:** `PRD.md`

## 1. Design Goal & Guiding Principle

讓 `GET /contract-k-candles/series` 與 `/k-candles/series` 說同一種話。**刻度的一切都重用** `KCandleSeriesQueryDomain`
（刻度挑選、區間切分、單次上限、該讀幾根原始 K 線），以全天候的市場規則建立；只有「一格怎麼合併」是合約自己的。

## 2. Change Scope

| Area | Action | What / Why |
| :--- | :--- | :--- |
| `domains.KCandleContractSeriesDomain` | **Add** | 依刻度把合約 K 線分格、每格合併成一根，交回 `KCandleContractSeriesDto` |
| `domains.KCandleSeriesQueryDomain` | **Modify** | 多一個 `ContractSeriesOf(candles)`，與 `SeriesOf` 並列 |
| `dto.KCandleContractSeriesDto` | **Add** | symbol、interval、kCandles（`KCandleContractDto`） |
| `KCandleContractService` | **Modify** | 注入 `MarketCatalogDomain`（取全天候市場）；新增 `GetKCandleContractSeries`，驗證錯誤轉成合約的哨兵 |
| `KCandleContractApplication` / `KCandleContractController` | **Modify** | 轉呼叫；`GET /contract-k-candles/series`，參數與現貨相同 |
| 現貨彙總 | **Not touched** | |

## 3. Merge rules（`KCandleContractSeriesDomain`）

- 價量與成交筆數：開最早、收最晚、高最高、低最低、量與筆數加總。
- 標記價格：同樣的開高低收合併（每一根都有）。
- 指數價格、溢價指數：同樣合併，但**格內任一根缺就整格那條線 `null`**。

## 7. Traceability

| PRD | Fulfilled by |
| :--- | :--- |
| US-01 | `KCandleContractSeriesDomain` ＋ `KCandleContractRepository.FindInRange` |
| US-02 | `KCandleSeriesQueryDomain`（重用）＋ service 轉哨兵 ＋ controller |
