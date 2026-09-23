# Product Requirements Document (PRD)

**Status:** Draft · **Owner:** James Hsueh

## 1. Background & Goal

合約 K 線只能一分鐘一根地查、一次最多一千根，畫不出長時間的圖。讓它與現貨一樣能依刻度彙總。
**Out of Scope**：合約即時跟盤、合約指標計算、現貨彙總的改變。

## 2. User Personas

系統擁有者，透過畫面或助理看合約的圖。

## 3. User Stories & Acceptance Criteria

### US-01 — 合約 K 線依刻度彙總 [P0]

```gherkin
Scenario: 同一格的價量合併
  Given 09:00 到 09:04 五根合約 K 線,開盤最早那根是 100,其中一根最高 120
  When 以五分鐘刻度查 09:00 到 09:05 的合約 K 線序列
  Then 回一根起始時間 09:00 的合約 K 線,開 100、收為 09:04 那根的收、高 120

Scenario: 成交筆數加總
  Given 那五根成交筆數各 7
  When 以五分鐘刻度查
  Then 那一根成交筆數 35

Scenario: 三條價格線各自合併
  Given 那五根裡標記價格最高的是 125,溢價指數最低的是 -0.0009
  When 以五分鐘刻度查
  Then 那一根標記價格最高 125、溢價指數最低 -0.0009

Scenario: 一格裡有舊資料,那條線沒有值
  Given 那五根裡有一根沒有指數價格
  When 以五分鐘刻度查
  Then 那一根的指數價格顯示為沒有值
  And 標記價格照常合併

Scenario: 沒有資料的一格不產出
  Given 09:05 到 09:09 沒有合約 K 線
  When 以五分鐘刻度查 09:00 到 09:10
  Then 只回 09:00 那一根

Scenario: 只讀合約的
  Given 同一時間現貨也有 K 線
  When 查合約 K 線序列
  Then 回來的每一根都是合約 K 線,帶著標記價格
```

### US-02 — 刻度的說法與現貨相同 [P0]

```gherkin
Scenario: 由可顯示根數挑刻度
  Given 查一整天,可顯示 100 根
  When 查合約 K 線序列
  Then 回十五分鐘刻度的序列,並說出用的是十五分鐘

Scenario: 兩種說法同時給
  Given 同時給了刻度與可顯示根數
  When 查合約 K 線序列
  Then 查詢被拒絕,並說明「彙總刻度與可顯示根數只能挑一種說法」

Scenario: 認不得的刻度
  Given 刻度是 2m
  When 查合約 K 線序列
  Then 查詢被拒絕

Scenario: 區間過大
  Given 以一分鐘刻度查一個月
  When 查合約 K 線序列
  Then 查詢被拒絕,並說明時間區間過大

Scenario: 沒指定合約標的
  Given 沒指定合約標的
  When 查合約 K 線序列
  Then 查詢被拒絕,並說明「必須指定交易標的」
```

## 4. Business Flow & Logic

- 刻度挑選、刻度區間切分、單次上限與現貨共用同一套規則；永續合約全天候，每一格都算交易時段。
- 價格線合併：開取最早、收取最晚、高取最高、低取最低；一格裡任何一根缺那條線，那一格那條線就沒有值。

## 5–7

N/A
