# Product Requirements Document (PRD)

**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:** 策略腳本同時被拿來「畫在 K 線圖上看」與「當交易策略的信號來源算買賣」。兩種腳本混在同一份清單，挑信號來源時看到畫線用的腳本，挑圖表指標時看到算信號用的腳本，數量一多就難找也容易選錯。
- **Expected Outcome:** 每一支策略腳本標明**用途**（圖表用／計算用）；交易策略、標的篩選、直接回測只接受計算用；圖表只列圖表用；擁有者可隨時切換用途（包含市集副本）。既有腳本一律為計算用。
- **Out of Scope:** 依指標值種類自動推測用途；新的用途種類；前端畫面與外掛（各自專案）。

---

## 2. User Personas

- **Primary Role(s):** 策略腳本擁有者；從市集加入副本的使用者；透過外掛代他操作的助理。
- **Usage Context:** 在網頁的腳本編輯器、交易策略工作台、K 線圖表，或透過外掛。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 建立時決定用途 [priority: P0]
**As a** 策略腳本擁有者, **I want** 建立腳本時標明它的用途, **so that** 它只出現在對的地方。

```gherkin
Scenario: 建立圖表用腳本
  Given 一支新腳本，用途說「圖表用」
  When 擁有者建立它
  Then 建立成功，用途為圖表用

Scenario: 建立時沒說用途
  Given 一支新腳本，沒有提到用途
  When 擁有者建立它
  Then 建立成功，用途為計算用

Scenario: 用途不在兩種之內
  Given 一支新腳本，用途說「回測用」
  When 擁有者建立它
  Then 建立被拒絕，並說明用途只能是圖表用或計算用
```

### US-02 — 切換用途 [priority: P0]
**As a** 策略腳本擁有者, **I want** 隨時切換腳本的用途, **so that** 我可以重新整理自己的腳本，包括從市集加入的副本。

```gherkin
Scenario: 沒人在用的腳本切成圖表用
  Given 一支計算用的腳本，沒有任何交易策略或篩選設定在用
  When 擁有者把它切成圖表用
  Then 切換成功，用途為圖表用

Scenario: 交易策略在用時不能切成圖表用
  Given 一支計算用的腳本，交易策略「4H 趨勢」在用
  When 擁有者把它切成圖表用
  Then 切換被拒絕，並說出交易策略「4H 趨勢」正在用這支腳本

Scenario: 篩選設定在用時不能切成圖表用
  Given 一支計算用的腳本，篩選設定「新幣」在用
  When 擁有者把它切成圖表用
  Then 切換被拒絕，並說出篩選設定「新幣」正在用這支腳本

Scenario: 圖表用切回計算用
  Given 一支圖表用的腳本
  When 擁有者把它切成計算用
  Then 切換成功，用途為計算用

Scenario: 切成原本的用途
  Given 一支計算用的腳本
  When 擁有者把它切成計算用
  Then 切換成功，用途仍為計算用

Scenario: 市集副本也能切換
  Given 一支計算用的市集副本，沒有人在用
  When 擁有者把它切成圖表用
  Then 切換成功，用途為圖表用
  And 副本的指標算式仍然看不到也改不了

Scenario: 別人的腳本
  Given 一支屬於別人的腳本
  When 使用者要切換它的用途
  Then 切換被拒絕，說法與找不到這支腳本相同

Scenario: 改寫內容時順便送用途
  Given 一支自己的腳本
  When 擁有者改寫它的內容時一併送來用途
  Then 改寫被拒絕，並說明請改用切換用途
```

### US-03 — 計算的地方只接受計算用 [priority: P0]
**As a** 交易策略擁有者, **I want** 只有計算用的腳本能當信號來源, **so that** 畫線用的腳本不會被誤拿去算買賣。

```gherkin
Scenario: 計算用的信號腳本當信號來源
  Given 交易策略的信號來源指名一支計算用、信號種類的腳本
  When 擁有者存下交易策略
  Then 存下成功

Scenario: 圖表用的腳本當信號來源
  Given 交易策略的信號來源指名一支圖表用的腳本
  When 擁有者存下交易策略
  Then 存下被拒絕，並說明這支腳本是圖表用，不能當信號來源

Scenario: 篩選設定指名圖表用的腳本
  Given 篩選設定指名一支圖表用的腳本
  When 擁有者存下篩選設定
  Then 存下被拒絕，並說明這支腳本是圖表用，不能用於篩選

Scenario: 直接回測圖表用的腳本
  Given 一支圖表用的腳本
  When 擁有者直接回測它
  Then 回測被拒絕，並說明圖表用的腳本不能回測

Scenario: 計算用但不是信號種類
  Given 交易策略的信號來源指名一支計算用、但指標值種類是一串數字的腳本
  When 擁有者存下交易策略
  Then 存下照舊被拒絕，並說明信號來源必須是信號種類
```

