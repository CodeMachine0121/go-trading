# Product Requirements Document (PRD) — 外掛授權必須指明對象服務

**Status:** Finalized
**Version:** v1.0
**Owner:** James Hsueh
**Stakeholders:** Engineering

---

## 1. Background & Goal (Why & Goal)

- **Problem Statement:** 「外掛做不了這件事」的網頁專屬動作（設定／移除幣安交易金鑰、打開／關掉自動下單、允許外掛授權）以「登入憑證有沒有標著對象服務」分辨網頁與外掛。但請求外掛授權時對象服務可以不給，不給的外掛換到的登入憑證沒有對象服務標記，與網頁登入發的無從分辨，外掛因此可以直接做網頁專屬的事。
- **Expected Outcome:**
  - 外掛換來或續用出來的登入憑證，**100% 標著對象服務與外掛代號**；
  - 網頁專屬動作對任何外掛憑證一律拒絕；
  - 修正上線前沒有對象服務的外掛登入，再也換不出新的憑證。
  - Claude Code 外掛的連線、換授權、續用流程不受影響。
- **Out of Scope:**
  - 對象服務白名單（營運者設定可接受的對象服務）——不新增設定項。
  - 主動撤銷上線前已發出、尚未過期的操作憑證（最長一個操作憑證有效期限內自然過期）。
  - 憑證查驗回覆內容變更。

---

## 2. User Personas

- **使用者（瀏覽器）：** 在交易服務網頁上登入，做只有本人能做的事。
- **外掛（例如 Claude Code）：** 代使用者操作交易服務；請求授權時帶著對象服務（外掛伺服器的網址）。
- **惡意或不合規的外掛：** 請求授權時不給對象服務，企圖拿到「看起來像網頁登入」的憑證。

---

## 3. User Stories & Acceptance Criteria

### US-01 — 請求授權時一定要指明合格的對象服務 [priority: P0]
**As a** 使用者, **I want** 每一個外掛授權都綁定一個明確的對象服務, **so that** 外掛拿到的憑證永遠不會被當成我在瀏覽器裡的登入。

```gherkin
Scenario: 對象服務是加密協定的完整網址
  Given 已登記的外掛與它登記過的送回地址
  And 對象服務是 "https://trading-mcp.coding-afternoon.com/mcp"
  When 外掛請求授權
  Then 記下一筆待授權請求，使用者被送到網頁的外掛授權頁

Scenario: 本機的對象服務可以用非加密協定
  Given 已登記的外掛與它登記過的送回地址
  And 對象服務是 "http://localhost:8787/mcp"
  When 外掛請求授權
  Then 記下一筆待授權請求，使用者被送到網頁的外掛授權頁

Scenario: 沒有對象服務
  Given 已登記的外掛與它登記過的送回地址
  And 沒有對象服務
  When 外掛請求授權
  Then 使用者被送回外掛，附上「請求無效」、說明「缺少對象服務」與原本的回執記號
  And 沒有記下任何待授權請求

Scenario Outline: 對象服務不是合格的完整網址
  Given 已登記的外掛與它登記過的送回地址
  And 對象服務是 "<對象服務>"
  When 外掛請求授權
  Then 使用者被送回外掛，附上「對象服務不合格」與原本的回執記號
  And 沒有記下任何待授權請求

  Examples:
    | 對象服務                                  |
    | trading-mcp                               |
    | /mcp                                      |
    | http://trading-mcp.example.com/mcp        |
    | https://trading-mcp.example.com/mcp#part  |
    | https://someone:secret@example.com/mcp    |
    | ftp://example.com/mcp                     |

Scenario: 外掛不可信時照舊直接拒絕
  Given 外掛代號不存在
  And 沒有對象服務
  When 外掛請求授權
  Then 直接拒絕，使用者不被送到任何地方
```

### US-02 — 換授權與續用永遠不發沒有對象服務的外掛憑證 [priority: P0]
**As a** 使用者, **I want** 修正前留下的、沒有對象服務的授權碼與外掛登入再也換不出憑證, **so that** 舊漏洞不會延續下去。

