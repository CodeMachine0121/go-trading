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
// /users, because what it creates is a session — and it deliberately creates nothing
// that is stored, which is why the answer is 200 and not 201. There is no session to
// go and look at afterwards; the caller is holding the whole of it.
func (userController *UserController) SignIn(ginContext *gin.Context) {
	var signInRequest models.SignInRequest

	if bindError := ginContext.ShouldBindJSON(&signInRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	accessTokenDto, err := userController.userApplication.SignIn(
		ginContext.Request.Context(), signInRequest.ToSignInDto())
	if err != nil {
		userController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, accessTokenDto)
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
