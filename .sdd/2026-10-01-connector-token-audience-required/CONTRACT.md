# Contract Traceability Matrix — 外掛授權必須指明對象服務

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/`（domains / service / security），`cmd/server`
Oracle: Acceptance Criteria + Business Rules + NFR（20 clauses）

縮寫：`DT` = `internal/domain/models/domains/tests/connector_authorization_domain_test.go`、`AT` = `internal/application/tests/connector_authorization_application_test.go`、`CT` = `internal/controller/tests/connector_authorization_controller_test.go`。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 對象服務是加密協定的完整網址 | 記下待授權請求、使用者被送到網頁授權頁 | connector_authorization_start_domain.go:53 (不拒) | AT:`a complete request…`；DT `complete` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 本機的對象服務可以用非加密協定 | 記下待授權請求（對象服務原樣）並送到授權頁 | connector_resource_domain.go:15 | AT:216；DT:585 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 沒有對象服務 | 送回外掛，附請求無效、「缺少對象服務」、回執記號；不記請求 | connector_authorization_start_domain.go:51 | AT:185（strict mock，無 Save）；CT:299 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 對象服務不是合格的完整網址（6 例） | 送回外掛，附對象服務不合格與回執記號；不記請求 | connector_resource_domain.go:15、start_domain.go:53 | DT:585（6 例全覆蓋）、DT:619（完整 redirect）、AT:185 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 外掛不可信時照舊直接拒絕 | 直接拒絕，不送任何地方 | connector_authorization_service.go `StartConnectorAuthorization`（client/redirect 先驗） | AT:234；CT `an unknown connector without a resource` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 綁定對象服務的授權碼換授權 | 一對憑證，登入憑證標著對象服務與外掛代號 | connector_authorization_code_domain.go `ToAccessTokenClaims`；service:273 | AT:610（Issue 精確 claims）；DT:636 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 沒有綁定對象服務的授權碼 | 授權碼無效；不開登入階段 | connector_authorization_code_domain.go:65 | AT:658（strict mock，無 Redeem）；DT `a code bound to no resource` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 記著對象服務的外掛登入續用 | 新的一對憑證，仍標同一對象服務與外掛代號 | session_domain.go `ToAccessTokenClaims`；user_service.go:277 | AT:828（Issue 精確 claims） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 沒記對象服務的舊外掛登入續用 | 授權無效；換發鏈未作廢 | user_service.go:258 | AT:886（strict mock，無 Rotate/RevokeChain）；CT:551（400 invalid_grant） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 網頁登入照常通過 | 照常進行 | user_service.go:395 | authentication_middleware_test.go:161；user_application_test.go:764 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 標著對象服務的憑證被拒 | 拒絕，要求重新登入 | user_service.go:395 | cmd/server/binance_trading_key_sign_in_test.go:11（真 JWT、401） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 沒有對象服務但標著外掛的憑證被拒 | 拒絕，要求重新登入 | user_service.go:395；jwt_access_token_proxy.go:95 | cmd/server/binance_trading_key_sign_in_test.go:11（auto-order、真 JWT、401）；middleware:161 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 對象服務必填且格式規則（含長度沿用「請求內容過長」） | 規則逐條成立；過長仍為請求無效 | start_domain.go:48-54；resource_domain.go | DT:585、DT `an overlong resource` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 修正前記下、沒有對象服務的待授權請求視同不存在 | 不能允許，不發授權碼 | connector_authorization_request_domain.go `Open` | AT `a request recorded without a resource cannot be approved…`；DT `recorded without a resource` | asserts-oracle | produces-oracle | ✅ conforms（code review 後補） |
| BR-2 | 拒絕順序：外掛與送回地址先於請求內容 | 不可信即直接拒絕 | service `StartConnectorAuthorization` | AT:234 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 憑證上的對象服務只來自允許的那一筆；換授權、續用另帶的不採用 | token 請求帶別的 resource，憑證仍為原對象服務 | `ConnectorTokenRequest` 不綁定 resource；code/session `ToAccessTokenClaims` | CT:523（契約檢查時補上）；CT:551 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 外掛憑證同時標對象服務與外掛代號；網頁專屬看到任一即拒 | 兩種標記皆寫入並讀回；任一即拒 | jwt_access_token_proxy.go:47/95；user_service.go:395 | jwt_access_token_proxy_test.go:197；user_application_test.go:764 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 無對象服務的授權碼「授權碼無效」；無對象服務的外掛登入續用「授權無效」、不作廢鏈 | 同 AC-7、AC-9 | 同上 | 同上 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 對象服務必為完整網址且不含頁內位置 | `#` 一律不合格（含空 fragment） | resource_domain.go:19 | DT:585 `a fragment`/`an empty fragment` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | Claude Code 的對象服務照常通過 | `https://trading-mcp.coding-afternoon.com/mcp` 不被拒 | resource_domain.go | DT:585 `the hosted connector server`；DT 啟動表 `complete` | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| — | 無。憑證查驗回覆未變（Out of Scope 遵守）；未新增 env var（白名單 Out of Scope 遵守） | — |

## Summary

- Conforms: 20/20 clauses ✅ (100%)
- Violations: —
- Mis-asserted: —
- Partial: BR-3 原為 🟡（沒有測試證明 token 請求另帶的 resource 不被採用），已補 CT:523 與 CT:551 後轉 ✅
- Gaps: —
- Unclear: —
- Orphans: 0

> Static conformance audit：依 PRD 預期判讀測試斷言與程式路徑，不以全套測試通過為依據。
