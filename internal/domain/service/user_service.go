package service

import (
	"context"
	"errors"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// UserService is the application layer's only entry point for the people this system
// recognises. Its three public use-case methods never call one another.
//
// Registering, signing in and being recognised live in one service rather than in
// two, because they change for one reason: what it takes to count as somebody this
// system knows. Split apart, both halves would hold the same store and the same
// notion of an account, and no change to either would ever touch only one of them —
// which is a single module written down twice.
type UserService struct {
	userRepository      domaininterface.IUserRepository
	passwordProofProxy  domaininterface.IPasswordProofProxy
	accessTokenProxy    domaininterface.IAccessTokenProxy
	clockProxy          domaininterface.IClockProxy
	accessTokenLifetime time.Duration
}

func NewUserService(
	userRepository domaininterface.IUserRepository,
	passwordProofProxy domaininterface.IPasswordProofProxy,
	accessTokenProxy domaininterface.IAccessTokenProxy,
	clockProxy domaininterface.IClockProxy,
	accessTokenLifetime time.Duration,
) *UserService {
	return &UserService{
		userRepository:      userRepository,
		passwordProofProxy:  passwordProofProxy,
		accessTokenProxy:    accessTokenProxy,
		clockProxy:          clockProxy,
		accessTokenLifetime: accessTokenLifetime,
	}
}

// RegisterUser creates a user and hands them back as stored. A registration that
// breaks a rule is refused before the password is turned into anything and before
// anything is written.
func (userService *UserService) RegisterUser(
	executionContext context.Context, registrationDto dto.UserRegistrationDto,
) (dto.UserDto, error) {
	registration, validationError := domains.NewUserRegistrationDomain(registrationDto)
	if validationError != nil {
		return dto.UserDto{}, validationError
	}

	passwordProof, proveError := userService.passwordProofProxy.Prove(registration.Password())
	if proveError != nil {
		return dto.UserDto{}, proveError
	}

	// Whether the address is free is left to the write, not asked beforehand. Asking
	// first would let two registrations arriving at once both find it free.
	savedUser, saveError := userService.userRepository.Save(
		executionContext, registration.ToEntity(passwordProof))
	if saveError != nil {
		return dto.UserDto{}, saveError
	}

	return savedUser.ToDto(), nil
}

// SignIn checks a pair and hands back a proof of identity good for as long as this
// system says a session lasts. It writes nothing: signing in is a question, and the
// answer to it is the token the caller walks away with.
func (userService *UserService) SignIn(
	executionContext context.Context, signInDto dto.SignInDto,
) (dto.AccessTokenDto, error) {
	signIn, credentialsError := domains.NewSignInDomain(signInDto)
	if credentialsError != nil {
		return dto.AccessTokenDto{}, credentialsError
	}

	user, findError := userService.userRepository.FindOneByEmail(executionContext, signIn.Email())
	if errors.Is(findError, domains.ErrUserNotFound) {
		// The password is checked against a decoy rather than skipped, so that
		// "nobody holds this address" takes as long to refuse as "wrong password"
		// does. Returning here directly would answer noticeably sooner, and how long
		// an answer took is information nobody meant to hand over.
		userService.passwordProofProxy.Matches(
			signIn.Password(), userService.passwordProofProxy.DecoyProof())

		return dto.AccessTokenDto{}, domains.ErrCredentialsRejected
	}
	if findError != nil {
		// Storage being broken is not a wrong password. Dressing it up as one would
		// have somebody retyping a password that was right all along.
		return dto.AccessTokenDto{}, findError
	}

	if !userService.passwordProofProxy.Matches(signIn.Password(), user.PasswordProof) {
		return dto.AccessTokenDto{}, domains.ErrCredentialsRejected
	}

	accessToken, issueError := userService.accessTokenProxy.Issue(
		user.ID, userService.clockProxy.Now().Add(userService.accessTokenLifetime))
	if issueError != nil {
		return dto.AccessTokenDto{}, issueError
	}

	return accessToken.ToDto(), nil
}

// IdentifyUser says who a proof of identity belongs to.
//
// The user is read back rather than taken from the proof, and that is what makes a
// token stop working when the account behind it is gone. A proof carries who it was
// issued to, not whether they are still here — those are different questions, and
// only the store can answer the second.
func (userService *UserService) IdentifyUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	if accessToken == "" {
		return dto.UserDto{}, domains.ErrAuthenticationRequired
	}

	userID, identifyError := userService.accessTokenProxy.UserIdentifiedBy(accessToken)
	if identifyError != nil {
		return dto.UserDto{}, identifyError
	}

	user, findError := userService.userRepository.FindOne(executionContext, userID)
	if errors.Is(findError, domains.ErrUserNotFound) {
		return dto.UserDto{}, domains.ErrAuthenticationRequired
	}
	if findError != nil {
		return dto.UserDto{}, findError
	}

	return user.ToDto(), nil
}
