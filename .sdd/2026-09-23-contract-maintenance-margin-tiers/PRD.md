# Product Requirements Document (PRD)

**Status:** Draft
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering

---

## 1. Background & Goal (Why & Goal)

### Problem Statement

合約交易規格只記最小那一級的維持保證金率。一筆大部位實際落在更高的一級，要求的保證金比例更高；
只用最小那一級去算，強制平倉價會被算得太遠，重演出來的結果比真實樂觀。完整分級只有帶帳戶身分才問得到。

### Expected Outcome

- 有帳戶金鑰時，系統認得的每個合約標的都帶著**整組維持保證金分級**，每天刷新，加入追蹤名單時立刻補上。
- 沒有金鑰時系統行為與這一刀之前一字不差。
- 指定合約標的查得到它的整組分級。

### Out of Scope

- 用分級計算強制平倉價、合約重演使用它。
- 需要交易或提款權限的操作。
- 分級的變更歷史。

---

## 2. User Personas

**Primary Role:** 系統擁有者（唯一使用者）。他會去申請一把**唯讀**帳戶金鑰放進設定，之後不需要做任何事。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 有金鑰才抓分級 [priority: P0]

**As a** 系統擁有者，**I want** 有金鑰時分級自己抓回來、沒金鑰時什麼都不受影響，**so that** 我可以先上線、晚點再補金鑰。

```gherkin
Scenario: 有金鑰時啟動就抓
  Given 設定了帳戶金鑰
  When 系統啟動
  Then 系統認得的每個合約標的都有整組分級

Scenario: 沒有金鑰時不抓也不報錯
  Given 沒有設定帳戶金鑰
  When 系統啟動
  Then 沒有抓分級,也沒有分級相關的錯誤紀錄

Scenario: 沒有金鑰時加入追蹤名單照常
  Given 沒有設定帳戶金鑰
  When 把 BTCUSDT 加進合約追蹤名單
  Then 加入成功,其餘資料照常補齊
  And BTCUSDT 沒有分級

Scenario: 有金鑰時加入追蹤名單立刻補上分級
  Given 設定了帳戶金鑰
  When 把 BTCUSDT 加進合約追蹤名單
  Then 加入成功
  And BTCUSDT 立刻有整組分級
```

### US-02 — 刷新整組替換，出事保留原本的 [priority: P0]

**As a** 系統擁有者，**I want** 分級永遠是交易所最新說的那一組，出事時也不會被清掉，**so that** 我查到的一定是一組完整、說得通的分級。

```gherkin
Scenario: 新的一組整組替換舊的
  Given BTCUSDT 原本有 3 級
  And 來源現在給 BTCUSDT 2 級
  When 刷新維持保證金分級
  Then BTCUSDT 只剩那 2 級,確認時間是這一次

Scenario: 來源不答話
  Given 來源不答話
  When 刷新維持保證金分級
  Then 每個標的保留原本的分級,並留下失敗紀錄

Scenario: 金鑰被拒
  Given 來源拒絕了這把金鑰
  When 刷新維持保證金分級
  Then 每個標的保留原本的分級,並留下說出金鑰被拒的紀錄

Scenario: 來源不再列出的標的保留最後一次
  Given OMGUSDT 有分級
  And 來源這一次沒有列出 OMGUSDT
  When 刷新維持保證金分級
  Then OMGUSDT 保留原本的分級與確認時間

Scenario: 一個標的的分級說不通時只有它不更新
  Given 來源給 BTCUSDT 的第 2 級下限低於第 1 級上限
  And 來源給 ETHUSDT 一組說得通的分級
  When 刷新維持保證金分級
  Then BTCUSDT 保留原本的分級,並留下「分級重疊」的紀錄
  And ETHUSDT 換成新的一組

Scenario: 只因存過合約 K 線而被列出的標的沒有分級
  Given 1000PEPEUSDT 只因存過合約 K 線而出現在合約標的清單上,從沒登錄
  When 刷新維持保證金分級
  Then 1000PEPEUSDT 沒有分級
```

### US-03 — 每一級的合法性 [priority: P1]

```gherkin
Scenario: 一般的一級
  Given 第 1 級名目 0 到 50000、維持保證金率 0.004、速算額 0、最高 125 倍
  When 刷新維持保證金分級
  Then 那一級被存下

Scenario: 名目上限不大於下限
  Given 某一級的名目上限等於下限
  When 刷新維持保證金分級
  Then 那個標的整組不更新,並留下「名目上限必須大於下限」的紀錄

Scenario: 維持保證金率不在零與一之間
  Given 某一級的維持保證金率是 1.2
  When 刷新維持保證金分級
  Then 那個標的整組不更新,並留下「維持保證金率必須介於零與一之間」的紀錄

Scenario: 速算額為負
  Given 某一級的速算額是 -1
  When 刷新維持保證金分級
  Then 那個標的整組不更新,並留下「維持保證金速算額不得為負」的紀錄

Scenario: 最高槓桿小於一倍
  Given 某一級的最高槓桿是 0
  When 刷新維持保證金分級
  Then 那個標的整組不更新,並留下「最高槓桿至少一倍」的紀錄

Scenario: 一級都沒有
  Given 來源給某個標的空的一組
  When 刷新維持保證金分級
  Then 那個標的整組不更新,並留下「至少要有一級」的紀錄
```

### US-04 — 查得到 [priority: P0]

```gherkin
Scenario: 查一個有分級的標的
  Given BTCUSDT 有 3 級
  When 查 BTCUSDT 的維持保證金分級
  Then 回那 3 級,由第 1 級到第 3 級,每一級帶著確認時間

Scenario: 查一個還沒抓過的標的
  Given ETHUSDT 還沒抓過分級
  When 查 ETHUSDT 的維持保證金分級
  Then 回空的,不算錯誤

Scenario: 沒指定合約標的
  Given 沒指定合約標的
  When 查維持保證金分級
  Then 查詢被拒絕,並說明「必須指定交易標的」
```

---

## 4. Business Flow & Logic

- **一次問全部**：來源一次回所有合約的分級，只存系統認得（已登錄）的標的。
- **整組替換**：一個標的的新分級在同一次寫入裡替換掉舊的整組，不會出現新舊混在一起。
- **逐標的判斷**：一個標的的分級說不通只影響它自己。
- **金鑰不外洩**：金鑰與密鑰不出現在任何紀錄或回應。

## 5. UI/UX Design & Interaction

N/A

## 6. Non-Functional Requirements

- 與交易規格共用合約行情的請求額度；一天一次加上每次加入名單一次，請求量可忽略。
- 只需要唯讀權限。

## 7. Dependencies & Risks

- **外部依賴**：幣安 USDⓈ-M 合約的帳戶端點，需要 API key 與簽名。
- **風險**：金鑰若誤設成有交易權限，系統本身不會用到，但洩漏的後果較大——文件提醒只開唯讀。

## 8. Appendix

- 需求共識：`BRIEF.md`
- 前一刀：`.sdd/2026-09-23-contract-market-supplements/`
