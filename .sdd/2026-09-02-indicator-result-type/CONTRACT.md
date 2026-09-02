# Contract Traceability Matrix — 指標值的種類可選

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/domain/models/domains`, `internal/domain/models/dto`, `internal/domain/models/vo`, `internal/infrastructure/script`, `internal/domain/service`, `internal/controller/models`
Oracle: Acceptance Criteria (14 scenarios) ＋ Core Business Rules (8) ＋ Non-Functional (3) = 25 clauses

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 宣告一個數字 | 「均價」為 110，且結果說明種類是「一個數字」 | `indicator_script_shape.go:65` · `indicator_calculation_service.go:59` | `yaegi_indicator_script_proxy_test.go:39` · `indicator_calculation_controller_test.go:186` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 宣告一串數字 | 「均線」依序為 100、105、110，種類是「一串數字」 | `indicator_script_shape.go:65` | `yaegi_indicator_script_proxy_test.go:433` · `indicator_calculation_controller_test.go:148` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 宣告一個是非 | 「黃金交叉」為「是」，種類是「一個是非」 | `indicator_script_shape.go:65` | `yaegi_indicator_script_proxy_test.go:456` · `indicator_calculation_controller_test.go:167` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 宣告一串是非 | 「逐根收紅」依序為是、否、是，種類是「一串是非」 | `indicator_script_shape.go:65` | `yaegi_indicator_script_proxy_test.go:493` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 宣告的種類不在四種之內 | 計算被拒絕、告知四種可選、不回傳任何指標值 | `indicator_result_type_domain.go:36` | `indicator_result_type_domain_test.go:59` · `indicator_calculation_service_test.go:268` · `indicator_calculation_controller_test.go:196` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 完全沒有宣告種類 | 視為「一個數字」，「均價」為 110，種類回報為「一個數字」 | `indicator_result_type_domain.go:41` | `indicator_result_type_domain_test.go:37` · `indicator_calculation_service_test.go:252` · `indicator_calculation_controller_test.go:186` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 宣告一個數字卻產出一串 | 拒絕，說明是算式的問題：宣告一個數字、算式卻產出一串；不回值 | `yaegi_indicator_script_proxy.go:78` | `yaegi_indicator_script_proxy_test.go:584` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 宣告一串是非卻產出數字 | 拒絕，說明是算式的問題：宣告一串是非、算式卻產出數字；不回值 | `yaegi_indicator_script_proxy.go:78` | `yaegi_indicator_script_proxy_test.go:588` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 算式什麼都沒放進結果 | 回傳空的一組結果、不是錯誤，種類仍照宣告回報 | `indicator_script_shape.go:48` | `yaegi_indicator_script_proxy_test.go:534`（以一串是非涵蓋同一條規則） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 某個指標對應到空的一串 | 該指標是空的一串，不視為錯誤 | `indicator_script_shape.go:76` · `indicator_value_dto.go:19` | `yaegi_indicator_script_proxy_test.go:515` · `indicator_value_dto_test.go:44` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 算出「否」不等於沒有值 | 該指標存在且為「否」，不會從結果中消失 | `indicator_script_shape.go:82` · `indicator_value_dto.go:19` | `yaegi_indicator_script_proxy_test.go:475` · `indicator_value_dto_test.go:31` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 宣告一串數字＋根數為 0 | 拒絕，告知計算根數必須大於零 | `indicator_calculation_domain.go:33` | `indicator_calculation_service_test.go:275`（宣告一串數字＋根數 0） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 宣告一個是非＋可用根數不足 | 拒絕，告知目前實際可用幾根 | `indicator_calculation_domain.go:77` | `indicator_calculation_service_test.go:285`（宣告一個是非＋可用不足） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 宣告一串數字時取用的 K 線與種類無關 | 算式拿到排除最新一根後、由早到晚的三根 | `indicator_calculation_domain.go:77` | `indicator_calculation_service_test.go:297`（宣告一串數字，仍是排除最新一根後由早到晚的三根） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 四選一 | 只有四種可宣告，其他一律拒絕 | `indicator_result_type_domain.go:14` | `indicator_result_type_domain_test.go:59` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 預設是一個數字 | 未宣告等同宣告「一個數字」 | `indicator_result_type_domain.go:41` | `indicator_result_type_domain_test.go:37` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 一次一種 | 同一次計算所有指標的值都是同一種 | `indicator_calculation_domain.go:64`（種類掛在一次計算上，無逐指標宣告的路徑） | `yaegi_indicator_script_proxy_test.go:515` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 依宣告驗收 | 算式形狀與宣告不一致即拒絕整次計算 | `yaegi_indicator_script_proxy.go:78` | `yaegi_indicator_script_proxy_test.go:553` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 形狀不符是算式的問題 | 以算式失敗（非請求失敗）呈現 | `yaegi_indicator_script_proxy.go:78` · `indicator_calculation_controller.go:52` | `yaegi_indicator_script_proxy_test.go:612` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-6 | 結果自帶種類 | 結果一併說明這次的指標值種類 | `indicator_calculation_service.go:66` | `indicator_calculation_service_test.go:238` · `indicator_calculation_controller_test.go:161` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-7 | 空的仍合法 | 空的一組結果與空的一串都不是錯誤 | `indicator_script_shape.go:48` | `yaegi_indicator_script_proxy_test.go:515` | asserts-oracle | produces-oracle | ✅ conforms |
| BR-8 | 其餘不變 | 取 K 線、排除最新一根、根數上限、允許時間與可用運算全部照舊 | 未更動 | 既有測試全數保留且通過 | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 不增加額外的資料讀取 | 宣告種類不多讀任何一根 K 線 | `indicator_calculation_service.go:44` | `indicator_calculation_service_test.go:66` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 沙箱範圍不因種類放寬 | 可用運算的白名單與種類無關 | `yaegi_indicator_script_proxy.go:30`（白名單在讀到種類之前就已固定） | `yaegi_indicator_script_proxy_test.go:221` | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-3 | 既有請求不得受影響 | 沒有宣告種類的請求，行為與結果形狀與過去一致 | `indicator_calculation_request.go:11` · `indicator_result_type_domain.go:41` | `indicator_calculation_controller_test.go:186`＋所有未改動的既有測試 | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `indicator_result_type_domain.go:37` | 宣告字串去除前後空白、比對不分大小寫 | 對應 PRD §4 Edge Cases 的寬容度說明，非越界 |
| `indicator_value_dto.go:28-44` | 「一個值」但內容為空時寫出 0／false 而非中斷 | undocumented（防禦性；契約未描述此狀態，由建構點保證不會發生） |

## Summary

- Conforms: 25/25 clauses ✅ (100%)
- Violations: 無
- Mis-asserted: 無
- Partial: 無

**第一輪稽核發現、已於本輪修掉的問題**

| ID | 當時的問題 | 修法 |
| :--- | :--- | :--- |
| AC-12、AC-13、AC-14 | 程式行為正確，但這三條的測試都在「未宣告種類」的前提下驗，沒有釘住 US-05 真正要保證的事：**宣告了種類也一樣**。種類的驗證若哪天被誤搬到根數之前，這三條不會變紅 | 補上三個在宣告種類之下重跑既有規則的測試（根數為零、可用不足、取用哪幾根） |
| BR-3 | 「一次計算只有一種」沒有任何測試釘住，只靠結構上沒有逐指標宣告的路徑 | 補上一個一次算出兩個指標的測試，斷言兩者都是同一種 |
- Gaps: 無
- Unclear: 無
- Orphans: 2

> 本次為靜態稽核：以驗收條件為準比對測試斷言與程式路徑，不執行自行發明的情境。
