# Product Requirements Document (PRD)

**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering

---

## 1. Background & Goal

### Problem Statement

「加入」市集策略腳本今天只是一行清單；使用者的交易策略與機器人直接連到作者手上那一支本體。
作者改寫，採用者的機器人無聲換規則；作者收回或刪除，採用者的機器人停擺；不按加入也能用識別碼直接拼進去。

### Expected Outcome

- 加入＝複製一份屬於採用者的策略腳本，只標記「從市集加入」；看不到算式、改不動、不能再發佈、刪得掉。
- 交易策略、交易策略重演與機器人只接受自己的策略腳本；一次性試算不受限。
- 既有資料一次搬成副本；記加入的那份清單與「取消加入」一併移除。
- 助手把副本的失敗當成別人的算式失敗。

### Out of Scope

- 記得副本從哪一支來、新版通知、市集顯示「已加入」、副本改名、看得到副本算式。

---

## 2. User Personas

- **採用者**：在市集加入別人策略腳本，拿去組交易策略與機器人的使用者。
- **發佈者**：把策略腳本放上市集的使用者。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 加入拿到一份自己的副本 [P0]

```gherkin
Scenario: 加入建立一份屬於採用者的副本
  Given 發佈者 B 發佈了策略腳本「動能」
  When 採用者 A 加入它
  Then A 多一支屬於自己的「動能」，標記為從市集加入
  And 它的名稱、說明、指標算式、指標值種類、行情種類與參數與 B 那一支加入當時相同

Scenario: 再加入一次撞名
  Given A 已加入過 B 的「動能」
  When A 再加入一次
  Then 加入被拒絕，並說出「策略腳本名稱「動能」已被使用」
  And A 仍只有一支「動能」

Scenario: 與自己既有的同名
  Given A 自己有一支叫「動能」的策略腳本
  When A 加入 B 的「動能」
  Then 加入被拒絕，並說出「策略腳本名稱「動能」已被使用」

Scenario: 沒發佈的加入不了
  Given B 的「動能」沒有發佈
  When A 加入它
  Then A 得到「找不到」

Scenario: 加入自己的不做任何事
  Given A 發佈了自己的「動能」
  When A 加入它
  Then A 的策略腳本不多也不少

Scenario: 作者改寫、收回或刪除都不影響副本
  Given A 有 B 的「動能」副本
  When B 改寫、收回或刪除原本那一支
  Then A 的副本內容不變，而且跑得動

Scenario: 副本看不到算式
  Given A 有 B 的「動能」副本
  When A 讀它
  Then A 看得到名稱、說明、參數，並知道它從市集加入
  And 看不到指標算式

Scenario: 副本改不動
  Given A 有 B 的「動能」副本
  When A 改寫它
  Then 改寫被拒絕，並說出「從市集加入的策略腳本不能改寫」

Scenario: 副本不能再發佈
  Given A 有 B 的「動能」副本
  When A 發佈它
  Then 發佈被拒絕，並說出「從市集加入的策略腳本不能再發佈」

Scenario: 不要了就刪掉副本
  Given A 有 B 的「動能」副本
  When A 刪除它
  Then 副本被刪除，B 的那一支不受影響

Scenario: 列出可用策略腳本時副本在採用來的那一組
  Given A 有自己的「均線」與 B 的「動能」副本
  When A 列出可用策略腳本
  Then 自己的那一組只有「均線」
  And 採用來的那一組有「動能」，不含算式，用副本自己的識別碼

Scenario: 取消加入不再是一個動作
  When 任何人要求取消加入市集上的一支
  Then 系統沒有這個動作可以做
```

### US-02 — 機器人只依賴自己的策略腳本 [P0]

