# Product Requirements Document (PRD) — 多標的機器人的資金分配

**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering（後端先上線；操作台與外掛之後照本文件的約定跟上）

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:** 一台機器人可以盯 1–10 個監聽標的，但部位規劃是「每個標的各自用滿」：部位資金 50、盯 3 個標的，三個同時建議開倉就是 3 × 50。擁有者以為自己只放了 50，實際曝險卻是三倍；開著自動下單的合約機器人會真的在幣安上開出三倍的倉。
- **Expected Outcome:**
  - 部位資金成為**整台機器人的總金額**；每個監聽標的有一個**分配比例**，只能用總金額 × 分配比例那一份。
  - 一台機器人所有標的同時開滿，用掉的資金合計**不超過總金額**（倉位大小模式在各自那一份裡運作）。
  - 建議部位訊息、執行紀錄與自動下單用的是**同一個數字**。
  - 只盯一個標的的機器人（分配 100%）行為與升級前一字不差。
- **Out of Scope:**
  - 檢查幣安帳戶的實際餘額。
  - 沒用到的份額借給別的標的、依信號動態調整分配。
  - 同時持倉數量上限；個別標的覆寫停損、停利、槓桿。

---

## 2. User Personas

- **Primary Role(s):** 機器人擁有者（操作台或外掛上建立、修改機器人；Telegram 上讀建議部位）。
- **Usage Context:** 想用一筆固定的錢讓一台機器人同時盯好幾個標的，並清楚知道每個標的最多會押多少；合約機器人開自動下單時尤其在意總曝險。

---

## 3. User Stories & Acceptance Criteria

### US-01 — [priority: P0] 每個標的只用自己分到的那一份
**As a** 機器人擁有者, **I want** 部位資金是整台的總金額、每個標的只用它分到的那一份, **so that** 一台機器人同時開滿也不會押超過我給的總金額。

```gherkin
Scenario: 全押時押滿分到的那一份
  Given 一台機器人總金額 150、倉位大小全押
  And 監聽標的 BTCUSDT 分配 50%、ETHUSDT 分配 50%
  When 這一輪 BTCUSDT 建議做多
  Then BTCUSDT 的建議開倉金額是 75

Scenario: 百分比模式在分到的那一份裡算
  Given 一台機器人總金額 1000、倉位大小百分比 10
  And BTCUSDT 分配 40%、ETHUSDT 分配 60%
  When 這一輪兩個標的都建議做多
  Then BTCUSDT 建議開倉 40、ETHUSDT 建議開倉 60

Scenario: 只盯一個標的時與從前一字不差
  Given 一台機器人總金額 50、固定金額 50，只盯 BTCUSDT（分配 100%）
  When 這一輪建議做多
  Then 建議開倉 50
  And 訊息不出現任何資金分配的字樣

Scenario: 固定金額超過分到的那一份
  Given 一台機器人總金額 150、固定金額 50
  And BTCUSDT、ETHUSDT、SOLUSDT 各分配 33.33%（各分到 49.995）
  When 這一輪 BTCUSDT 建議做多
  Then 訊息說「這個標的分到的資金不足，押不下 50」，不印開倉金額
  And 合約機器人開著自動下單時也不開倉

Scenario: 沒用到的份額不借給別人
  Given 一台機器人總金額 100、全押
  And BTCUSDT 分配 30%、ETHUSDT 分配 30%（剩 40% 沒動用）
  When 這一輪只有 ETHUSDT 建議做多
  Then ETHUSDT 建議開倉 30

Scenario: 合約機器人的總金額是保證金
  Given 一台合約機器人總金額 100、全押、槓桿 5
  And BTCUSDT 分配 50%
  When 這一輪 BTCUSDT 建議做多
  Then 建議保證金 50、名目 250
  And 自動下單開的數量就是建議部位上的那個數量

Scenario: 分配不是 100% 時訊息說出分配
  Given 一台機器人總金額 1000，BTCUSDT 分配 33.33%
  When 這一輪 BTCUSDT 建議做多
  Then 建議部位多一行「資金分配 33.33%：總金額 1000 中的 333.3」
```