```gherkin
Scenario: 綁定對象服務的授權碼換授權
  Given 一張綁定對象服務 "https://trading-mcp.coding-afternoon.com/mcp" 的有效授權碼
  When 外掛以正確的外掛代號、送回地址與挑戰謎底換授權
  Then 外掛拿到一對憑證，登入憑證標著該對象服務與該外掛代號

Scenario: 沒有綁定對象服務的授權碼
  Given 一張沒有綁定對象服務的有效授權碼
  When 外掛以正確的外掛代號、送回地址與挑戰謎底換授權
  Then 拒絕，回覆「授權碼無效」
  And 沒有開出任何外掛登入階段

Scenario: 記著對象服務的外掛登入續用
  Given 一段記著對象服務的外掛登入階段
  When 外掛以它的續用憑證與外掛代號續用
  Then 外掛拿到新的一對憑證，登入憑證仍標著同一對象服務與外掛代號

Scenario: 沒記對象服務的舊外掛登入續用
  Given 一段沒記對象服務的外掛登入階段
  When 外掛以它的續用憑證與外掛代號續用
  Then 拒絕，回覆「授權無效」，外掛須請使用者重新連線
  And 這段登入的換發鏈沒有被作廢
```

### US-03 — 網頁專屬的事拒絕任何外掛憑證 [priority: P0]
**As a** 使用者, **I want** 只有我在瀏覽器裡的登入能設定交易金鑰、切換自動下單、允許外掛, **so that** 外掛不能替我做這些高風險的事。

```gherkin
Scenario: 網頁登入照常通過
  Given 網頁登入發出的登入憑證
  When 使用者設定幣安交易金鑰
  Then 照常進行

Scenario: 標著對象服務的憑證被拒
  Given 標著對象服務的登入憑證
  When 以它設定幣安交易金鑰
  Then 拒絕，要求重新登入

Scenario: 沒有對象服務但標著外掛的憑證被拒
  Given 沒有對象服務、但標著外掛代號的登入憑證
  When 以它打開一台機器人的自動下單
  Then 拒絕，要求重新登入
```

---

## 4. Business Flow & Logic

- **Core Business Rules:**
  - 對象服務必填；合格 = 有協定、有主機的完整網址，協定為加密協定，或指向本機（localhost、127.0.0.1、::1）時可用非加密協定；不得帶頁內位置、不得帶帳號密碼；長度上限沿用「請求內容過長」。
  - 拒絕順序：先驗外掛與送回地址（不可信即直接拒絕），再驗請求內容（回應類型、挑戰、長度、對象服務）。缺少對象服務算「請求無效」，不合格算「對象服務不合格」。
  - 登入憑證上的對象服務只來自使用者允許的那一筆待授權請求；換授權、續用時外掛另外帶來的對象服務不採用。
  - 外掛換來的登入憑證同時標著對象服務與外掛代號；網頁專屬動作看到任一標記即拒絕。
  - 沒有對象服務的授權碼「授權碼無效」；沒有對象服務的外掛登入階段續用「授權無效」，不作廢換發鏈（不是盜用）。
- **Edge Cases:**
  - 修正上線前已發出的外掛操作憑證若沒有對象服務，仍可在其有效期限（預設 15 分鐘）內使用，之後自然失效；其續用被拒，外掛須重新連線。
  - 網頁登入的登入階段續用不受影響。

---

## 5. UI/UX Design & Interaction

N/A（外掛被送回時的錯誤訊息沿用既有「請求無效」格式；新增「對象服務不合格」一種）。

---

## 6. Non-Functional Requirements

- **Security:** 遵循資源指示（resource indicator）標準：對象服務必為完整網址且不含頁內位置；外掛憑證永遠受對象服務限制。
- **Compatibility:** Claude Code 的對象服務 `https://trading-mcp.coding-afternoon.com/mcp` 必須照常通過。

---

## 7. Dependencies & Risks

- **External Dependencies:** 外掛伺服器以憑證查驗比對對象服務，行為不變。
- **Known Risks:** 已連線的外掛若是在修正前連線且當時沒給對象服務，續用會失敗，需請使用者在 /mcp 重新連線一次（Claude Code 一直有給對象服務，不受影響）。

---

## 8. Appendix

- 前一切片：`2026-09-27-connector-authorization`。
