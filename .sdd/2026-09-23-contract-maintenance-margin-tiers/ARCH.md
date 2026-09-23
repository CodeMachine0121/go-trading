# 合約維持保證金分級 — Architecture Design

**Status:** Confirmed
**Source PRD:** `.sdd/2026-09-23-contract-maintenance-margin-tiers/PRD.md`
**Tech context:** Go · Gin · GORM (PostgreSQL) · Clean / Onion

---

## 1. Design Goal & Guiding Principle

- **In one sentence:** 有帳戶金鑰時，把每個已登錄合約標的的整組維持保證金分級抓回來、整組替換地存下來、查得到；沒金鑰時零影響。
- **Guiding principle:** **「沒有金鑰」是 proxy 的事實，不是呼叫端的分支。** proxy 在沒有金鑰時直接回一個具名的哨兵錯誤，
  不送出任何請求；service 與 application 不必知道金鑰存不存在，只需要辨認這個錯誤並安靜地略過。
  背景 job 則在組裝根判斷：沒有金鑰就不註冊，免得每天寫一行「沒有金鑰」。

## 2. Change Scope

| Area | Action | What / Why |
| :--- | :--- | :--- |
| 維持保證金分級整條路 | **Add** | entity、VO、domain（一級＋一整組）、proxy 介面＋實作（簽名請求）、repository、service、application、controller、job |
| `ContractTradingSymbolApplication.AddToWatchlist` | **Modify** | 加入後多刷新一次分級；沒有金鑰時安靜略過 |
| `ApplicationConfig.ContractIngestion` | **Modify** | 金鑰、密鑰、分級位址、刷新間隔 |
| `dependencies.go` / `SchemaMigrator` | **Modify** | 組裝、路由、job（只在有金鑰時註冊）、登錄 entity |
| 交易規格 | **Not touched** | 最小那一級的維持保證金率照舊留著——它不需要金鑰，是沒金鑰時唯一的依據 |

## 3. New Classes / Modules

| Name | Kind | Responsibility |
| :--- | :--- | :--- |
| `entities.ContractMaintenanceMarginTier` | Entity | 一級：symbol、tier、名目下限、名目上限、維持保證金率、速算額、最高槓桿、確認時間；`(symbol, tier)` 唯一 |
| `vo.ContractMaintenanceMarginTierVo` / `vo.ContractMaintenanceMarginLadderVo` | VO | 來源回報的一級／一個標的的一整組 |
| `domains.ContractMaintenanceMarginLadderDomain` | Domain Model | 一整組的規則：至少一級、每一級自己說得通、依級數排好、級與級之間不重疊；`ToEntities(confirmedAt)` |
| `IContractMaintenanceMarginTierProxy` | Interface | `FetchMaintenanceMarginLadders(ctx)`：一次回所有合約的整組分級 |
| `BinanceContractMaintenanceMarginTierProxy` | Proxy | 以 HMAC-SHA256 簽名打 `/fapi/v1/leverageBracket`；沒有金鑰回 `ErrContractAccountCredentialsMissing`；數字以 `json.Number` 讀，不經過浮點 |
| `IContractMaintenanceMarginTierRepository` / `ContractMaintenanceMarginTierRepository` | Repository | `ReplaceLadders(ctx, map)`：每個標的整組刪掉再寫入，同一個交易；`FindBySymbol` |
| `ContractMaintenanceMarginTierService` | Domain Service | `RefreshLadders`（已登錄的標的）、`FindTiers`；報告存了幾個標的、哪幾個說不通 |
| `ContractMaintenanceMarginTierApplication` / Controller / Job | — | 轉呼叫；`GET /contract-maintenance-margin-tiers?symbol=`；啟動一次、之後每 `interval` |

## 6. Extensibility & Handoff Notes

- **下一個需求**：合約重演算強制平倉價。它讀 `FindBySymbol` 拿整組，依部位名目找到那一級，用「名目 × 維持保證金率 − 速算額」得到維持保證金。
- **需要帳戶身分的下一支端點**（例如帳戶實際手續費率）：重用同一份金鑰設定與簽名方式；簽名邏輯目前只在這個 proxy 裡，第二支出現時再抽出共用。

## 7. Traceability

| PRD | Fulfilled by |
| :--- | :--- |
| US-01 | proxy 的哨兵錯誤 ＋ service 報告 ＋ application 略過 ＋ 組裝根只在有金鑰時註冊 job |
| US-02 | `ContractMaintenanceMarginTierService.RefreshLadders` ＋ repository `ReplaceLadders`（交易內整組替換） |
| US-03 | `ContractMaintenanceMarginLadderDomain` |
| US-04 | service `FindTiers` ＋ controller |

## 8. Risks & Open Decisions

- 金鑰權限：只需要唯讀；文件寫明。
- 來源的時間戳記與伺服器時鐘差太多會被拒（`recvWindow`）；以系統時鐘簽名，差距問題以紀錄呈現。
