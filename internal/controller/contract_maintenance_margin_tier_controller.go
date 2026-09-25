package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

type ContractMaintenanceMarginTierController struct {
	tierApplication *application.ContractMaintenanceMarginTierApplication
}

func NewContractMaintenanceMarginTierController(
	tierApplication *application.ContractMaintenanceMarginTierApplication,
) *ContractMaintenanceMarginTierController {
	return &ContractMaintenanceMarginTierController{tierApplication: tierApplication}
}

// GetTiers handles GET /contract-maintenance-margin-tiers.
func (tierController *ContractMaintenanceMarginTierController) GetTiers(ginContext *gin.Context) {
	tierDtos, findError := tierController.tierApplication.GetTiers(
		ginContext.Request.Context(), ginContext.Query("symbol"))
	if errors.Is(findError, domains.ErrContractMaintenanceMarginTierValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": findError.Error()})

		return
	}
	if findError != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": findError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, tierDtos)
}
