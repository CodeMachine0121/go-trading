# Product Requirements Document (PRD)

**Feature:** 交易標的登錄
**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** 後端（go-trading）、前端（go-trading-frontend）

> **本切片推翻了 `.sdd/2026-09-02-tradable-symbol-list/` 的 US-03。**
> 該切片約定「清單說的是有資料，不是打算追蹤」；本切片改為
> 「已登錄的即使沒有資料也算」。理由見下方 Background。
> 先前那份 PRD 保留不改（它記錄的是當時的共識），以本份為準。

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:**
  系統剛裝好、資料庫剛建好的那一刻，一根 K 線都沒有，
  於是「有哪些交易標的」的答案是「一個都沒有」，操作介面上的選單是空的。
  使用者連第一個問題都問不出來，只能等自動抓取跑完第一輪。

  先前的規則之所以會這樣，是因為它把清單定義成「實際握有 K 線的那些」，
  當時的理由是「挑得到卻查不出東西的選項，比沒有這個選項更糟」。
  那個理由在**已經有資料**的系統上成立，在**剛建好**的系統上不成立——
  它的結果是一個選項都沒有，那更糟。而且已登錄的標的正是系統本來就會去抓的那些，等一輪就有資料。

- **Expected Outcome:**
  資料庫一建好，選單就已經有 BTCUSDT 與 ETHUSDT 可挑；
  重跑建立資料庫結構幾次都不會多出重複的登錄。

- **Out of Scope:**
  - 從畫面或介面新增、改名、移除已登錄的交易標的。
  - 讓預設交易標的可設定。
  - 自動抓取的觀察清單——它與登錄是兩件事，不因這個切片改變。
  - 交易標的名稱的格式規則。
  - 既有的查詢、新增、修改、刪除 K 線與指標計算。

---

## 2. User Personas

- **Primary Role(s):** 把系統裝起來的人（專案作者本人），以及代替他發問的操作介面。
- **Usage Context:** 建立資料庫結構是安裝時做一次、之後偶爾重跑的動作；
  重跑必須安全，不能因為跑第二次就壞掉或長出重複資料。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 建好資料庫就認得幾個市場 [priority: P0]

**As a** 把系統裝起來的人，**I want** 建立資料庫結構的同時就登錄預設交易標的，
**so that** 操作介面一打開就有東西可挑，不必等第一批資料。

```gherkin
Scenario: 全新的資料庫
  Given 一個交易標的都沒有登錄過
  When 建立資料庫結構
  Then BTCUSDT 與 ETHUSDT 都被登錄
  And 說明這次新登錄了 BTCUSDT 與 ETHUSDT

Scenario: 重跑一次不會重複登錄
  Given BTCUSDT 與 ETHUSDT 都已經登錄過
  When 再次建立資料庫結構
  Then 兩個都不重複登錄
  And 說明這次沒有新登錄任何一個

Scenario: 只補上缺的那一個
  Given 只有 BTCUSDT 已經登錄過
  When 建立資料庫結構
  Then ETHUSDT 被登錄，BTCUSDT 不重複登錄
  And 說明這次新登錄了 ETHUSDT
```

### US-02 — 清單是「已登錄的」加上「有資料的」 [priority: P0]

**As a** 要讀行情的人，**I want** 清單同時包含系統認得的市場與實際有資料的市場，
**so that** 剛建好的系統有東西可挑，而我手動建的新市場也查得到。

```gherkin
Scenario: 已登錄但還沒有任何資料的也要出現
  Given 登錄了 BTCUSDT 與 ETHUSDT
  And 兩者都還沒有任何 K 線
  When 詢問有哪些交易標的
  Then 回覆 BTCUSDT 與 ETHUSDT

Scenario: 有資料但沒登錄過的也要出現
  Given 登錄了 BTCUSDT
  And 另外有一根 XRPUSDT 的 K 線
  When 詢問有哪些交易標的
  Then 回覆 BTCUSDT 與 XRPUSDT

Scenario: 兩邊都有的只出現一次
  Given 登錄了 BTCUSDT
  And BTCUSDT 也有 K 線
  When 詢問有哪些交易標的
  Then 只回覆 BTCUSDT 一個

Scenario: 依名稱由小到大
  Given 登錄了 SOLUSDT
  And 另外有一根 BTCUSDT 的 K 線
  When 詢問有哪些交易標的
  Then 依序回覆 BTCUSDT、SOLUSDT

Scenario: 兩邊都空時回覆空的清單
  Given 一個交易標的都沒登錄，也沒有任何 K 線
  When 詢問有哪些交易標的
  Then 回覆一個空的清單，且不視為錯誤

Scenario: 已登錄的市場即使資料被刪光也還在
  Given 登錄了 ETHUSDT
  And ETHUSDT 的 K 線全部被刪除
  When 詢問有哪些交易標的
  Then 仍然回覆 ETHUSDT
```

---

## 4. Business Flow & Logic

**Flow（登錄）:**

1. 建立資料庫結構。
2. 讀出目前已經登錄了哪些交易標的。
3. 把預設交易標的裡**還沒登錄的**挑出來，登錄它們。
4. 回報這次新登錄了哪幾個。

**Flow（查詢）:**

1. 讀出已登錄的交易標的。
2. 讀出實際有 K 線的交易標的。
3. 兩邊合併、去重、依名稱由小到大回覆。

**Core Business Rules:**

- **BR-1 預設交易標的**：BTCUSDT 與 ETHUSDT，寫在程式碼裡，目前不可設定。
- **BR-2 登錄的時機**：建立資料庫結構時。
- **BR-3 冪等**：登錄前先確認在不在；重跑幾次結果都一樣，不會長出重複的登錄。
- **BR-4 回報**：每次登錄要說得出這次新登錄了哪幾個（可能是零個）。
- **BR-5 清單來源**：已登錄的 ∪ 有 K 線的，去重、依名稱由小到大。

**Edge Cases:**

- 資料庫讀寫失敗 → 建立資料庫結構整個失敗並如實說明，不留下一半登錄好的狀態說它成功了。
- 兩個建立資料庫結構的動作同時進行 → 不得因為搶著登錄同一個標的而失敗。

---

## 5. UI/UX Design & Interaction

本專案不含畫面。登錄只在命令列上發生，唯一的介面要求是：
執行完要看得出**這次新登錄了哪幾個**，而不是只說「完成」。

---

## 6. Non-Functional Requirements

- **NFR-1 安全重跑**：建立資料庫結構可以重跑任意次數，結果都一樣。
- **NFR-2 查詢成本**：清單查詢多讀一張表，回應時間需與先前在同一個量級。

---

## 7. Dependencies & Risks

- **External Dependencies:** 無。
- **Known Risks:**
  - 已登錄但長期沒有資料的標的會一直出現在清單上，挑了就是「查無 K 線」。
    這是本切片刻意接受的取捨（見 Background）；目前沒有移除登錄的方式，
    需要時再開一個切片處理。

---

## 8. Appendix

- 需求共識：`.sdd/2026-09-02-trading-symbol-registry/BRIEF.md`
- 被本切片推翻的規則：`.sdd/2026-09-02-tradable-symbol-list/PRD.md` 的 US-03
- 通用語言：`.sdd/UL-MAP.md`（已登錄交易標的、預設交易標的、可查交易標的）