### US-04 — 圖表只列圖表用、試算兩種都行 [priority: P1]
**As a** 看盤的使用者, **I want** 圖表的指標選單只列畫線用的腳本, **so that** 清單乾淨好找。

```gherkin
Scenario: 圖表用的腳本出現在圖表選單
  Given 一支圖表用、畫得成線的腳本
  When 使用者打開圖表的指標選單
  Then 這支腳本出現在選單裡

Scenario: 計算用的腳本不出現在圖表選單
  Given 一支計算用、畫得成線的腳本
  When 使用者打開圖表的指標選單
  Then 這支腳本不出現在選單裡

Scenario: 計算用的腳本照樣能試算
  Given 一支計算用的腳本
  When 擁有者單純算一次指標
  Then 計算不因用途被拒絕，並回覆這一次算出的指標結果
```

### US-05 — 用途看得到、市集帶著走 [priority: P1]
**As a** 市集使用者, **I want** 看到腳本的用途並在加入時帶走, **so that** 加入的副本放在對的地方。

```gherkin
Scenario: 查詢腳本看得到用途
  Given 一支圖表用的腳本
  When 擁有者查詢它或列出可用的腳本
  Then 看到它的用途是圖表用

Scenario: 市集顯示用途
  Given 發佈者的腳本是圖表用
  When 其他使用者在市集瀏覽它
  Then 看到它的用途是圖表用

Scenario: 加入時副本帶著用途
  Given 市集上一支圖表用的腳本
  When 使用者加入它
  Then 副本的用途是圖表用

Scenario: 發佈者之後切換不影響副本
  Given 使用者已加入一支圖表用腳本的副本
  And 發佈者之後把原腳本切成計算用
  When 使用者查看自己的副本
  Then 副本的用途仍是圖表用
```

### US-06 — 既有資料升級 [priority: P0]
**As a** 既有腳本擁有者, **I want** 升級後所有腳本都能照舊被交易策略使用, **so that** 我的機器人不會因為升級而停擺。

```gherkin
Scenario: 既有畫線用腳本
  Given 一支升級前就存在、畫線用的腳本
  When 系統升級
  Then 它的用途是計算用

Scenario: 既有市集副本
  Given 一支升級前就存在的市集副本
  When 系統升級
  Then 它的用途是計算用

Scenario: 升級重做不覆蓋擁有者的切換
  Given 升級後擁有者把一支腳本切成圖表用
  When 升級再做一次
  Then 它的用途仍是圖表用
```

---

## 4. Business Flow & Logic

- **Core Business Rules:**
  - 用途是「多加的一道」檢查：交易策略信號來源、篩選設定、直接回測除了原有規則（信號種類、行情種類、擁有權）之外，再要求計算用。
  - 機器人每一輪讀規則、交易策略回測讀規則時照舊經過信號來源的檢查；因為被引用的腳本不能切成圖表用，正常情況下不會在這裡被擋。
  - 「單純算一次指標」不檢查用途（作者試算，也是圖表套用指標時實際的計算方式）；圖表只列圖表用是選單層面的規則。
- **Edge Cases:**
  - 舊的畫面在改寫腳本內容時送來用途 → 被拒絕並指向切換用途（不默默吞掉）。
  - 切換用途同時有人正要把它加進交易策略 → 存交易策略時的檢查會擋下圖表用的腳本，兩邊都不會留下不一致的引用。

---

## 5. UI/UX Design & Interaction

N/A（畫面在前端專案）。

---

## 6. Non-Functional Requirements

- **Compatibility:** 升級可重複執行、結果不變；既有交易策略、篩選設定、機器人升級後全部照常運作（所有腳本皆為計算用）。

---

## 7. Dependencies & Risks

- **Dependencies:** 前端（圖表選單、腳本編輯器、切換用途）與外掛（腳本清單、建立、切換用途）須同步改版。
- **Known Risks:** 升級後畫線用腳本暫時不在圖表選單，需擁有者手動切換——使用者已知悉並接受。

---

## 8. Appendix

- `BRIEF.md`（同資料夾）
