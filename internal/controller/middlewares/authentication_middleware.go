package middlewares

import (
	"errors"
	"net/http"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// Lower-cased because the scheme is compared case-insensitively.
const bearerScheme = "bearer "

// Unexported and read only through CurrentUserID so no handler can misspell the key.
const currentUserKey = "currentUserID"

// AuthenticationMiddleware resolves the bearer token to a user; any invalid token answers 401 (sign in again), while a recognised but not-yet-activated account answers 403.
type AuthenticationMiddleware struct {
	userApplication *application.UserApplication
}

func NewAuthenticationMiddleware(userApplication *application.UserApplication) *AuthenticationMiddleware {
	return &AuthenticationMiddleware{userApplication: userApplication}
}

// Handle lets through only identified, activated users; activation is decided by the domain, not here.
func (authenticationMiddleware *AuthenticationMiddleware) Handle(ginContext *gin.Context) {
	userDto, identifyError := authenticationMiddleware.userApplication.IdentifyActivatedUser(
		ginContext.Request.Context(), accessTokenOf(ginContext))
	authenticationMiddleware.admitOrRefuse(ginContext, userDto, identifyError)
}

// HandleWebSignIn also refuses connector tokens, for the routes only the user in the browser may use.
func (authenticationMiddleware *AuthenticationMiddleware) HandleWebSignIn(ginContext *gin.Context) {
	userDto, identifyError := authenticationMiddleware.userApplication.IdentifyActivatedWebUser(
		ginContext.Request.Context(), accessTokenOf(ginContext))
	authenticationMiddleware.admitOrRefuse(ginContext, userDto, identifyError)
}

func (authenticationMiddleware *AuthenticationMiddleware) admitOrRefuse(
	ginContext *gin.Context, userDto dto.UserDto, identifyError error,
) {
	// 403, not 401: callers treat 401 as "sign in again", which cannot help an account awaiting activation; the instruction comes from the domain error.
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

	// Only the ID is stored, since every user reaching a handler is already activated.
	ginContext.Set(currentUserKey, userDto.ID)
	ginContext.Next()
}

// CurrentUserID returns the requesting user's ID; the middleware guarantees it is set before any handler runs.
func CurrentUserID(ginContext *gin.Context) uint {
	// A missing or mistyped value yields zero, which downstream models already refuse.
	identifier, _ := ginContext.Value(currentUserKey).(uint)

	return identifier
}

// accessTokenOf returns an empty string when the header is missing or not a bearer token, leaving rejection to the domain.
func accessTokenOf(ginContext *gin.Context) string {
	authorization := ginContext.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(authorization), bearerScheme) {
		return ""
	}

	return strings.TrimSpace(authorization[len(bearerScheme):])
}
