package http

import (
	"net/http"

	deliveryhttp "github.com/osmosis-labs/sqs/delivery/http"
	"github.com/osmosis-labs/sqs/domain"
	"github.com/osmosis-labs/sqs/domain/mvc"
	_ "github.com/osmosis-labs/sqs/domain/passthrough"
	"github.com/osmosis-labs/sqs/orderbook/types"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

// PassthroughHandler is the http handler for passthrough use case
type PassthroughHandler struct {
	PUsecase mvc.PassthroughUsecase
	OUsecase mvc.OrderBookUsecase
}

const resourcePrefix = "/passthrough"

func formatPassthroughResource(resource string) string {
	return resourcePrefix + resource
}

// NewPassthroughHandler will initialize the pools/ resources endpoint
func NewPassthroughHandler(e *echo.Echo, ptu mvc.PassthroughUsecase, ou mvc.OrderBookUsecase) {
	handler := &PassthroughHandler{
		PUsecase: ptu,
		OUsecase: ou,
	}

	e.GET(formatPassthroughResource("/portfolio-assets/:address"), handler.GetPortfolioAssetsByAddress)
	e.GET(formatPassthroughResource("/active-orders"), handler.GetActiveOrders)
}

// @Summary Returns portfolio assets associated with the given address by category.
// @Description The returned data represents the potfolio asset breakdown by category for the specified address.
// The categories include user balances, unstaking, staked, in-locks, pooled, unclaimed rewards, and total.
// The user balances and total assets are brokend down by-coin with the capitalization of the entire account value.
//
// @Produce  json
// @Success 200  {object}  passthroughdomain.PortfolioAssetsResult  "Portfolio assets by-category and capitalization of the entire account value"
// @Failure 500  {object}  domain.ResponseError  "Response error"
// @Param address path string true "Wallet Address"
// @Router /passthrough/portfolio-assets/{address} [get]
func (a *PassthroughHandler) GetPortfolioAssetsByAddress(c echo.Context) error {
	address := c.Param("address")

	if address == "" {
		return c.JSON(http.StatusInternalServerError, domain.ResponseError{Message: "invalid address: cannot be empty"})
	}

	portfolioAssetsResult, err := a.PUsecase.GetPortfolioAssets(c.Request().Context(), address)
	if err != nil {
		return c.JSON(http.StatusPartialContent, domain.ResponseError{Message: err.Error()})
	}

	return c.JSON(http.StatusOK, portfolioAssetsResult)
}

// @Summary Returns all active orderbook orders associated with the given address.
// @Description The returned data represents all active orders for all orderbooks available for the specified address.
//
// The is_best_effort flag indicates whether the error occurred while processing the orders due which not all orders were returned in the response.
//
// @Produce  json
// @Success 200           {object}  types.GetActiveOrdersResponse  "List of active orders for all available orderboooks for the given address"
// @Failure 400           {object}  domain.ResponseError                 "Response error"
// @Failure 500           {object}  domain.ResponseError                 "Response error"
// @Param  userOsmoAddress  query  string  true  "Osmo wallet address"
// @Router /passthrough/active-orders [get]
func (a *PassthroughHandler) GetActiveOrders(c echo.Context) (err error) {
	ctx := c.Request().Context()

	span := trace.SpanFromContext(ctx)
	defer func() {
		if err != nil {
			span.RecordError(err)
			// nolint:errcheck // ignore error
			c.JSON(domain.GetStatusCode(err), domain.ResponseError{Message: err.Error()})
		}

		// Note: we do not end the span here as it is ended in the middleware.
	}()

	var req types.GetActiveOrdersRequest
	if err := deliveryhttp.UnmarshalRequest(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, domain.ResponseError{Message: err.Error()})
	}

	// Validate the request
	if err := req.Validate(); err != nil {
		return c.JSON(http.StatusBadRequest, domain.ResponseError{Message: err.Error()})
	}

	orders, isBestEffort, err := a.OUsecase.GetActiveOrders(ctx, req.UserOsmoAddress)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, domain.ResponseError{Message: types.ErrInternalError.Error()})
	}

	resp := types.NewGetAllOrderResponse(orders, isBestEffort)

	return c.JSON(http.StatusOK, resp)
}