### US-02 — [priority: P0] 建立與修改時給分配比例
**As a** 機器人擁有者, **I want** 建立或修改機器人時替每個監聽標的給分配比例, **so that** 我決定每個標的能用多少。

```gherkin
Scenario: 給每一個標的分配比例
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我給 BTCUSDT 60、ETHUSDT 40 建立機器人
  Then 建立成功，BTCUSDT 分配 60%、ETHUSDT 分配 40%

Scenario: 合計正好 100
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我給 BTCUSDT 50、ETHUSDT 50 建立機器人
  Then 建立成功

Scenario: 合計少於 100
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我給 BTCUSDT 30、ETHUSDT 30 建立機器人
  Then 建立成功，40% 不動用

Scenario: 合計超過 100
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我給 BTCUSDT 70、ETHUSDT 40 建立機器人
  Then 被拒絕，說明「分配比例加起來是 110%，不得超過 100%」

Scenario: 分配比例為零
  When 我給 BTCUSDT 0 建立機器人
  Then 被拒絕，說明「BTCUSDT 的分配比例必須大於 0、不超過 100（單位：%）」

Scenario: 分配比例超過 100
  When 我給 BTCUSDT 100.5 建立機器人
  Then 被拒絕，說明「BTCUSDT 的分配比例必須大於 0、不超過 100（單位：%）」

Scenario: 超過兩位小數
  When 我給 BTCUSDT 33.333 建立機器人
  Then 被拒絕，說明「BTCUSDT 的分配比例最多到小數點後兩位」

Scenario: 只給一部分標的
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我只給 BTCUSDT 的分配比例建立機器人
  Then 被拒絕，說明「ETHUSDT 沒有分配比例——要嘛每個監聽標的都給，要嘛都不給」

Scenario: 指到不在清單上的標的
  Given 監聽標的 BTCUSDT、ETHUSDT
  When 我給 BTCUSDT、ETHUSDT、SOLUSDT 分配比例建立機器人
  Then 被拒絕，說明「SOLUSDT 不是這台機器人的監聽標的」

Scenario: 同一個標的給兩次
  When 我給「btcusdt」50、「BTCUSDT」30 建立機器人
  Then 被拒絕，說明「BTCUSDT 的分配比例給了兩次」

Scenario: 執行中不能修改
  Given 機器人執行中
  When 我修改分配比例
  Then 被拒絕，說明要先停止（照舊）
```

### US-03 — [priority: P0] 沒給時平均分配
**As a** 機器人擁有者, **I want** 不給分配比例時系統替我平均分配, **so that** 不想細調的人也不會不小心押到好幾倍。

```gherkin
Scenario: 一個標的
  When 我只盯 BTCUSDT、不給分配比例建立機器人
  Then BTCUSDT 分配 100%

Scenario: 三個標的
  When 我盯 3 個標的、不給分配比例建立機器人
  Then 每個分配 33.33%，合計 99.99%

Scenario: 七個標的
  When 我盯 7 個標的、不給分配比例建立機器人
  Then 每個分配 14.28%，合計 99.96%

Scenario: 修改時標的不變就保留原本的分配
  Given 已停止的機器人，BTCUSDT 70%、ETHUSDT 30%
  When 我只改名稱、監聽標的不變、不給分配比例
  Then 仍是 BTCUSDT 70%、ETHUSDT 30%

Scenario: 修改時只改了順序也算不變
  Given 已停止的機器人，BTCUSDT 70%、ETHUSDT 30%
  When 我把監聽標的改成 ETHUSDT、BTCUSDT，不給分配比例
  Then 仍是 BTCUSDT 70%、ETHUSDT 30%

Scenario: 修改時標的變了就重新平均
  Given 已停止的機器人，BTCUSDT 70%、ETHUSDT 30%
  When 我把監聽標的改成 BTCUSDT、ETHUSDT、SOLUSDT，不給分配比例
  Then 三個各分配 33.33%
```

