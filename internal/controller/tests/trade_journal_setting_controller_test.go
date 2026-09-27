package controller_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type tradeJournalSettingRouterUnderTest struct {
	engine                        *gin.Engine
	tradeJournalSettingRepository *mocks.MockITradeJournalSettingRepository
	tradeTagRepository            *mocks.MockITradeTagRepository
	contractTradeRecordRepository *mocks.MockIContractTradeRecordRepository
}

func newTradeJournalSettingRouterUnderTest(t *testing.T) tradeJournalSettingRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)).AnyTimes()
	fixture := tradeJournalSettingRouterUnderTest{
		tradeJournalSettingRepository: mocks.NewMockITradeJournalSettingRepository(mockController),
		tradeTagRepository:            mocks.NewMockITradeTagRepository(mockController),
		contractTradeRecordRepository: mocks.NewMockIContractTradeRecordRepository(mockController),
	}
	spotTradeRecordRepository := mocks.NewMockISpotTradeRecordRepository(mockController)
	spotTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	settingController := controller.NewTradeJournalSettingController(application.NewTradeJournalSettingApplication(
		service.NewTradeJournalSettingService(
			fixture.tradeJournalSettingRepository, fixture.tradeTagRepository, fixture.contractTradeRecordRepository,
			spotTradeRecordRepository, clockProxy)))

	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine := gin.New()
	engine.GET("/users/me/trade-journal-settings", requiresSignIn, settingController.GetSetting)
	engine.PUT("/users/me/trade-journal-settings", requiresSignIn, settingController.SaveFeeRates)
	engine.GET("/users/me/trade-tags", requiresSignIn, settingController.ListTags)
	engine.POST("/users/me/trade-tags", requiresSignIn, settingController.CreateTag)
	engine.PUT("/users/me/trade-tags/:id", requiresSignIn, settingController.RenameTag)
	engine.DELETE("/users/me/trade-tags/:id", requiresSignIn, settingController.DeleteTag)
	fixture.engine = engine

	return fixture
}

func (fixture tradeJournalSettingRouterUnderTest) send(method string, path string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	response := httptest.NewRecorder()
	fixture.engine.ServeHTTP(response, request)

	return response
}

func TestTradeJournalSettingRouterFeeRates(t *testing.T) {
	fixture := newTradeJournalSettingRouterUnderTest(t)
	fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
		Return(entities.TradeJournalSetting{}, false, nil).Times(3)
	fixture.tradeJournalSettingRepository.EXPECT().SaveFeeRates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, setting entities.TradeJournalSetting) (entities.TradeJournalSetting, error) {
			return setting, nil
		})

	read := fixture.send(http.MethodGet, "/users/me/trade-journal-settings", "")
	saved := fixture.send(http.MethodPut, "/users/me/trade-journal-settings", `{"makerFeeRate":"0.02","takerFeeRate":"0.05"}`)
	negative := fixture.send(http.MethodPut, "/users/me/trade-journal-settings", `{"takerFeeRate":"-0.01"}`)
	notJson := fixture.send(http.MethodPut, "/users/me/trade-journal-settings", `{`)

	require.Equal(t, http.StatusOK, read.Code)
	assert.JSONEq(t, `{"makerFeeRate":null,"takerFeeRate":null}`, read.Body.String())
	require.Equal(t, http.StatusOK, saved.Code)
	assert.JSONEq(t, `{"makerFeeRate":"0.02","takerFeeRate":"0.05"}`, saved.Body.String())
	assert.Equal(t, http.StatusBadRequest, negative.Code)
	assert.Equal(t, http.StatusBadRequest, notJson.Code)
}

func TestTradeJournalSettingRouterFailsAsABadGateway(t *testing.T) {
	fixture := newTradeJournalSettingRouterUnderTest(t)
	fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
		Return(entities.TradeJournalSetting{}, false, context.DeadlineExceeded).Times(3)

	assert.Equal(t, http.StatusBadGateway, fixture.send(http.MethodGet, "/users/me/trade-journal-settings", "").Code)
	assert.Equal(t, http.StatusBadGateway, fixture.send(http.MethodPut, "/users/me/trade-journal-settings", `{}`).Code)
	assert.Equal(t, http.StatusBadGateway, fixture.send(http.MethodGet, "/users/me/trade-tags", "").Code)
}

