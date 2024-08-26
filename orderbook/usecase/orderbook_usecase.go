package orderbookusecase

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/osmosis-labs/osmosis/osmomath"
	cwpoolmodel "github.com/osmosis-labs/osmosis/v25/x/cosmwasmpool/model"
	"github.com/osmosis-labs/sqs/domain/mvc"
	orderbookdomain "github.com/osmosis-labs/sqs/domain/orderbook"
	orderbookgrpcclientdomain "github.com/osmosis-labs/sqs/domain/orderbook/grpcclient"
	"github.com/osmosis-labs/sqs/log"
	"github.com/osmosis-labs/sqs/orderbook/telemetry"
	"github.com/osmosis-labs/sqs/sqsdomain"
	"go.uber.org/zap"

	clmath "github.com/osmosis-labs/osmosis/v25/x/concentrated-liquidity/math"
)

type orderbookUseCaseImpl struct {
	orderbookRepository orderbookdomain.OrderBookRepository
	orderBookClient     orderbookgrpcclientdomain.OrderBookClient
	poolsUsecease       mvc.PoolsUsecase
	tokensUsecease      mvc.TokensUsecase
	logger              log.Logger
}

var _ mvc.OrderBookUsecase = &orderbookUseCaseImpl{}

// New creates a new orderbook use case.
func New(
	orderbookRepository orderbookdomain.OrderBookRepository,
	orderBookClient orderbookgrpcclientdomain.OrderBookClient,
	poolsUsecease mvc.PoolsUsecase,
	tokensUsecease mvc.TokensUsecase,
	logger log.Logger,
) mvc.OrderBookUsecase {
	return &orderbookUseCaseImpl{
		orderbookRepository: orderbookRepository,
		orderBookClient:     orderBookClient,
		poolsUsecease:       poolsUsecease,
		tokensUsecease:      tokensUsecease,
		logger:              logger,
	}
}

// GetTicks implements mvc.OrderBookUsecase.
func (o *orderbookUseCaseImpl) GetAllTicks(poolID uint64) (map[int64]orderbookdomain.OrderbookTick, bool) {
	return o.orderbookRepository.GetAllTicks(poolID)
}

// StoreTicks implements mvc.OrderBookUsecase.
func (o *orderbookUseCaseImpl) ProcessPool(ctx context.Context, pool sqsdomain.PoolI) error {
	cosmWasmPoolModel := pool.GetSQSPoolModel().CosmWasmPoolModel
	if cosmWasmPoolModel == nil {
		return fmt.Errorf("cw pool model is nil when processing order book")
	}

	poolID := pool.GetId()
	if !cosmWasmPoolModel.IsOrderbook() {
		return fmt.Errorf("pool is not an orderbook pool %d", poolID)
	}

	// Update the orderbook client with the orderbook pool ID.
	ticks := cosmWasmPoolModel.Data.Orderbook.Ticks
	if len(ticks) == 0 {
		return nil // early return, nothing do
	}

	cwModel, ok := pool.GetUnderlyingPool().(*cwpoolmodel.CosmWasmPool)
	if !ok {
		return fmt.Errorf("failed to cast pool model to CosmWasmPool")
	}

	// Get tick IDs
	tickIDs := make([]int64, 0, len(ticks))
	for _, tick := range ticks {
		tickIDs = append(tickIDs, tick.TickId)
	}

	// Fetch tick states
	tickStates, err := o.fetchTicksForOrderbook(ctx, cwModel.ContractAddress, tickIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch ticks for pool %s: %w", cwModel.ContractAddress, err)
	}

	// Fetch unrealized cancels
	unrealizedCancels, err := o.fetchTickUnrealizedCancels(ctx, cwModel.ContractAddress, tickIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch unrealized cancels for pool %s: %w", cwModel.ContractAddress, err)
	}

	tickDataMap := make(map[int64]orderbookdomain.OrderbookTick, len(ticks))
	for i, tick := range ticks {
		unrealizedCancel := unrealizedCancels[i]

		// Validate the tick IDs match between the tick and the unrealized cancel
		if unrealizedCancel.TickID != tick.TickId {
			return fmt.Errorf("tick id mismatch when fetching unrealized ticks %d %d", unrealizedCancel.TickID, tick.TickId)
		}

		tickState := tickStates[i]
		if tickState.TickID != tick.TickId {
			return fmt.Errorf("tick id mismatch when fetching tick states %d %d", tickState.TickID, tick.TickId)
		}

		// Update tick map for the pool
		tickDataMap[tick.TickId] = orderbookdomain.OrderbookTick{
			Tick:              &ticks[i],
			TickState:         tickState.TickState,
			UnrealizedCancels: unrealizedCancel.UnrealizedCancelsState,
		}
	}

	// Store the ticks
	o.orderbookRepository.StoreTicks(poolID, tickDataMap)

	return nil
}

