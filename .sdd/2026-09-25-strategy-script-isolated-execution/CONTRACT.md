# Contract Traceability Matrix — 策略腳本隔離執行

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/infrastructure/script/`（compartment / worker / runner / response / wire）、`cmd/server/`、`internal/config/`
Oracle: Acceptance Criteria（15 clauses）+ Business Rules（4）+ NFR（2）

測試檔簡稱：`CT` = `tests/indicator_script_compartment_test.go`、`WT` = `tests/indicator_script_worker_test.go`、`PT` = `tests/yaegi_indicator_script_proxy_test.go`、`PC` = `tests/yaegi_indicator_script_per_candle_test.go`、`CFG` = `internal/config/tests/application_config_test.go`。

記憶體上限相關的列（AC-5～AC-9）由 CI 在 Linux、不開 race detector 的那一步實測通過（run 36032920025，commit 1bd11bb），不只是讀程式碼推斷。

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 一次指標計算的結果與改版前相同 | 得到 120 | `indicator_script_compartment.go` execute → worker → runner | CT「what a script prints does not spoil its answer」（120）；WT「a single calculation answers with its values」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 逐根重演的結果與改版前相同 | 依序 100、110、120 | compartment executeForEachElement（整段一個子行程） | PC「the candle a run stands on is that run's last one」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | 一次重演只讀一次算式，狀態延續 | 依序 1、2、3 | 一次重演一個 worker，`runner.answer` 只呼叫一次 executeForEachElement | PC「the script is read once and then run…」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | 沒有任何 K 線的重演 | 無結果、無失敗 | response.perElementValues（空） | PC「no candles at all produces no results and no failure」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 一次計算要的記憶體超過上限 | 算式失敗，含「超出記憶體上限」與「512MB」；服務照常 | worker `RLIMIT_DATA`（`indicator_script_worker.go`）+ compartment 讀 stderr 的 out of memory | CT「a single calculation fails and names the cap」（Linux 實測） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 重演的第一根就要超過上限 | 整段失敗、含「超出記憶體上限」、無部分結果 | 同上 | CT「a replay fails as a whole with no partial result」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-7 | 在上限內用大量記憶體照常完成 | 得到 1 | 512MB 上限 + GOMEMLIMIT 80% | CT `TestCompartmentLetsAScriptUseMemoryWithinTheCap` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 一支吃爆記憶體不影響另一支 | 甲「超出記憶體上限」、乙 120 | 每次執行各自一個子行程 | CT「another script running at the same moment is untouched」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 前一個隔間倒下後立刻再算 | 得到 120 | compartment 不留狀態 | CT「the next calculation right after one ran out of memory works」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 算不完的算式在允許時間到時中止 | 約 0.5 秒後「算式在 500ms 內未能算完，已中止」；不留執行中的東西 | runner 的每根允許時間（子行程內）；compartment 的總時限保險 + Kill/Wait/WaitDelay | PT `TestExecuteGivesUpOnAScriptThatNeverFinishes`（訊息、時間）；CT `TestCompartmentLeavesNothingRunningAfterGivingUp`（goroutine 數回到原點，代表子行程已被 Wait 回收） | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 發動計算的人中途離開 | 「算式已中止，因為發動它的請求已經結束」；不留執行中的東西 | compartment `executionContext.Done()` → Kill + Wait | CT `TestCompartmentLeavesNothingRunningAfterGivingUp` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 每一根各自有完整的允許時間 | 一千組結果 | runner 每根各自一份允許時間；總時限 = 允許時間 × 根數 + 5 秒 | CT `TestCompartmentGivesEveryCandleOfALongReplayItsOwnAllowance` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 算式在計算中刻意引發錯誤 | 「算式執行失敗」 | runner 原話 → response.failure | CT「a script that panics on purpose fails while running」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 算式取用沒人宣告的參數 | 指名 period；不是一般算式失敗 | response 帶名稱 → `domains.UndeclaredParameter` | PT:1039-1040（ErrorIs NotDeclared、NotErrorIs ScriptFailed）；WT「a knob nobody declared is answered by name」 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 讀不懂的算式 | 「算式無法解讀」 | runner 原話經回應傳回 | PT 拒絕表格（expectedReason「算式無法解讀」）；WT「a script failing inside the compartment…」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 凡執行指標算式處（計算、重演、機器人；現貨、合約）一律在隔間裡 | 四個 proxy 實例全部走隔間 | `cmd/server/dependencies.go`（四個 proxy 注入同一份 `indicatorScriptIsolation`）；proxy 只持有 compartment | 組裝根無測試；proxy 層由整套 script 測試覆蓋 | no-test（組裝根） | produces-oracle | 🟡 partial |
| BR-2 | 超出上限說「超出記憶體上限」，其他意外說「算式執行失敗」 | 兩種說法分得開 | compartment 依 stderr 分流 | CT `TestCompartmentReportsAnyCompartmentThatGoesDownAsTheScriptFailing`（三種死法） | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 隔間還沒開始算就倒下 → 算式執行失敗 | 「算式執行失敗」 | compartment 啟動失敗分支 | CT 同上「cannot even be started」 | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 服務本身正在關閉 → 進行中的隔間隨之收掉 | 隔間收掉 | 依呼叫端 context 取消而 Kill；容器停止時整個 cgroup 一起結束 | 無 | no-test | unclear | ❔ unclear |
| NFR-1 | 準備隔間每次 ≤ 約 50ms，重演只付一次 | 每次執行的額外開銷在 50ms 內 | 同一個執行檔 re-exec；一次重演一個子行程 | 無計時斷言；非 race 整套約 180 次開子行程共 8 秒（平均每次 < 45ms，含算式本身） | no-test | produces-oracle | 🟡 partial |
| NFR-2 | 任何算式都不能讓服務停止，也不留執行中的東西 | 同 AC-5～11 | 同上；另外子行程的環境變數給空集合 | 同 AC-5～11；「空環境」無測試（外部觀察不到） | asserts-oracle（主體） | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `indicator_script_compartment.go` `command.Env` 空集合 | 子行程不繼承服務的環境變數（資料庫密碼、金鑰） | undocumented in PRD（ARCH 有記載的縱深防禦）；PRD 範圍外但不違反範圍外清單 |
| `indicator_script_runner.go` 直譯器 Stdout/Stderr 丟棄 | 算式印出的文字丟掉 | 對應 PRD「印出任何文字不得混進計算結果」的邊界案例；因為 yaegi 的 `println` 本來就走 stderr，突變測試殺不掉，屬於縱深防禦 |
| `.github/workflows/ci.yml` 不開 race 的記憶體上限驗證步驟 | 在 Linux、不開 race 的條件下證明上限生效，有跳過就判失敗 | 支撐 AC-5～9 的驗證機制，非產品行為 |

## Summary

- Conforms: 18/21 clauses ✅ (86%)
- Violations: —
- Mis-asserted: —
- Partial: BR-1（組裝根無測試，與專案既有慣例一致）、NFR-1（無計時斷言）
- Gaps: —
- Unclear: BR-4（服務關閉時的收尾依賴呼叫端 context 與容器生命週期，讀程式碼無法完全判定）
- Orphans: 3（皆為縱深防禦或驗證機制，不是範圍蔓延）

> Static conformance audit against the Acceptance Criteria。AC-5～AC-9 另有 CI 在 Linux 上的實測佐證。
