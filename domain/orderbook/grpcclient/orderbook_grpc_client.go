package orderbookgrpcclientdomain

import (
	"context"

	cosmwasmdomain "github.com/osmosis-labs/sqs/domain/cosmwasm"
	orderbookdomain "github.com/osmosis-labs/sqs/domain/orderbook"
	orderbookplugindomain "github.com/osmosis-labs/sqs/domain/orderbook/plugin"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
)

// OrderBookClient is an interface for fetching orders by tick from the orderbook contract.
type OrderBookClient interface {
	// GetOrdersByTick fetches orders by tick from the orderbook contract.
	GetOrdersByTick(ctx context.Context, contractAddress string, tick int64) ([]orderbookplugindomain.Order, error)

	// GetActiveOrders fetches active orders by owner from the orderbook contract.
	GetActiveOrders(ctx context.Context, contractAddress string, ownerAddress string) (orderbookdomain.Orders, uint64, error)

	// GetTickUnrealizedCancels fetches unrealized cancels by tick from the orderbook contract.
	GetTickUnrealizedCancels(ctx context.Context, contractAddress string, tickIDs []int64) ([]UnrealizedTickCancels, error)

	// QueryTicks fetches ticks by tickIDs from the orderbook contract.
	QueryTicks(ctx context.Context, contractAddress string, ticks []int64) ([]orderbookdomain.Tick, error)
}

// orderbookClientImpl is an implementation of OrderbookCWAPIClient.
type orderbookClientImpl struct {
	wasmClient wasmtypes.QueryClient
}

var _ OrderBookClient = (*orderbookClientImpl)(nil)

// New creates a new orderbookClientImpl.
func New(wasmClient wasmtypes.QueryClient) *orderbookClientImpl {
	return &orderbookClientImpl{
		wasmClient: wasmClient,
	}
}

// GetOrdersByTick implements OrderbookCWAPIClient.
func (o *orderbookClientImpl) GetOrdersByTick(ctx context.Context, contractAddress string, tick int64) ([]orderbookplugindomain.Order, error) {
	ordersByTick := ordersByTick{Tick: tick}

	var orders ordersByTickResponse
	if err := cosmwasmdomain.QueryCosmwasmContract(ctx, o.wasmClient, contractAddress, ordersByTickRequest{OrdersByTick: ordersByTick}, &orders); err != nil {
		return nil, err
	}

	return orders.Orders, nil
}

// GetActiveOrders implements OrderbookCWAPIClient.
func (o *orderbookClientImpl) GetActiveOrders(ctx context.Context, contractAddress string, ownerAddress string) (orderbookdomain.Orders, uint64, error) {
	var orders activeOrdersResponse
	if err := cosmwasmdomain.QueryCosmwasmContract(ctx, o.wasmClient, contractAddress, activeOrdersRequest{OrdersByOwner: ordersByOwner{Owner: ownerAddress}}, &orders); err != nil {
		return nil, 0, err
	}

	return orders.Orders, orders.Count, nil
}

// GetTickUnrealizedCancels implements OrderbookCWAPIClient.
func (o *orderbookClientImpl) GetTickUnrealizedCancels(ctx context.Context, contractAddress string, tickIDs []int64) ([]UnrealizedTickCancels, error) {
	var unrealizedCancels unrealizedCancelsResponse
	if err := cosmwasmdomain.QueryCosmwasmContract(ctx, o.wasmClient, contractAddress, unrealizedCancelsByTickIdRequest{UnrealizedCancels: unrealizedCancelsRequestPayload{TickIds: tickIDs}}, &unrealizedCancels); err != nil {
		return nil, err
	}
	return unrealizedCancels.Ticks, nil
}

// QueryTicks implements OrderBookClient.
func (o *orderbookClientImpl) QueryTicks(ctx context.Context, contractAddress string, ticks []int64) ([]orderbookdomain.Tick, error) {
	var orderbookTicks queryTicksResponse
	if err := cosmwasmdomain.QueryCosmwasmContract(ctx, o.wasmClient, contractAddress, queryTicksRequest{TicksByID: ticksByID{TickIDs: ticks}}, &orderbookTicks); err != nil {
		return nil, err
	}
	return orderbookTicks.Ticks, nil
}
