package assistantqueries

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// The three things below are shared by the two capabilities that hand K candles to
// the assistant, and by nothing else. They live together in one file because they
// are one idea — how much market the assistant may be shown, and how it is told what
// it is looking at — and because writing them once is what keeps the two capabilities
// from drifting into showing the assistant different things under the same ceiling.

// assistantMomentOf reads a moment the assistant named. It is refused as an argument
// problem rather than reported as a fault, because the assistant wrote it and can
// write it again.
func assistantMomentOf(namedMoment string, argumentName string) (time.Time, error) {
	moment, parseError := time.Parse(time.RFC3339, namedMoment)
	if parseError != nil {
		return time.Time{}, fmt.Errorf(
			"%w: %s 必須是 RFC3339 時間（例如 2026-09-04T00:00:00Z），收到的是「%s」",
			domains.ErrAssistantQueryArgument, argumentName, namedMoment)
	}

	return moment.UTC(), nil
}

// mostRecentCandles keeps the newest of what was read, up to what the ceiling allows,
// and says whether anything was dropped.
//
// The newest rather than the oldest, because every question the assistant is asked
// about a market is about where it is now. Handing over the first two hundred candles
// of a five-hundred-candle stretch would answer a question nobody asked.
func mostRecentCandles(
	kCandleDtos []dto.KCandleDto, candleLimit domains.AssistantCandleLimitDomain,
) ([]dto.KCandleDto, bool) {
	if len(kCandleDtos) <= candleLimit.Count() {
		return kCandleDtos, false
	}

	return kCandleDtos[len(kCandleDtos)-candleLimit.Count():], true
}

// assistantCandleNoteFor is what the assistant must be told about what it is looking
// at: that a stretch held nothing, or that it is seeing less than the stretch holds.
//
// Being told is the whole point of the ceiling. Silence would leave the assistant
// reading a slice as the whole, which is the one failure a cost ceiling could
// otherwise cause on its own.
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
