package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// UserApplication orchestrates the use cases about who this system recognises.
type UserApplication struct {
	userService *service.UserService
}

func NewUserApplication(userService *service.UserService) *UserApplication {
	return &UserApplication{userService: userService}
}

func (userApplication *UserApplication) RegisterUser(
	executionContext context.Context, registrationDto dto.UserRegistrationDto,
) (dto.UserDto, error) {
	return userApplication.userService.RegisterUser(executionContext, registrationDto)
}

func (userApplication *UserApplication) SignIn(
	executionContext context.Context, signInDto dto.SignInDto,
) (dto.AccessTokenDto, error) {
	return userApplication.userService.SignIn(executionContext, signInDto)
}

func (userApplication *UserApplication) IdentifyUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	return userApplication.userService.IdentifyUser(executionContext, accessToken)
}
