package assistantqueries

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// assistantMomentOf parses an RFC3339 moment the assistant named, refusing it as an argument
// error so the assistant can correct it.
func assistantMomentOf(namedMoment string, argumentName string) (time.Time, error) {
	moment, parseError := time.Parse(time.RFC3339, namedMoment)
	if parseError != nil {
		return time.Time{}, fmt.Errorf(
			"%w: %s 必須是 RFC3339 時間（例如 2026-09-04T00:00:00Z），收到的是「%s」",
			domains.ErrAssistantQueryArgument, argumentName, namedMoment)
	}

	return moment.UTC(), nil
}

// mostRecentCandles keeps the newest candles up to the ceiling, because the assistant is asked
// about where the market is now, and reports whether any were dropped.
func mostRecentCandles(
	kCandleDtos []dto.KCandleDto, candleLimit domains.AssistantCandleLimitDomain,
) ([]dto.KCandleDto, bool) {
	if len(kCandleDtos) <= candleLimit.Count() {
		return kCandleDtos, false
	}

	return kCandleDtos[len(kCandleDtos)-candleLimit.Count():], true
}

// assistantCandleNoteFor tells the assistant when a stretch was empty or truncated, so it never
// reads a slice as the whole.
func assistantCandleNoteFor(readCount int, shownCount int, truncated bool) string {
	if readCount == 0 {
		return "這段時間內沒有任何 K 線資料。"
	}

	if truncated {
		return fmt.Sprintf(
			"注意：這段時間共有 %d 根，已截斷，只給你最新的 %d 根。"+
				"下結論時不要把它當成整段的全貌；需要更早的部分請改用更長的彙總刻度或縮小時間區間。",
			readCount, shownCount)
	}

	return ""
}