// GetActiveOrders implements mvc.OrderBookUsecase.
func (o *orderbookUseCaseImpl) GetActiveOrders(ctx context.Context, address string) ([]orderbookdomain.LimitOrder, error) {
	orderbooks, err := o.poolsUsecease.GetAllCanonicalOrderbookPoolIDs()
	if err != nil {
		return nil, fmt.Errorf("failed to get all canonical orderbook pool IDs: %w", err)
	}

	var results []orderbookdomain.LimitOrder
	for _, orderbook := range orderbooks {
		orders, count, err := o.orderBookClient.GetActiveOrders(context.TODO(), orderbook.ContractAddress, address)
		if err != nil {
			telemetry.GetActiveOrdersErrorCounter.Inc()
			o.logger.Error(telemetry.GetActiveOrdersErrorMetricName, zap.Any("contract", orderbook.ContractAddress), zap.Any("contract", address), zap.Any("err", err))
			continue
		}

		// There are orders to process for given orderbook
		if count == 0 {
			continue
		}

		o.logger.Info("Active orders", zap.Any("orders", orders), zap.Any("count", count), zap.Any("err", err))

		quoteToken, err := o.tokensUsecease.GetMetadataByChainDenom(orderbook.Quote)
		if err != nil {
			o.logger.Error("failed to get token metadata for quote", zap.Any("quote", orderbook.Quote), zap.Error(err))
			continue
		}

		baseToken, err := o.tokensUsecease.GetMetadataByChainDenom(orderbook.Base)
		if err != nil {
			o.logger.Error("failed to get token metadata for base", zap.Any("base", orderbook.Base), zap.Error(err))
			continue
		}

		for _, order := range orders {
			repositoryTick, ok := o.orderbookRepository.GetTickByID(orderbook.PoolID, order.TickId)
			if !ok {
				telemetry.GetTickByIDNotFoundCounter.Inc()
				o.logger.Info(telemetry.GetTickByIDNotFoundMetricName, zap.Any("contract", orderbook.ContractAddress), zap.Any("ticks", order.TickId), zap.Any("ok", ok))
			}

			result, err := o.createLimitOrder(
				order,
				repositoryTick.TickState,
				repositoryTick.UnrealizedCancels,
				orderbookdomain.Asset{
					Symbol:   quoteToken.CoinMinimalDenom,
					Decimals: quoteToken.Precision,
				},
				orderbookdomain.Asset{
					Symbol:   baseToken.CoinMinimalDenom,
					Decimals: baseToken.Precision,
				},
				orderbook.ContractAddress,
			)
			if err != nil {
				telemetry.CreateLimitOrderErrorCounter.Inc()
				o.logger.Error(telemetry.CreateLimitOrderErrorMetricName, zap.Any("order", order), zap.Any("err", err))
				continue
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// TransformOrder transforms an order into a mapped limit order.
func (o *orderbookUseCaseImpl) createLimitOrder(
	order orderbookdomain.Order,
	tickState orderbookdomain.TickState,
	unrealizedCancels orderbookdomain.UnrealizedCancels,
	quoteAsset orderbookdomain.Asset,
	baseAsset orderbookdomain.Asset,
	orderbookAddress string,
) (orderbookdomain.LimitOrder, error) {
	// Parse quantity as int64
	quantity, err := strconv.ParseInt(order.Quantity, 10, 64)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing quantity: %w", err)
	}

	// Convert quantity to decimal for the calculations
	quantityDec := osmomath.NewDec(quantity)

	placedQuantity, err := strconv.ParseInt(order.PlacedQuantity, 10, 64)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing placed quantity: %w", err)
	}

	placedQuantityDec, err := osmomath.NewDecFromStr(order.PlacedQuantity)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing placed quantity: %w", err)
	}

	// Calculate percent claimed
	percentClaimed := placedQuantityDec.Sub(quantityDec).Quo(placedQuantityDec)

	// Calculate normalization factor for price
	normalizationFactor, err := o.tokensUsecease.GetSpotPriceScalingFactorByDenom(baseAsset.Symbol, quoteAsset.Symbol)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error getting spot price scaling factor: %w", err)
	}

	// Determine tick values and unrealized cancels based on order direction
	var tickEtas, tickUnrealizedCancelled int64
	if order.OrderDirection == "bid" {
		tickEtas, err = strconv.ParseInt(tickState.BidValues.EffectiveTotalAmountSwapped, 10, 64)
		if err != nil {
			return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing bid effective total amount swapped: %w", err)
		}

		tickUnrealizedCancelled, err = strconv.ParseInt(unrealizedCancels.BidUnrealizedCancels.String(), 10, 64)
		if err != nil {
			return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing bid unrealized cancels: %w", err)
		}
	} else {
		tickEtas, err = strconv.ParseInt(tickState.AskValues.EffectiveTotalAmountSwapped, 10, 64)
		if err != nil {
			return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing ask effective total amount swapped: %w", err)
		}

		tickUnrealizedCancelled, err = strconv.ParseInt(unrealizedCancels.AskUnrealizedCancels.String(), 10, 64)
		if err != nil {
			return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing ask unrealized cancels: %w", err)
		}
	}

	// Calculate total ETAs and total filled

	etas, err := strconv.ParseInt(order.Etas, 10, 64)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing etas: %w", err)
	}

	tickTotalEtas := tickEtas + tickUnrealizedCancelled

	totalFilled := int64(math.Max(
		float64(tickTotalEtas-(etas-(placedQuantity-quantity))),
		0,
	))

	// Calculate percent filled using
	percentFilled, err := osmomath.NewDecFromStr(strconv.FormatFloat(math.Min(float64(totalFilled)/float64(placedQuantity), 1), 'f', -1, 64))
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error calculating percent filled: %w", err)
	}

	// Determine order status based on percent filled
	status, err := order.Status(percentFilled.MustFloat64())
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("mapping order status: %w", err)
	}

	// Calculate price based on tick ID
	price, err := clmath.TickToPrice(order.TickId)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("converting tick to price: %w", err)
	}

	// Calculate output based on order direction
	var output osmomath.Dec
	if order.OrderDirection == "bid" {
		output = placedQuantityDec.Quo(price.Dec())
	} else {
		output = placedQuantityDec.Mul(price.Dec())
	}

	// Calculate normalized price
	normalizedPrice := price.Dec().Mul(normalizationFactor)

	// Convert placed_at to a nano second timestamp
	placedAt, err := strconv.ParseInt(order.PlacedAt, 10, 64)
	if err != nil {
		return orderbookdomain.LimitOrder{}, fmt.Errorf("error parsing placed_at: %w", err)
	}
	placedAt = time.Unix(0, placedAt).Unix()

	// Return the mapped limit order
	return orderbookdomain.LimitOrder{
		TickId:           order.TickId,
		OrderId:          order.OrderId,
		OrderDirection:   order.OrderDirection,
		Owner:            order.Owner,
		Quantity:         quantity,
		Etas:             order.Etas,
		ClaimBounty:      order.ClaimBounty,
		PlacedQuantity:   placedQuantity,
		PercentClaimed:   percentClaimed.String(),
		TotalFilled:      totalFilled,
		PercentFilled:    percentFilled.String(),
		OrderbookAddress: orderbookAddress,
		Price:            normalizedPrice.String(),
		Status:           status,
		Output:           output.String(),
		QuoteAsset:       quoteAsset,
		BaseAsset:        baseAsset,
		PlacedAt:         placedAt,
	}, nil
}

