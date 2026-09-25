package controller

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// Matched with a case-insensitive prefix because HTTP compares the scheme name case-insensitively.
const bearerScheme = "bearer "

type UserController struct {
	userApplication *application.UserApplication
}

func NewUserController(userApplication *application.UserApplication) *UserController {
	return &UserController{userApplication: userApplication}
}

// RegisterUser handles POST /users.
func (userController *UserController) RegisterUser(ginContext *gin.Context) {
	var userRegistrationRequest models.UserRegistrationRequest

	if bindError := ginContext.ShouldBindJSON(&userRegistrationRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	userDto, err := userController.userApplication.RegisterUser(
		ginContext.Request.Context(), userRegistrationRequest.ToRegistrationDto())
	if err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, userDto)
}

// SignIn handles POST /sessions.
// SignIn stays 200 rather than 201 because a session has no address to look it up at; it is reachable only via its renewal proof.
func (userController *UserController) SignIn(ginContext *gin.Context) {
	var signInRequest models.SignInRequest

	if bindError := ginContext.ShouldBindJSON(&signInRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	sessionTokensDto, err := userController.userApplication.SignIn(
		ginContext.Request.Context(), signInRequest.ToSignInDto())
	if err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, sessionTokensDto)
}

// RenewSession handles POST /sessions/renewal.
// POST because the renewal proof must travel in a body, and clients and intermediaries drop DELETE bodies.
func (userController *UserController) RenewSession(ginContext *gin.Context) {
	var sessionRenewalRequest models.SessionRenewalRequest

	if bindError := ginContext.ShouldBindJSON(&sessionRenewalRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	sessionTokensDto, err := userController.userApplication.RenewSession(
		ginContext.Request.Context(), sessionRenewalRequest.ToRenewalDto())
	if err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, sessionTokensDto)
}

// RevokeSession handles POST /sessions/revocation.
// Answers 204 whether or not anything was ended, since the requested state holds either way; only a lookup failure errors.
func (userController *UserController) RevokeSession(ginContext *gin.Context) {
	var sessionRenewalRequest models.SessionRenewalRequest

	if bindError := ginContext.ShouldBindJSON(&sessionRenewalRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	if err := userController.userApplication.RevokeSession(
		ginContext.Request.Context(), sessionRenewalRequest.ToRenewalDto()); err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// GetCurrentUser handles GET /users/me.
func (userController *UserController) GetCurrentUser(ginContext *gin.Context) {
	userDto, err := userController.userApplication.IdentifyUser(
		ginContext.Request.Context(), userController.readAccessToken(ginContext))
	if err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, userDto)
}

// ChangePassword handles POST /users/me/password.
// Answers 204 because nothing is returned, and the account comes from the token rather than the body.
func (userController *UserController) ChangePassword(ginContext *gin.Context) {
	var passwordChangeRequest models.PasswordChangeRequest

	if bindError := ginContext.ShouldBindJSON(&passwordChangeRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	if err := userController.userApplication.ChangePassword(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		passwordChangeRequest.ToPasswordChangeDto(),
	); err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readAccessToken returns an empty string when the header is missing or not a bearer token, leaving rejection to the domain.
func (userController *UserController) readAccessToken(ginContext *gin.Context) string {
	authorization := ginContext.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(authorization), bearerScheme) {
		return ""
	}

	return strings.TrimSpace(authorization[len(bearerScheme):])
}

// respondWithError maps only this feature's own errors.
func (userController *UserController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrUserValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrEmailAlreadyRegistered) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	// Both are 401 but kept as two errors because the remedies differ: retype the password or sign in again.
	if errors.Is(err, domains.ErrCredentialsRejected) ||
		errors.Is(err, domains.ErrAuthenticationRequired) {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}
	// 429, not 401 (which sends callers to sign in again) nor 423; the unlock time is also returned as its own field so callers need not parse the message.
	var signInLocked domains.SignInLockedError
	if errors.As(err, &signInLocked) {
		ginContext.JSON(http.StatusTooManyRequests, gin.H{
			"message":     signInLocked.Error(),
			"lockedUntil": signInLocked.LockedUntil.UTC().Format(time.RFC3339),
		})
		return
	}
	// 403, not 401: callers treat 401 as "sign in again", but here only the current password field needs fixing.
	if errors.Is(err, domains.ErrCurrentPasswordRejected) {
		ginContext.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
		return
	}
	// A missing signing key is a system failure; the caller's password was right.
	if errors.Is(err, domains.ErrAccessTokenUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}
