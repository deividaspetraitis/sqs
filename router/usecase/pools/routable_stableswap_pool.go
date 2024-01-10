package pools

import (
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/osmosis-labs/osmosis/osmomath"
	"github.com/osmosis-labs/sqs/sqsdomain"

	"github.com/osmosis-labs/osmosis/v21/x/poolmanager"
	poolmanagertypes "github.com/osmosis-labs/osmosis/v21/x/poolmanager/types"

	"github.com/osmosis-labs/osmosis/v21/x/gamm/pool-models/stableswap"
)

var _ sqsdomain.RoutablePool = &routableStableswapPoolImpl{}

type routableStableswapPoolImpl struct {
	ChainPool     *stableswap.Pool "json:\"pool\""
	TokenOutDenom string           "json:\"token_out_denom\""
	TakerFee      osmomath.Dec     "json:\"taker_fee\""
}

// CalculateTokenOutByTokenIn implements RoutablePool.
func (r *routableStableswapPoolImpl) CalculateTokenOutByTokenIn(tokenIn sdk.Coin) (sdk.Coin, error) {
	tokenOut, err := r.ChainPool.CalcOutAmtGivenIn(sdk.Context{}, sdk.NewCoins(tokenIn), r.TokenOutDenom, r.GetSpreadFactor())
	if err != nil {
		return sdk.Coin{}, err
	}

	return tokenOut, nil
}

// GetTokenOutDenom implements RoutablePool.
func (r *routableStableswapPoolImpl) GetTokenOutDenom() string {
	return r.TokenOutDenom
}

// String implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) String() string {
	return fmt.Sprintf("pool (%d), pool type (%d), pool denoms (%v), token out (%s)", r.ChainPool.Id, poolmanagertypes.Balancer, r.ChainPool.GetPoolDenoms(sdk.Context{}), r.TokenOutDenom)
}

// ChargeTakerFee implements sqsdomain.RoutablePool.
// Charges the taker fee for the given token in and returns the token in after the fee has been charged.
func (r *routableStableswapPoolImpl) ChargeTakerFeeExactIn(tokenIn sdk.Coin) (tokenInAfterFee sdk.Coin) {
	tokenInAfterTakerFee, _ := poolmanager.CalcTakerFeeExactIn(tokenIn, r.TakerFee)
	return tokenInAfterTakerFee
}

// GetTakerFee implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) GetTakerFee() math.LegacyDec {
	return r.TakerFee
}

// SetTokenOutDenom implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) SetTokenOutDenom(tokenOutDenom string) {
	r.TokenOutDenom = tokenOutDenom
}

// GetSpreadFactor implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) GetSpreadFactor() math.LegacyDec {
	return r.ChainPool.GetSpreadFactor(sdk.Context{})
}

// GetId implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) GetId() uint64 {
	return r.ChainPool.Id
}

// GetPoolDenoms implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) GetPoolDenoms() []string {
	return r.ChainPool.GetPoolDenoms(sdk.Context{})
}

// GetType implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) GetType() poolmanagertypes.PoolType {
	return poolmanagertypes.Balancer
}

// CalcSpotPrice implements sqsdomain.RoutablePool.
func (r *routableStableswapPoolImpl) CalcSpotPrice(baseDenom string, quoteDenom string) (osmomath.BigDec, error) {
	spotPrice, err := r.ChainPool.SpotPrice(sdk.Context{}, quoteDenom, baseDenom)
	if err != nil {
		return osmomath.BigDec{}, err
	}
	return spotPrice, nil
}