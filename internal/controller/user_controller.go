package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// bearerScheme is how a proof of identity announces itself in the Authorization
// header. HTTP says the scheme name is compared without regard to case, which is why
// it is matched with a case-insensitive prefix rather than with strings.CutPrefix.
const bearerScheme = "bearer "

// UserController exposes the sign-in use cases over HTTP.
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
//
// Signing in is a POST to a resource of its own rather than to something under
// /users, because what it creates is a session. The answer stays 200 rather than
// becoming 201 now that a session really is stored: 201 promises somewhere to go and
// look at what was created, and there is no such address — a session is only ever
// reachable by holding its renewal proof, which is the point of it.
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
//
// It is a POST rather than a body-carrying DELETE or PUT for a plain reason: which
// session is meant is named by the renewal proof, the proof can only travel in a
// body, and a DELETE with a body is something an assortment of clients and
// intermediaries quietly drop.
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
//
// It answers 204 whether or not there was anything to end, because "this sign-in no
// longer works" is true either way — and a caller told otherwise would retry to
// reach a state it is already in. The only thing that makes this fail is the system
// itself being unable to look.
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

// readAccessToken pulls the proof of identity out of the Authorization header,
// answering with nothing when the header is missing or carries some other scheme.
//
// It does not turn those away itself. "No proof was presented" and "the proof
// presented is not valid" are the same refusal to the person holding neither, and
// the rule that says so already lives one layer in — writing it here as well would
// be a second copy that can disagree with the first.
//
// This is the seam the day the market endpoints need a door of their own: it becomes
// a middleware that puts the identified user on the request, and this handler reads
// it from there instead. It is not one today because one endpoint does not need a
// layer of indirection to be reached.
func (userController *UserController) readAccessToken(ginContext *gin.Context) string {
	authorization := ginContext.GetHeader("Authorization")
	if !strings.HasPrefix(strings.ToLower(authorization), bearerScheme) {
		return ""
	}

	return strings.TrimSpace(authorization[len(bearerScheme):])
}

// respondWithError maps a domain error onto the status code that reports it. It
// knows only this feature's own errors: a caller must not have to recognise a
// strategy's failure to find out their password was wrong.
func (userController *UserController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrUserValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrEmailAlreadyRegistered) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	// Both of these are "you are not getting in", and both are 401. They stay two
	// errors rather than one because they are refusals of different things — a pair
	// that did not match, and a proof that is not valid — and the caller does
	// different things about them: type the password again, or sign in again.
	if errors.Is(err, domains.ErrCredentialsRejected) ||
		errors.Is(err, domains.ErrAuthenticationRequired) {
		ginContext.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}
	// Having no key to sign with is the system being unable to do its job, not the
	// caller having asked wrongly — their password was right and there is nothing
	// they can change. Saying so is what stops somebody debugging their own password
	// for an hour.
	if errors.Is(err, domains.ErrAccessTokenUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}
