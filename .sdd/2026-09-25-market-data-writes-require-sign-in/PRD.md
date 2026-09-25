# Product Requirements Document (PRD)

**Feature:** 改動行情資料須先登入
**Status:** Draft
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:**
  行情資料今天**看得到也改得動**，而且兩件事都不必說出你是誰。
  任何找得到這個系統的人都能新增、修改、刪除 K 線，叫系統回補、做歷史同步，
  或增減觀察清單與合約追蹤名單。每一位使用者的重演與每一台策略機器人讀的都是**同一份行情**，
  一個陌生人改掉的一根收盤價，會悄悄進到所有人的結果裡。

- **Expected Outcome:** 可驗證的結果有五項：
  1. 現貨與合約兩邊**每一件改動行情資料的事**，沒有目前登入者就被擋下，並說明要**先登入**。
  2. 有目前登入者但**待開通**時被擋下，附上**同一段開通指示**——不是「請先登入」。
  3. 被擋下的改動**一點都沒有發生**。
  4. **已開通**的使用者做這些事，規則、數字與回覆**與今天完全一樣**。
  5. **看行情資料**（K 線、彙總 K 線、單根 K 線、即時跟盤、交易標的清單、合約那邊同樣的查詢與合約補充資料）
     **仍然不必登入**，而且帶著壞掉的身分證明也照常回答。

- **Out of Scope:**
  - 角色、管理者身分、「誰能改得比別人多」。
  - 記下是誰改了哪一根 K 線。
  - 系統自己排定的抓取（每分鐘一輪、啟動回補、即時跟盤存入走完的那一根），照常進行。
  - 即時跟盤耗用來源連線的上限。
  - 每位使用者各自一份觀察清單。

---

## 2. User Personas

- **Primary Role(s):**
  - **已開通的使用者**——透過操作台或代操外掛維護行情：補資料、同步歷史、決定追蹤哪些標的。
  - **沒有登入的訪客**——只看行情圖、跟盤；**不再能改任何東西**。
  - **待開通的使用者**——登入得進來，但改不了行情，被告知該怎麼開通。
- **Usage Context:**
  改動行情的動作發生在操作台的行情頁與代操外掛的工具裡，都是已經登入的人在用；
  看行情的動作則常常是沒有登入的人在看圖。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 只有說得出自己是誰、而且已開通的人能改行情 [priority: P0]

**As a** 已開通的使用者，**I want** 行情資料只有已開通的使用者改得動，
**so that** 我的重演與機器人讀到的行情不會被一個不知道是誰的人動過。

```gherkin
Scenario: 已開通的使用者新增一根 K 線
  Given alice@example.com 已開通、已登入
  When 她新增一根 BTCUSDT 的 K 線
  Then 那根 K 線照常存下，回覆與今天相同

Scenario: 沒有登入就新增 K 線
  Given 沒有目前登入者
  When 新增一根 BTCUSDT 的 K 線
  Then 被拒絕，說明要先登入
  And 那根 K 線沒有被存下

Scenario: 帶著過期的身分證明刪除 K 線
  Given 請求帶著一份已過期的登入憑證
  When 刪除一根 BTCUSDT 的 K 線
  Then 被拒絕，說明要先登入
  And 那根 K 線仍在

Scenario: 待開通的使用者修改 K 線
  Given bob@example.com 待開通、已登入
  When 他修改一根 BTCUSDT 的 K 線
  Then 被拒絕，附上開通指示，而不是「請先登入」
  And 那根 K 線沒有被改

Scenario: 已開通的使用者手動回補
  Given alice@example.com 已開通、已登入
  When 她手動回補 BTCUSDT
  Then 照常回補並回覆補進幾根

Scenario: 沒有登入就手動回補
  Given 沒有目前登入者
  When 手動回補 BTCUSDT
  Then 被拒絕，說明要先登入
  And 沒有向行情來源抓任何東西

Scenario: 已開通的使用者啟動歷史同步
  Given alice@example.com 已開通、已登入
  When 她啟動 BTCUSDT 回溯 30 天的歷史同步
  Then 照常收下並回覆那筆歷史同步輪次

Scenario: 沒有登入就啟動歷史同步
  Given 沒有目前登入者
  When 啟動 BTCUSDT 回溯 30 天的歷史同步
  Then 被拒絕，說明要先登入
  And 沒有記下任何歷史同步輪次

Scenario: 沒有登入就查歷史同步進度
  Given 一趟歷史同步正在跑
  And 沒有目前登入者
  When 查那一趟走到哪裡
  Then 被拒絕，說明要先登入

Scenario: 已開通的使用者查歷史同步進度
  Given 一趟歷史同步正在跑
  And alice@example.com 已開通、已登入
  When 她查那一趟走到哪裡
  Then 照常回覆已完成幾段、共幾段

Scenario: 已開通的使用者把標的加進觀察清單
  Given alice@example.com 已開通、已登入
  When 她把 ETHUSDT 加進觀察清單
  Then ETHUSDT 照常變成追蹤中

Scenario: 沒有登入就從觀察清單移除
  Given ETHUSDT 在觀察清單上
  And 沒有目前登入者
  When 把 ETHUSDT 從觀察清單移除
  Then 被拒絕，說明要先登入
  And ETHUSDT 仍在觀察清單上

Scenario Outline: 合約那邊的改動與現貨一模一樣
  Given 沒有目前登入者
  When <合約改動>
  Then 被拒絕，說明要先登入

  Examples:
    | 合約改動 |
    | 新增、修改或刪除一根合約 K 線 |
    | 手動回補一個合約標的 |
    | 啟動一趟合約歷史同步，或查它的進度 |
    | 把合約標的加進或移出合約追蹤名單 |
```