func TestTradeJournalSettingRouterTags(t *testing.T) {
	t.Run("tags are listed, created, renamed and deleted", func(t *testing.T) {
		fixture := newTradeJournalSettingRouterUnderTest(t)
		seededAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
		ownTag := entities.TradeTag{ID: 5, OwnerID: signedInViewerID, Kind: "setup", Name: "突破"}
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), signedInViewerID).
			Return(entities.TradeJournalSetting{DefaultMistakeTagsSeededAt: &seededAt}, true, nil)
		fixture.tradeTagRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).Return([]entities.TradeTag{ownTag}, nil)
		fixture.tradeTagRepository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(ownTag, nil)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil).Times(2)
		fixture.tradeTagRepository.EXPECT().Rename(gomock.Any(), uint(5), "回踩").
			Return(entities.TradeTag{ID: 5, OwnerID: signedInViewerID, Kind: "setup", Name: "回踩"}, nil)
		fixture.contractTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), uint(5)).Return(int64(0), nil)
		fixture.tradeTagRepository.EXPECT().Delete(gomock.Any(), uint(5)).Return(nil)

		listed := fixture.send(http.MethodGet, "/users/me/trade-tags", "")
		created := fixture.send(http.MethodPost, "/users/me/trade-tags", `{"kind":"setup","name":"突破"}`)
		renamed := fixture.send(http.MethodPut, "/users/me/trade-tags/5", `{"name":"回踩"}`)
		deleted := fixture.send(http.MethodDelete, "/users/me/trade-tags/5", "")

		require.Equal(t, http.StatusOK, listed.Code)
		assert.JSONEq(t, `[{"id":5,"kind":"setup","name":"突破"}]`, listed.Body.String())
		assert.Equal(t, http.StatusCreated, created.Code)
		assert.Equal(t, http.StatusOK, renamed.Code)
		assert.Contains(t, renamed.Body.String(), `"name":"回踩"`)
		assert.Equal(t, http.StatusNoContent, deleted.Code)
	})

	t.Run("a duplicate or a tag in use answers 409, somebody else's 404, bad input 400", func(t *testing.T) {
		fixture := newTradeJournalSettingRouterUnderTest(t)
		ownTag := entities.TradeTag{ID: 5, OwnerID: signedInViewerID, Kind: "mistake", Name: "移動止損"}
		fixture.tradeTagRepository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(entities.TradeTag{}, domains.ErrTradeTagNameConflict)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil)
		fixture.contractTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), uint(5)).Return(int64(4), nil)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(entities.TradeTag{ID: 6, OwnerID: 99}, nil)

		duplicate := fixture.send(http.MethodPost, "/users/me/trade-tags", `{"kind":"setup","name":"突破"}`)
		inUse := fixture.send(http.MethodDelete, "/users/me/trade-tags/5", "")
		strangers := fixture.send(http.MethodPut, "/users/me/trade-tags/6", `{"name":"x"}`)
		blank := fixture.send(http.MethodPost, "/users/me/trade-tags", `{"kind":"setup","name":""}`)

		assert.Equal(t, http.StatusConflict, duplicate.Code)
		assert.Equal(t, http.StatusConflict, inUse.Code)
		assert.Contains(t, inUse.Body.String(), "還有 4 筆交易貼著它")
		assert.Equal(t, http.StatusNotFound, strangers.Code)
		assert.Equal(t, http.StatusBadRequest, blank.Code)
		assert.Equal(t, http.StatusBadRequest, fixture.send(http.MethodPost, "/users/me/trade-tags", `{`).Code)
		assert.Equal(t, http.StatusBadRequest, fixture.send(http.MethodPut, "/users/me/trade-tags/5", `{`).Code)
		assert.Equal(t, http.StatusBadRequest, fixture.send(http.MethodPut, "/users/me/trade-tags/x", `{}`).Code)
		assert.Equal(t, http.StatusBadRequest, fixture.send(http.MethodDelete, "/users/me/trade-tags/0", "").Code)
	})
}