```gherkin
Scenario: 交易策略可以指名副本
  Given A 有 B 的「動能」副本
  When A 建立信號來源指名那份副本的交易策略
  Then 交易策略被存下來

Scenario: 交易策略不能直接指名別人的
  Given B 的「動能」已發佈，A 沒有副本
  When A 建立或改寫交易策略，信號來源指名 B 那一支
  Then 被拒絕，並說出「這支策略腳本不是你的，請先把它加入你的策略腳本」

Scenario: 重演交易策略時只接受自己的
  Given 一份交易策略的信號來源指名一支不是它擁有者的策略腳本
  When 擁有者重演它
  Then 被拒絕，並說出同一句

Scenario: 機器人一輪遇到不是自己的就停
  Given 機器人引用的交易策略指名一支不是它擁有者的策略腳本
  When 機器人跑一輪
  Then 機器人停擺，原因是某一支策略腳本找不到了

Scenario: 一次性試算照舊可以用別人已發佈的
  Given B 的「動能」已發佈
  When A 用它算指標或跑策略腳本回測
  Then 照常算出結果
```

### US-03 — 既有資料一次搬好 [P0]

```gherkin
Scenario: 既有的加入換成副本
  Given 更新前 A 加入過 B 的「動能」
  When 系統更新
  Then A 有一支從市集加入的「動能」

Scenario: 依賴別人腳本的信號來源改指副本
  Given 更新前 A 的交易策略指名 B 的「動能」
  When 系統更新
  Then 那個信號來源改指 A 的「動能」副本，算式與 B 那一支更新當時相同

Scenario: 既有加入與既有依賴只換成一份
  Given 更新前 A 加入過 B 的「動能」，交易策略也指名它
  When 系統更新
  Then A 只有一份「動能」副本，信號來源指向它

Scenario: 撞名時加上標記
  Given 更新前 A 自己有「動能」，也加入過 B 的「動能」
  When 系統更新
  Then 副本叫「動能（市集）」

Scenario: 再撞名就編號
  Given 更新前 A 已有「動能」與「動能（市集）」，也加入過 B 的「動能」
  When 系統更新
  Then 副本叫「動能（市集 2）」

Scenario: 記加入的清單被移除
  When 系統更新
  Then 記加入的那份清單不再存在

Scenario: 再更新一次什麼都不多
  Given 系統已更新過
  When 系統再啟動一次
  Then 不再多建副本，信號來源不再變動
```

### US-04 — 助手看副本 [P0]

```gherkin
Scenario: 副本的算式失敗時助手只拿到固定一句話
  Given A 有 B 的「動能」副本，執行時以作者寫的一段文字失敗
  When 助手替 A 用它算指標
  Then 助手拿到「這支策略腳本不是你的，它執行失敗，不提供細節」

Scenario: 助手改不了副本
  Given A 有 B 的「動能」副本
  When 助手改寫它
  Then 助手拿到「從市集加入的策略腳本不能改寫」，沒有留下待確認修改
```

---

## 4. Business Flow & Logic

- 副本是一支普通的策略腳本，擁有者是採用者，多一個「從市集加入」標記。
- 副本：讀（不含算式）、執行、刪除；不能改寫、發佈。
- 「別人寫的算式」：不是提問者的，或是從市集加入的。
- 搬移只在需要時做事（冪等）；同一位擁有者對同一支在一次搬移裡只建一份。
- 搬移找不到原本那一支（已刪除），或原本那一支已被作者收回的信號來源，維持原樣——它的機器人本來就已停擺；替它複製一份等於推翻作者收回的決定。
- 撞名加標記時，原名先截短，確保加上標記後仍不超過策略腳本名稱的長度上限。

---

## 5. UI/UX Design & Interaction

- 前端：市集只有「加入」；副本出現在「我的策略腳本」採用來的那一組，刪除即不要了（前端切片另寫）。

---

## 6. Non-Functional Requirements

- **Security:** 副本仍拿不到算式；別人的腳本不再能進入任何機器人的規則。
- **Compatibility:** 可用策略腳本回應的「採用來的」那一組改成副本（與自己的策略腳本同一種形狀，不含算式）；取消加入的路徑移除。

---

## 7. Dependencies & Risks

- 前端與 MCP 需要同步：取消加入改為刪除副本；交易策略要指名副本。

---

## 8. Appendix

- 前一切片：`2026-09-26-assistant-injection-hardening`；市集切片：`2026-09-10-strategy-script-ownership-and-marketplace`
