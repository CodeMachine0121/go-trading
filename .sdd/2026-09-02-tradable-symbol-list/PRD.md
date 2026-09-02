# Product Requirements Document (PRD)

**Feature:** 可查交易標的清單
**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** 後端（go-trading）、前端（go-trading-frontend）

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:**
  交易標的目前是一個要手打進去的名字。打錯一個字母不會有人告訴你打錯了，
  只會查出「這段區間內沒有 K 線」——而那句話跟「這檔真的沒資料」長得一模一樣。
  更根本的問題是：系統手上到底有哪幾檔的行情，**現在沒有任何方式問得到**。

- **Expected Outcome:**
  呼叫端能一次拿到「系統握有 K 線的每一個交易標的」，
  於是操作介面可以把那個手打的欄位換成一份挑得到的清單，打錯字這件事從此不會發生。

- **Out of Scope:**
  - 讓使用者新增、修改或移除清單上的交易標的——清單是既有資料的投影，不是一份可編輯的名單。
  - 交易標的名稱的格式規則。
  - 每個交易標的附帶的其他資訊（有幾根、最新一根是什麼時候、目前價格）。
  - 自動抓取的觀察清單——那是另一件事，且不因這個切片而改變。
  - 既有的查詢、新增、修改、刪除 K 線與指標計算。

---

## 2. User Personas

- **Primary Role(s):** 操作介面（代替看行情的人發問），以及直接用工具打 API 的專案作者本人。
- **Usage Context:** 畫面一打開就會問一次，用來把「交易標的」這個欄位填成一份可挑的清單。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 問得到系統手上有哪幾檔 [priority: P0]

**As a** 要查行情的人，**I want** 知道系統握有哪些交易標的，
**so that** 我從裡面挑，而不是憑記憶打字。

```gherkin
Scenario: 列出握有 K 線的每一個交易標的
  Given 系統握有 BTCUSDT 的三根 K 線與 ETHUSDT 的一根 K 線
  When 詢問有哪些交易標的
  Then 回覆 BTCUSDT 與 ETHUSDT 兩個

Scenario: 同一個交易標的只出現一次
  Given 系統握有 BTCUSDT 的一百根 K 線，沒有別的交易標的
  When 詢問有哪些交易標的
  Then 只回覆 BTCUSDT 一個

Scenario: 一根 K 線都沒有時回覆空的清單
  Given 系統一根 K 線都沒有
  When 詢問有哪些交易標的
  Then 回覆一個空的清單，且不視為錯誤
```

### US-02 — 順序每次都一樣 [priority: P0]

**As a** 要查行情的人，**I want** 清單的順序是固定的，
**so that** 我每次打開畫面，同一檔都在同一個位置。

```gherkin
Scenario: 依名稱由小到大
  Given 系統握有 SOLUSDT、BTCUSDT、ETHUSDT 的 K 線
  When 詢問有哪些交易標的
  Then 依序回覆 BTCUSDT、ETHUSDT、SOLUSDT

Scenario: 再問一次順序不變
  Given 系統握有的 K 線沒有任何改變
  When 再詢問一次有哪些交易標的
  Then 順序與上一次完全相同
```

### US-03 — 清單說的是「有資料」，不是「打算追蹤」 [priority: P0]

**As a** 要查行情的人，**I want** 清單上的每一檔都真的查得出東西，
**so that** 我不會挑了一檔卻拿到空的結果、還以為是自己查錯。

```gherkin
Scenario: 設定上要追蹤但還沒有資料的不算
  Given 設定上要追蹤 BTCUSDT 與 XRPUSDT
  And 系統只握有 BTCUSDT 的 K 線
  When 詢問有哪些交易標的
  Then 只回覆 BTCUSDT

Scenario: K 線被刪光之後就不再出現
  Given 系統原本握有 BTCUSDT 與 ETHUSDT 的 K 線
  And ETHUSDT 的 K 線全部被刪除
  When 詢問有哪些交易標的
  Then 只回覆 BTCUSDT
```

---

## 4. Business Flow & Logic

**Flow:**

1. 呼叫端詢問有哪些交易標的。
2. 系統從實際存下的 K 線裡，取出出現過的每一個交易標的，去除重複。
3. 系統依名稱由小到大回覆。

**Core Business Rules:**

- **BR-1 來源**：清單一律取自**實際存下的 K 線**，不取自觀察清單設定。
- **BR-2 去重**：一個交易標的不論有幾根 K 線，都只出現一次。
- **BR-3 排序**：依名稱由小到大，順序固定。
- **BR-4 空集合**：一根 K 線都沒有時是空清單，不是錯誤。

**Edge Cases:**

- 儲存層讀取失敗 → 整次詢問失敗並如實說明，不回覆半份清單。

---

## 5. UI/UX Design & Interaction

本專案不含畫面。清單的呈現由前端負責（見前端的「交易標的選單」切片）。
對本專案而言唯一的介面要求是：空清單與讀取失敗必須分得出來——
前者是正常回覆，後者要明確說是失敗。

---

## 6. Non-Functional Requirements

- **NFR-1 效能**：這是畫面一打開就會問一次的查詢，回應時間需與既有的區間查詢在同一個量級。
- **NFR-2 不留存**：清單不快取、不寫入，每次詢問都反映當下的資料。

---

## 7. Dependencies & Risks

- **External Dependencies:** 無。資料來源是系統自己已經存下的 K 線。
- **Known Risks:**
  - 清單是掃過既有 K 線得出的。資料量成長之後，這個問題會愈來愈貴，
    而它每開一次畫面就被問一次。記在技術設計的風險欄。

---

## 8. Appendix

- 需求共識：`.sdd/2026-09-02-tradable-symbol-list/BRIEF.md`
- 通用語言：`.sdd/UL-MAP.md`（可查交易標的）
- 前端對應切片：`../go-trading-frontend/.sdd/2026-09-02-trading-symbol-picker/`