const maxQueryTicks = 500

func (o *orderbookUseCaseImpl) fetchTicksForOrderbook(ctx context.Context, contractAddress string, tickIDs []int64) ([]orderbookdomain.Tick, error) {
	finalTickStates := make([]orderbookdomain.Tick, 0, len(tickIDs))

	for i := 0; i < len(tickIDs); i += maxQueryTicks {
		end := i + maxQueryTicks
		if end > len(tickIDs) {
			end = len(tickIDs)
		}

		currentTickIDs := tickIDs[i:end]

		tickStates, err := o.orderBookClient.QueryTicks(ctx, contractAddress, currentTickIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch ticks for pool %s: %w", contractAddress, err)
		}

		finalTickStates = append(finalTickStates, tickStates...)
	}

	if len(finalTickStates) != len(tickIDs) {
		return nil, fmt.Errorf("mismatch in number of ticks fetched: expected %d, got %d", len(tickIDs), len(finalTickStates))
	}

	return finalTickStates, nil
}

const maxQueryTicksCancels = 100

func (o *orderbookUseCaseImpl) fetchTickUnrealizedCancels(ctx context.Context, contractAddress string, tickIDs []int64) ([]orderbookgrpcclientdomain.UnrealizedTickCancels, error) {
	allUnrealizedCancels := make([]orderbookgrpcclientdomain.UnrealizedTickCancels, 0, len(tickIDs))

	for i := 0; i < len(tickIDs); i += maxQueryTicksCancels {
		end := i + maxQueryTicksCancels
		if end > len(tickIDs) {
			end = len(tickIDs)
		}

		currentTickIDs := tickIDs[i:end]

		unrealizedCancels, err := o.orderBookClient.GetTickUnrealizedCancels(ctx, contractAddress, currentTickIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch unrealized cancels for ticks %v: %w", currentTickIDs, err)
		}

		allUnrealizedCancels = append(allUnrealizedCancels, unrealizedCancels...)
	}

	if len(allUnrealizedCancels) != len(tickIDs) {
		return nil, fmt.Errorf("mismatch in number of unrealized cancels fetched: expected %d, got %d", len(tickIDs), len(allUnrealizedCancels))
	}

	return allUnrealizedCancels, nil
}