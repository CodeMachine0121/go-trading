package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// bearerScheme is how a proof of identity announces itself in the Authorization
// header, lower-cased because the scheme is compared without regard to case.
const bearerScheme = "bearer "

// currentUserKey is where the identified user is left for the handlers behind this
// door. It is unexported and read only through CurrentUserID, so no handler can
// invent its own spelling of the key and quietly read nothing.
const currentUserKey = "currentUserID"

// AuthenticationMiddleware turns a proof of identity into a user on the request,
// and turns everything else away.
//
// It exists because ownership arrived. Until strategy scripts belonged to people, one
// endpoint read the header for itself and that was enough; now every path that
// touches a strategy script has to know who is asking, and asking each of them to read the
// header would be the same six lines written six times — six places for one of them
// to be forgotten, which is the only kind of mistake that matters here.
//
// Missing, malformed, expired and pointing at somebody who is gone are one answer,
// because they are one thing to the holder: sign in again.
//
// Being recognised but not yet let in is the one answer that is not that, and it is
// kept apart deliberately. To the holder those two call for opposite actions — sign
// in again, or go and ask to be let in — and told the first when the second is true,
// they would sign in successfully and land right back here.
type AuthenticationMiddleware struct {
	userApplication *application.UserApplication
}

func NewAuthenticationMiddleware(userApplication *application.UserApplication) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{userApplication: userApplication}
}

// Handle is the door. A request that gets through carries an identified user who has
// been let in; one that does not gets 401 or 403 and no handler runs.
//
// The door asks one question and reads the answer. It does not identify somebody and
// then consult a flag: what counts as being let in belongs to the domain, and asking
// it here would be this feature's rule written down a second time, in the layer least
// able to test it.
func (authenticationMiddleware *AuthenticationMiddleware) Handle(ginContext *gin.Context) {
	userDto, identifyError := authenticationMiddleware.userApplication.IdentifyActivatedUser(
		ginContext.Request.Context(), accessTokenOf(ginContext))

	// 403 rather than 401, for the same reason a wrong current password is 403 here:
	// in this system 401 means one thing only — "this sign-in no longer counts, go
	// and sign in again" — and callers act on it by sending the person back to the
	// sign-in screen. This person's sign-in is fine. Sending them there would have
	// them do the one thing that cannot change their situation.
	//
	// The instruction is carried by the error rather than assembled here, so that
	// where letters go stays a fact the domain holds and this layer never learns.
	var notActivated domains.AccountNotActivatedError
	if errors.As(identifyError, &notActivated) {
		ginContext.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"message":               notActivated.Error(),
			"activationInstruction": notActivated.Instruction,
		})
		return
	}

	if identifyError != nil {
		ginContext.AbortWithStatusJSON(http.StatusUnauthorized,
			gin.H{"message": domains.ErrAuthenticationRequired.Error()})
		return
	}

	// Only the identifier is left for the handlers, not the standing. Everybody who
	// reaches a handler has been let in, so there is no second case for a handler to
	// tell apart — and a value nobody can act on is a value somebody will one day
	// act on wrongly.
	ginContext.Set(currentUserKey, userDto.ID)
	ginContext.Next()
}

// CurrentUserID is who this request belongs to, for a handler sitting behind the
// door. Reaching a handler without one is impossible — the door aborts first — so
// this answers a single value rather than a value and a doubt, and a handler is
// spared writing an "if not" branch that can never run.
func CurrentUserID(ginContext *gin.Context) uint {
	// Both "nothing is there" and "something else is there" come out as zero, and
	// neither is written as a branch of its own: only this file's own door ever
	// writes that key, so neither can happen, and a branch that cannot run is a
	// branch nothing can prove right. Zero is nobody either way, and the models
	// downstream already refuse nobody.
	identifier, _ := ginContext.Value(currentUserKey).(uint)

	return identifier
}

// accessTokenOf pulls the proof out of the Authorization header, answering with
// nothing when the header is missing or carries some other scheme.
//
// It does not turn those away itself. "No proof was presented" and "the proof
// presented is not valid" are the same refusal to the person holding neither, and
// the rule that says so already lives one layer in.
func accessTokenOf(ginContext *gin.Context) string {
	authorization := ginContext.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(authorization), bearerScheme) {
		return ""
	}

	return strings.TrimSpace(authorization[len(bearerScheme):])
}
