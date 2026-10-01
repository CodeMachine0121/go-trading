# Contract Traceability Matrix — 幣安自動下單設定

Contract: PRD.md
Design map: ARCH.md
Implementation: `internal/` (+ `cmd/server/dependencies.go` routes)
Oracle: Acceptance Criteria (45 clauses: 38 AC, 5 BR, 2 NFR)

Abbreviations: `app/` = `internal/application/tests/`, `ctl/` = `internal/controller/tests/`, `dom/` = `internal/domain/models/domains/tests/`, `ent/` = `internal/domain/models/entities/tests/`, `per/` = `internal/infrastructure/persistence/tests/`, `exc/` = `internal/infrastructure/exchange/tests/`, `srv/` = `cmd/server/`.
Strict mocks (`gomock`) fail a test on any un-expected call, so "Binance was not asked / nothing was written" is asserted by leaving that mock without expectations.

## Clauses

| ID | Clause | Spec-expected (oracle) | Impl | Test | Test audit | Code audit | Status |
|----|--------|------------------------|------|------|------------|------------|--------|
| AC-1 | 第一次設定（例 1） | 成功；讀回結尾 a1b2、可交易市場只有現貨 | `service/binance_trading_key_service.go` SaveTradingKey | `app/binance_trading_key_application_test.go` "a first key Binance accepts for spot…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-2 | 再設定一次是覆蓋（例 2） | 仍只一組；讀回結尾 c3d4 | `persistence/binance_trading_key_repository.go` Replace (ON CONFLICT user_id) | `per/` TestBinanceTradingKeyRepositoryReplaceKeepsOneKeyPerPerson | asserts-oracle | produces-oracle | ✅ conforms |
| AC-3 | API Key 不得為空白（例 3） | 拒絕，「必須給 API Key」；沒問幣安 | `domains/binance_trading_key_domain.go` | `app/` "blank strings are refused…"; `ctl/` "a blank string is a bad request" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-4 | Secret Key 不得為空白（例 4） | 拒絕，「必須給 Secret Key」；沒問幣安 | same | `app/` same table; `dom/` TestNewBinanceTradingKeyDomainRefusesBlankStrings | asserts-oracle | produces-oracle | ✅ conforms |
| AC-5 | 前後空白不予保留（例 5） | 成功；問幣安的是去空白後兩串；結尾無空白 | `domains/binance_trading_key_domain.go` | `app/` "Binance is asked with both strings trimmed" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-6 | 沒有登入就讀不了、存不了、移除不了（例 6） | 三者皆拒絕，「請重新登入」 | `middlewares/authentication_middleware.go` | `ctl/binance_trading_key_controller_test.go` 401 cases; `srv/binance_trading_key_sign_in_test.go` | asserts-oracle (status; message is the shared middleware message tested beside the middleware) | produces-oracle | ✅ conforms |
| AC-7 | 讀回只看得到結尾、市場、時刻（例 7） | 看到 a1b2、現貨、時刻；無完整 API Key、無 Secret Key | `entities/binance_trading_key.go` ToDto | `ctl/` "the owner reads the tail…" (exact JSON); `ent/` | asserts-oracle | produces-oracle | ✅ conforms |
| AC-8 | 很短的 API Key 仍然遮到拼不回去 | 4 字金鑰讀回看不到那 4 字 | `domains/binance_trading_key_domain.go` ToEntity | `dom/` "a four-character key shows no tail" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-9 | 留存處沒有可以直接拿去用的金鑰 | 留存內容找不到兩串原文 | service seals both before Replace | `app/` first case asserts stored = sealed(...) values | asserts-oracle | produces-oracle | ✅ conforms |
| AC-10 | 系統開不了鎖時寧可拒絕（例 8） | 拒絕，「系統目前無法安全保存幣安交易金鑰」；沒存；沒問幣安 | service SaveTradingKey seal branch | `app/` "without a sealing key…"; `ctl/` 503 case | asserts-oracle | produces-oracle | ✅ conforms |
| AC-11 | 看不到別人的（例 9） | 乙看到還沒設定 | repository FindOneByUser by user_id | `per/` TestBinanceTradingKeyRepositoryFindOneByUserSaysNothingIsThere | asserts-oracle | produces-oracle | ✅ conforms |
| AC-12 | 還沒設定過不是錯誤 | 回「還沒有設定」，不算失敗 | service GetTradingKey | `app/` "never having stored one is not a failure"; `ctl/` 200 configured:false | asserts-oracle | produces-oracle | ✅ conforms |
| AC-13 | 現貨與合約都開（例 10） | 成功；現貨、合約 | verification domain + entity | `app/` "a key with both markets…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-14 | 只開合約（例 11） | 成功；只有合約 | same | `app/` "a contract-only key…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-15 | 兩種交易權限都沒開（例 12） | 拒絕「沒有任何交易權限」；舊的不動 | `domains/binance_trading_key_verification_domain.go` | `app/` refusal table; `dom/` messages; `ctl/` 422 noTradingPermission | asserts-oracle | produces-oracle | ✅ conforms |
| AC-16 | 金鑰不被接受（例 13） | 拒絕「幣安不接受這組金鑰」；舊的不動 | proxy + verification domain | `exc/` refusal table; `app/`; `ctl/` 422 keyRejected | asserts-oracle | produces-oracle | ✅ conforms |
| AC-17 | 連不上幣安（例 14） | 拒絕「連不上幣安，請稍後再試」；舊的不動 | proxy | `exc/` TestBinanceTradingKeyVerificationProxyReportsUnreachableBinance; `ctl/` 502 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-18 | 幣安遲遲不答（例 15） | 拒絕「等太久，這次沒有存成」；舊的不動 | proxy timeout branches | `exc/` ReportsASlowBinanceAsTimedOut; `ctl/` 504 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-19 | 可交易市場只在存入當下記一次（例 16） | 不重存讀回仍只現貨 | reads never touch the proxy | `app/` TestBinanceTradingKeyApplicationReads (proxy mock has no expectations) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-20 | 已停止的現貨機器人打開（例 17） | 打開成功；讀回開著 | `service/strategy_bot_service.go` EnableAutoOrder | `app/strategy_bot_auto_order_application_test.go` stopped case; `ctl/` 200 autoOrderEnabled:true | asserts-oracle | produces-oracle | ✅ conforms |
| AC-21 | 執行中的機器人也改得動（例 18） | 成功；仍執行中 | same | `app/` running case asserts RunState | asserts-oracle | produces-oracle | ✅ conforms |
| AC-22 | 沒有幣安交易金鑰不能打開（例 19） | 拒絕「請先完成幣安交易金鑰設定」；仍關著 | `domains/strategy_bot_auto_order_domain.go` | `app/` "without a trading key…"; `ctl/` 409 reason | asserts-oracle | produces-oracle | ✅ conforms |
| AC-23 | 可交易市場不涵蓋（例 20） | 拒絕「沒有合約交易權限」；仍關著 | `domains/tradable_markets_domain.go` RequireCovering | `app/`; `dom/` TestTradableMarketsDomainRequireCovering | asserts-oracle | produces-oracle | ✅ conforms |
| AC-24 | 關掉永遠可以（例 21） | 沒金鑰也關得掉；讀回關著 | StrategyBotService.DisableAutoOrder | `app/` "switching off needs no key…" | asserts-oracle | produces-oracle | ✅ conforms |
| AC-25 | 已經關著再關一次不算失敗 | 成功；仍關著 | same | same loop (wasOn=false) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-26 | 新建的與既有的一律是關閉（例 22） | 新建與既有皆關著 | entity default false; Save never names the column | `per/` EnableAutoOrderOnlyAgainstTheKeyThatWasChecked (new bot false); `per/` TestSchemaMigratorLeavesEveryExistingBotWithAutoOrderOff | asserts-oracle | produces-oracle | ✅ conforms |
| AC-27 | 改機器人內容不會順手改到自動下單 | 修改成功；仍開著 | StrategyBotRepository.Save / UpdateRunState column lists | `per/` RewritesAndRoundsLeaveTheAutoOrderSwitchAlone | asserts-oracle | produces-oracle | ✅ conforms |
| AC-28 | 開關打開了，機器人仍然只說話（例 23） | 只送 Telegram；不下單 | run application untouched | `app/strategy_bot_run_application_test.go` StillOnlySpeaksWithAutoOrderSwitchedOn | asserts-oracle | produces-oracle | ✅ conforms |
| AC-29 | 改不動別人的機器人（例 24） | 「找不到」；原封不動 | findOwnedBot | `app/` stranger cases (no write expected); `ctl/` 404 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-30 | 移除金鑰時所有開關一律關掉（例 25） | 移除成功；還沒設定；兩台關著 | Repository.DeleteByUser (one transaction) | `per/` DeleteByUserSwitchesEveryOneOfTheirBotsOff | asserts-oracle | produces-oracle | ✅ conforms |
| AC-31 | 沒有設定也可以要求移除（例 26） | 回報沒有設定，不算失敗 | DeleteByUser no-op; controller 204 | `per/` DeleteByUserWithNothingStoredIsNotAFailure; `ctl/` 204 | asserts-oracle | produces-oracle | ✅ conforms |
| AC-32 | 換成只開現貨的新金鑰（例 27） | 成功；甲關、乙開 | service passes uncovered kinds; Replace switches off | `per/` ReplaceSwitchesOffOnlyTheOwnersUncoveredBots; `app/` uncovered kinds | asserts-oracle | produces-oracle | ✅ conforms |
| AC-33 | 換存失敗時什麼都不變（例 28） | 失敗 keyRejected；舊金鑰、甲乙不變 | service returns before Replace | `app/` refusal table (Replace not expected) | asserts-oracle | produces-oracle | ✅ conforms |
| AC-34 | 打開與換金鑰同時發生 | 甲不是開著 | StrategyBotRepository.EnableAutoOrder FOR SHARE + token | `per/` EnableAutoOrderWaitsOutAReplacementInFlight, EnableAutoOrderOnlyAgainstTheKeyThatWasChecked | asserts-oracle | produces-oracle | ✅ conforms |
| AC-35 | 外掛看得到狀態，看不到金鑰內容（例 29） | 自動下單開；已設定、可交易合約；無任何金鑰字元 | `GET …/status` (requiresSignIn); StrategyBotDto.AutoOrderEnabled | `ctl/` status exact JSON; `ent/` status no tail; `srv/` status readable by connector | asserts-oracle | produces-oracle | ✅ conforms |
| AC-36 | 外掛做不了任何寫入（例 30） | 皆被拒（請重新登入）；不變 | routes on requiresWebSignIn | `srv/` TestBinanceTradingKeyWritesAndKeyTailRefuseConnectors | asserts-oracle (401) | produces-oracle | ✅ conforms |
| AC-37 | 外掛讀不到含結尾那一份 | 被拒 | GET key on requiresWebSignIn | same test | asserts-oracle | produces-oracle | ✅ conforms |
| AC-38 | 已開著再開（derived from BR 冪等） | 成功，不重寫 | EnableAutoOrder IsEnabled short-circuit | `app/` "an already-on bot…" | asserts-oracle | produces-oracle | ✅ conforms |
| BR-1 | 檢查順序：空白 → 開鎖 → 問幣安 | blank refused before seal; seal refused before Binance | service SaveTradingKey | `app/` blank & seal cases with strict mocks | asserts-oracle | produces-oracle | ✅ conforms |
| BR-2 | 可交易市場的認定（現貨與槓桿 → 現貨；合約 → 合約） | enableSpotAndMarginTrading→spot, enableFutures→contract | proxy wire | `exc/` ReadsTheTradingPermissions | asserts-oracle | produces-oracle | ✅ conforms |
| BR-3 | 不檢查「不能有什麼權限」 | withdrawals enabled still accepted | wire reads only two fields | `exc/` "both" case includes enableWithdrawals:true | asserts-oracle | produces-oracle | ✅ conforms |
| BR-4 | 不變式在儲存失敗時也成立 | 換存/移除寫開關失敗 → 金鑰不變 | transactions in Replace / DeleteByUser | `per/` KeyChangesRollBackWhenSwitchesCannotBeWritten, LeavesSwitchesAloneWhenTheKeyCannotBeWritten | asserts-oracle | produces-oracle | ✅ conforms |
| BR-5 | 時間戳記超出範圍、要求放慢、看不懂 → 連不上幣安 | unreachable, not keyRejected | proxy | `exc/` -1021 / 429 / unreadable cases | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-1 | 存入至多等一個等待上限（預設 10 秒） | default 10s, configurable | `config/application_config.go`; http.Client Timeout | `internal/config/tests` Binance cases; `exc/` timeout | asserts-oracle | produces-oracle | ✅ conforms |
| NFR-2 | 回覆與錯誤裡沒有 Secret Key／完整 API Key | none present | DTO types; proxy never returns an error string | `ctl/` NotContains in save & refusal cases; `exc/` query lacks secret | asserts-oracle | produces-oracle | ✅ conforms |

## Orphans (code with no clause)

| Code | Description | Verdict |
|------|-------------|---------|
| `controller/strategy_bot_controller.go` reason `binanceTradingKeyChanged` | 409 when the key changed while switching on | explained by AC-34 (concurrency); no orphan |
| `GET /users/me/binance-trading-key/status` | connector-readable summary | explained by PRD §4 decision and AC-35; no orphan |

No code implements an Out-of-Scope item (no order placement, no permission re-check, no extra notification).

## Summary

- Conforms: 45/45 clauses ✅ (100%)
- Violations: none
- Mis-asserted: none
- Partial: none (AC-26 "existing bots off" was partial before a migration test was added in this run)
- Gaps: none
- Unclear: none
- Orphans: 0

Ceiling: static conformance audit against the PRD acceptance criteria; mapped tests were run as corroboration only.
