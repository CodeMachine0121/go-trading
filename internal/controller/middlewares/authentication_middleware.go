package middlewares

import (
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
// It exists because ownership arrived. Until strategies belonged to people, one
// endpoint read the header for itself and that was enough; now every path that
// touches a strategy has to know who is asking, and asking each of them to read the
// header would be the same six lines written six times — six places for one of them
// to be forgotten, which is the only kind of mistake that matters here.
//
// Missing, malformed, expired and pointing at somebody who is gone are one answer,
// because they are one thing to the holder: sign in again.
type AuthenticationMiddleware struct {
	userApplication *application.UserApplication
}

func NewAuthenticationMiddleware(userApplication *application.UserApplication) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{userApplication: userApplication}
}

// Handle is the door. A request that gets through carries an identified user; one
// that does not gets 401 and no handler runs.
func (authenticationMiddleware *AuthenticationMiddleware) Handle(ginContext *gin.Context) {
	userDto, identifyError := authenticationMiddleware.userApplication.IdentifyUser(
		ginContext.Request.Context(), accessTokenOf(ginContext))
	if identifyError != nil {
		ginContext.AbortWithStatusJSON(http.StatusUnauthorized,
			gin.H{"message": domains.ErrAuthenticationRequired.Error()})
		return
	}

	ginContext.Set(currentUserKey, userDto.ID)
	ginContext.Next()
}

// CurrentUserID is who this request belongs to, for a handler sitting behind the
// door. Reaching a handler without one is impossible — the door aborts first — so
// this answers a single value rather than a value and a doubt, and a handler is
// spared writing an "if not" branch that can never run.
func CurrentUserID(ginContext *gin.Context) uint {
	userID, isPresent := ginContext.Get(currentUserKey)
	if !isPresent {
		return 0
	}

	identifier, isIdentifier := userID.(uint)
	if !isIdentifier {
		return 0
	}

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