### US-02 — 看行情不必登入 [priority: P0]

**As a** 沒有登入的訪客，**I want** 照常看行情圖與跟盤，
**so that** 行情這份公開的市場事實不因為這道門而少了一個用途。

```gherkin
Scenario: 沒有登入照常查 K 線
  Given 沒有目前登入者
  When 查 BTCUSDT 一段時間的 K 線、彙總 K 線或單獨一根
  Then 照常回答

Scenario: 沒有登入照常跟盤
  Given 沒有目前登入者
  When 即時跟盤 BTCUSDT
  Then 照常開始跟盤

Scenario: 沒有登入照常查交易標的清單
  Given 沒有目前登入者
  When 查交易標的清單或合約交易標的清單
  Then 照常回答，含誰在追蹤中

Scenario: 沒有登入照常查合約補充資料
  Given 沒有目前登入者
  When 查合約 K 線、資金費率結算、維持保證金階梯或持倉統計
  Then 照常回答

Scenario: 帶著壞掉的身分證明看行情
  Given 請求帶著一份已過期的登入憑證
  When 查 BTCUSDT 一段時間的 K 線
  Then 照常回答，不因那份憑證被擋
```

---

## 4. Business Flow & Logic

- **Core Business Rules:**
  - **改動行情資料**＝新增／修改／刪除 K 線、手動回補、啟動歷史同步、查歷史同步進度、增減觀察清單；
    合約那邊的同一組事也是。每一件都**先確認目前登入者且已開通**，才做任何事。
  - 查歷史同步進度歸在「改」這邊：它回答的是某人發動的一件改動做到哪了，不是市場事實；
    輪次依序編號，開著等於讓人翻出誰在補哪些標的、在哪裡壞掉。
  - 拒絕的說法沿用既有兩句：「請先登入」與「尚未開通＋開通指示」。
  - 已開通的使用者之間**沒有差別**；沒有「這份行情是誰的」這種歸屬。
- **Edge Cases:**
  - 憑證在一趟歷史同步跑到一半時過期：**那一趟照跑**（它不屬於任何請求），只是再查進度要換一份新的憑證。
  - 系統自己排定的抓取不經過這道門，照常進行。

---

## 5. UI/UX Design & Interaction

- 本系統這一側無畫面。操作台與代操外掛各自決定沒有登入時怎麼提示；它們必須**先**改成帶著身分證明做這些事。

---

## 6. Non-Functional Requirements

- **Security:** 改動行情一律要先登入且已開通；看行情不必登入。
- **Performance:** 看行情不因這道門多一次檢查。

---

## 7. Dependencies & Risks

- **External Dependencies:** 操作台與代操外掛必須先上線帶身分證明的版本；否則這一側上線的那一刻，它們的回補、同步與觀察清單操作會全部被擋下。
- **Known Risks:** 任何已開通的使用者仍然改得動全系統共用的一份行情——這次擋的是「不知道是誰」，不是「不夠格」。

---

## 8. Appendix

- 帳號開通把關切片（2026-09-19）明說留給「另一件事」的，就是這一塊。