### US-04 — [priority: P0] 查機器人看得到分配
**As a** 機器人擁有者, **I want** 查機器人時看到每個監聽標的的分配比例, **so that** 操作台與外掛能顯示、也能原樣改回去。

```gherkin
Scenario: 每個監聽標的都帶著分配比例
  Given 一台機器人 BTCUSDT 60%、ETHUSDT 40%
  When 我查這台機器人
  Then 監聽標的 BTCUSDT 顯示分配 60%、ETHUSDT 顯示分配 40%
```

### US-05 — [priority: P0] 既有機器人升級
**As a** 機器人擁有者, **I want** 升級後我的機器人自動有合理的分配, **so that** 不必一台一台去改。

```gherkin
Scenario: 只盯一個標的
  Given 升級前只盯 BTCUSDT 的機器人
  When 系統升級
  Then BTCUSDT 分配 100%，行為與升級前相同

Scenario: 盯三個標的
  Given 升級前盯 BTCUSDT、ETHUSDT、SOLUSDT 的機器人
  When 系統升級
  Then 三個各分配 33.33%

Scenario: 重複升級不改掉擁有者後來的設定
  Given 已升級過，擁有者把 BTCUSDT、ETHUSDT 改成 70%、30%
  When 系統再升級一次
  Then 仍是 70%、30%
```

---

## 4. Business Flow & Logic

- **Core Business Rules:**
  - **分到的資金＝總金額 × 分配比例 ÷ 100**，不取整（總金額 1000、33.33% → 333.3）。
  - 倉位大小模式、停損、停利、槓桿在分到的那一份裡照舊運作；押不下時說「這個標的分到的資金不足」。只盯一個標的、分配 100% 時沿用原本措辭「部位資金不足」。
  - 每個分配比例 > 0、≤ 100、最多兩位小數；合計 ≤ 100。檢查順序：標的有沒有在清單上與重複 → 每個標的都有 → 單一數值範圍與小數位 → 合計。
  - 平均分配：100 ÷ 標的數，無條件捨去到兩位小數，零頭不分給任何人。
  - 修改時沒給：監聽標的集合與原本相同（順序不論）→ 保留；否則平均分配。
  - 沒有部位規劃（總金額空白）的機器人照樣記著分配比例，只是沒有建議部位可算；之後補上總金額即生效。
  - 分配只影響之後的開倉與建議；已開著的機器人持倉、平倉數量不變（平倉照舊只平機器人持倉那麼多）。
  - 訊息裡的分配行只在分配不是 100% 時出現。
- **Edge Cases:**
  - 升級途中失敗：整筆回到升級前，下次啟動再做。
  - 升級判斷「需要平均分配」的依據是：一台機器人分配比例合計超過 100（升級前每個標的視為 100%）——所以擁有者自己設過的合法分配永遠不會被改。

---

## 5. UI/UX Design & Interaction

- 操作台與外掛之後跟上（另案）：每個監聽標的旁邊一個分配比例輸入；預設顯示平均分配；即時顯示合計與「未動用 X%」；超過 100% 時不讓送出。
- 部位資金欄位標示為「總金額（保證金）」，合約機器人並說明名目＝保證金 × 槓桿。
- 每個標的旁顯示「分到 333.3」＝總金額 × 分配比例 ÷ 100。

---

## 6. Non-Functional Requirements

- **相容性：** 不給分配比例的舊寫法照樣能建立與修改機器人（得到平均分配或保留原本分配）。
- **Performance / Security / Analytics：** N/A（沿用既有機器人的權限與流程）。

---

## 7. Dependencies & Risks

- **Known Risks:** 升級後盯多個標的的機器人每個標的能押的錢會變小（例如固定金額 50、總金額 50、盯 2 個 → 每個只分到 25，押不下）；擁有者需要把總金額調大。這是本次刻意的行為改變。
- 操作台與外掛依本文件約定另行更新；後端先上線且向下相容。

---

## 8. Appendix

- 取代 `.sdd/2026-10-20-multi-symbol-strategy-bot/` 中「部位規劃每個標的各自用滿」的決策。
