package pools_test

import (
	"context"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/osmosis-labs/sqs/domain"
	cosmwasmdomain "github.com/osmosis-labs/sqs/domain/cosmwasm"
	ingesttypes "github.com/osmosis-labs/sqs/ingest/types"
	"github.com/osmosis-labs/sqs/router/usecase/pools"

	"github.com/osmosis-labs/osmosis/osmomath"
	"github.com/osmosis-labs/osmosis/v28/app/apptesting"
	concentratedmodel "github.com/osmosis-labs/osmosis/v28/x/concentrated-liquidity/model"
)

func deepCopyTickModel(tickModel *ingesttypes.TickModel) *ingesttypes.TickModel {
	ticks := make([]ingesttypes.LiquidityDepthsWithRange, len(tickModel.Ticks))
	copy(ticks, tickModel.Ticks)
	return &ingesttypes.TickModel{
		Ticks:            ticks,
		CurrentTickIndex: tickModel.CurrentTickIndex,
		HasNoLiquidity:   tickModel.HasNoLiquidity,
	}
}

func withHasNoLiquidity(tickModel *ingesttypes.TickModel) *ingesttypes.TickModel {
	tickModel = deepCopyTickModel(tickModel)
	tickModel.HasNoLiquidity = true
	return tickModel
}

func withCurrentTickIndex(tickModel *ingesttypes.TickModel, currentTickIndex int64) *ingesttypes.TickModel {
	tickModel = deepCopyTickModel(tickModel)
	tickModel.CurrentTickIndex = currentTickIndex
	return tickModel
}

func withTicks(tickModel *ingesttypes.TickModel, ticks []ingesttypes.LiquidityDepthsWithRange) *ingesttypes.TickModel {
	tickModel = deepCopyTickModel(tickModel)
	tickModel.Ticks = ticks
	return tickModel
}

// Tests the CalculateTokenOutByTokenIn method of the RoutableConcentratedPoolImpl struct
// when the pool is concentrated.
//
// It uses the same success test cases as the chain logic.
// The error cases are tested in a separate fixture because the edge cases are different..
func (s *RoutablePoolTestSuite) TestCalculateTokenOutByTokenIn_Concentrated_SuccessChainVectors() {
	tests := apptesting.SwapOutGivenInCases

	for name, tc := range tests {
		s.Run(name, func() {
			// Note: router quote tests do not have the concept of slippage protection.
			// These quotes are used to derive the slippage protection amount.
			// So we skip these tests.
			if strings.Contains(name, "slippage protection") {
				s.T().Skip("no slippage protection in router quote tests")
			}

			s.SetupAndFundSwapTest()
			concentratedPool := s.PreparePoolWithCustSpread(tc.SpreadFactor)
			// add default position
			s.SetupDefaultPosition(concentratedPool.GetId())
			s.SetupSecondPosition(tc, concentratedPool)

			// Refetch the pool
			concentratedPool, err := s.App.ConcentratedLiquidityKeeper.GetConcentratedPoolById(s.Ctx, concentratedPool.GetId())
			s.Require().NoError(err)

			// Get liquidity for full range
			ticks, currentTickIndex, err := s.App.ConcentratedLiquidityKeeper.GetTickLiquidityForFullRange(s.Ctx, concentratedPool.GetId())
			s.Require().NoError(err)

			poolWrapper := &ingesttypes.PoolWrapper{
				ChainModel: concentratedPool,
				TickModel: &ingesttypes.TickModel{
					Ticks:            ticks,
					CurrentTickIndex: currentTickIndex,
					HasNoLiquidity:   false,
				},
				SQSModel: ingesttypes.SQSPool{
					PoolLiquidityCap:      osmomath.NewInt(100),
					PoolLiquidityCapError: "",
					Balances:              sdk.Coins{},
					PoolDenoms:            []string{"foo", "bar"},
				},
			}
			cosmWasmPoolsParams := cosmwasmdomain.CosmWasmPoolsParams{
				ScalingFactorGetterCb: domain.UnsetScalingFactorGetterCb,
			}
			routablePool, err := pools.NewRoutablePool(poolWrapper, tc.TokenInDenom, tc.TokenOutDenom, noTakerFee, cosmWasmPoolsParams)
			s.Require().NoError(err)

			tokenOut, err := routablePool.CalculateTokenOutByTokenIn(context.TODO(), tc.TokenIn)

			s.Require().NoError(err)
			s.Require().Equal(tc.ExpectedTokenOut.String(), tokenOut.String())
		})
	}
}

// Tests the CalculateTokenInByTokenOut method of the RoutableConcentratedPoolImpl struct
// when the pool is concentrated.
//
// It uses the same success test cases as the chain logic.
// The error cases are tested in a separate fixture because the edge cases are different..
func (s *RoutablePoolTestSuite) TestCalculateTokenInByTokenOut_Concentrated_SuccessChainVectors() {
	tests := apptesting.SwapInGivenOutCases

	for name, tc := range tests {
		s.Run(name, func() {
			// Note: router quote tests do not have the concept of slippage protection.
			// These quotes are used to derive the slippage protection amount.
			// So we skip these tests.
			if strings.Contains(name, "slippage protection") {
				s.T().Skip("no slippage protection in router quote tests")
			}

			s.SetupAndFundSwapTest()
			concentratedPool := s.PreparePoolWithCustSpread(tc.SpreadFactor)
			// add default position
			s.SetupDefaultPosition(concentratedPool.GetId())
			s.SetupSecondPosition(tc, concentratedPool)

			// Refetch the pool
			concentratedPool, err := s.App.ConcentratedLiquidityKeeper.GetConcentratedPoolById(s.Ctx, concentratedPool.GetId())
			s.Require().NoError(err)

			// Get liquidity for full range
			ticks, currentTickIndex, err := s.App.ConcentratedLiquidityKeeper.GetTickLiquidityForFullRange(s.Ctx, concentratedPool.GetId())
			s.Require().NoError(err)

			poolWrapper := &ingesttypes.PoolWrapper{
				ChainModel: concentratedPool,
				TickModel: &ingesttypes.TickModel{
					Ticks:            ticks,
					CurrentTickIndex: currentTickIndex,
					HasNoLiquidity:   false,
				},
				SQSModel: ingesttypes.SQSPool{
					PoolLiquidityCap:      osmomath.NewInt(100),
					PoolLiquidityCapError: "",
					Balances:              sdk.Coins{},
					PoolDenoms:            []string{"foo", "bar"},
				},
			}
			cosmWasmPoolsParams := cosmwasmdomain.CosmWasmPoolsParams{
				ScalingFactorGetterCb: domain.UnsetScalingFactorGetterCb,
			}

			routablePool, err := pools.NewRoutablePool(poolWrapper, tc.TokenInDenom, tc.TokenOutDenom, noTakerFee, cosmWasmPoolsParams)
			s.Require().NoError(err)

			tokenIn, err := routablePool.CalculateTokenInByTokenOut(context.TODO(), tc.TokenOut)

			s.Require().NoError(err)
			s.Require().Equal(tc.ExpectedTokenIn.String(), tokenIn.String())
		})
	}
}

// This test cases focuses on testing error and edge cases for CL quote calculation out by token in.
func (s *RoutablePoolTestSuite) TestCalculateTokenOutByTokenIn_Concentrated_ErrorAndEdgeCases() {
	const (
		defaultCurrentTick = int64(0)
	)

	tests := map[string]struct {
		tokenIn       sdk.Coin
		tokenOutDenom string

		tickModelOverwrite          *ingesttypes.TickModel
		isTickModelNil              bool
		shouldCreateDefaultPosition bool

		expectedTokenOut sdk.Coin
		expectError      error
	}{
		"error: failed to get tick model": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			isTickModelNil: true,

			expectError: domain.ConcentratedPoolNoTickModelError{
				PoolId: defaultPoolID,
			},
		},
		"error: current bucket index is negative": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			tickModelOverwrite: withCurrentTickIndex(defaultTickModel, -1),

			expectError: domain.ConcentratedCurrentTickNotWithinBucketError{
				PoolId:             defaultPoolID,
				CurrentBucketIndex: -1,
				TotalBuckets:       0,
			},
		},
		"error: current bucket index is greater than or equal to total buckets": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			tickModelOverwrite: defaultTickModel,

			expectError: domain.ConcentratedCurrentTickNotWithinBucketError{
				PoolId:             defaultPoolID,
				CurrentBucketIndex: defaultCurrentTick,
				TotalBuckets:       defaultCurrentTick,
			},
		},
		"error: has no liquidity": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			tickModelOverwrite: withHasNoLiquidity(defaultTickModel),

			expectError: domain.ConcentratedNoLiquidityError{
				PoolId: defaultPoolID,
			},
		},
		"error: current tick is not within current bucket": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			tickModelOverwrite: withTicks(defaultTickModel, []ingesttypes.LiquidityDepthsWithRange{
				{
					LowerTick:       defaultCurrentTick - 2,
					UpperTick:       defaultCurrentTick - 1,
					LiquidityAmount: DefaultLiquidityAmt,
				},
			}),

			expectError: domain.ConcentratedCurrentTickAndBucketMismatchError{
				CurrentTick: defaultCurrentTick,
				LowerTick:   defaultCurrentTick - 2,
				UpperTick:   defaultCurrentTick - 1,
			},
		},
		"error: zero current sqrt price": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			tickModelOverwrite: &ingesttypes.TickModel{
				Ticks: []ingesttypes.LiquidityDepthsWithRange{
					{
						LowerTick:       defaultCurrentTick,
						UpperTick:       defaultCurrentTick + 1,
						LiquidityAmount: DefaultLiquidityAmt,
					},
				},
				CurrentTickIndex: defaultCurrentTick,

				// Note that despite setting HasNoLiquidity to false,
				// the pool is in invalid state. We expect that the ingester
				// will not allow this to happen.
				HasNoLiquidity: false,
			},

			expectError: domain.ConcentratedZeroCurrentSqrtPriceError{PoolId: defaultPoolID},
		},
		"error: not enough liquidity to complete swap": {
			tokenIn:       DefaultCoin1,
			tokenOutDenom: Denom0,

			shouldCreateDefaultPosition: true,

			tickModelOverwrite: withTicks(defaultTickModel, []ingesttypes.LiquidityDepthsWithRange{
				{
					LowerTick:       DefaultCurrentTick,
					UpperTick:       DefaultCurrentTick + 1,
					LiquidityAmount: DefaultLiquidityAmt,
				},
			}),

			expectError: domain.ConcentratedNotEnoughLiquidityToCompleteSwapError{
				PoolId:   defaultPoolID,
				AmountIn: DefaultCoin1.String(),
			},
		},
	}

	for name, tc := range tests {
		s.Run(name, func() {
			s.SetupTest()

			var (
				tickModel *ingesttypes.TickModel
				err       error
			)

			pool := s.PrepareConcentratedPool()
			concentratedPool, ok := pool.(*concentratedmodel.Pool)
			s.Require().True(ok)

			if tc.shouldCreateDefaultPosition {
				s.SetupDefaultPosition(concentratedPool.Id)
			}

			// refetch the pool
			pool, err = s.App.ConcentratedLiquidityKeeper.GetConcentratedPoolById(s.Ctx, concentratedPool.Id)
			s.Require().NoError(err)
			concentratedPool, ok = pool.(*concentratedmodel.Pool)
			s.Require().True(ok)

			if tc.tickModelOverwrite != nil {
				tickModel = tc.tickModelOverwrite

			} else if tc.isTickModelNil {
				// For clarity:
				tickModel = nil
			} else {
				// Get liquidity for full range
				ticks, currentTickIndex, err := s.App.ConcentratedLiquidityKeeper.GetTickLiquidityForFullRange(s.Ctx, concentratedPool.Id)
				s.Require().NoError(err)

				tickModel = &ingesttypes.TickModel{
					Ticks:            ticks,
					CurrentTickIndex: currentTickIndex,
					HasNoLiquidity:   false,
				}
			}

			routablePool := pools.RoutableConcentratedPoolImpl{
				ChainPool:     concentratedPool,
				TickModel:     tickModel,
				TokenOutDenom: tc.tokenOutDenom,
				TakerFee:      osmomath.ZeroDec(),
			}

			tokenOut, err := routablePool.CalculateTokenOutByTokenIn(context.TODO(), tc.tokenIn)

			if tc.expectError != nil {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expectError.Error())
				return
			}
			s.Require().NoError(err)

			s.Require().Equal(tc.expectedTokenOut.String(), tokenOut.String())
		})
	}
}

// This test cases focuses on testing error and edge cases for CL quote calculation in by token out.
func (s *RoutablePoolTestSuite) TestCalculateTokenInByTokenOut_Concentrated_ErrorAndEdgeCases() {
	const (
		defaultCurrentTick = int64(0)
	)

	tests := map[string]struct {
		tokenOut     sdk.Coin
		tokenInDenom string

		tickModelOverwrite          *ingesttypes.TickModel
		isTickModelNil              bool
		shouldCreateDefaultPosition bool

		expectedTokenIn sdk.Coin
		expectError     error
	}{
		"error: failed to get tick model": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			isTickModelNil: true,

			expectError: domain.ConcentratedPoolNoTickModelError{
				PoolId: defaultPoolID,
			},
		},
		"error: current bucket index is negative": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			tickModelOverwrite: withCurrentTickIndex(defaultTickModel, -1),

			expectError: domain.ConcentratedCurrentTickNotWithinBucketError{
				PoolId:             defaultPoolID,
				CurrentBucketIndex: -1,
				TotalBuckets:       0,
			},
		},
		"error: current bucket index is greater than or equal to total buckets": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			tickModelOverwrite: defaultTickModel,

			expectError: domain.ConcentratedCurrentTickNotWithinBucketError{
				PoolId:             defaultPoolID,
				CurrentBucketIndex: defaultCurrentTick,
				TotalBuckets:       defaultCurrentTick,
			},
		},
		"error: has no liquidity": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			tickModelOverwrite: withHasNoLiquidity(defaultTickModel),

			expectError: domain.ConcentratedNoLiquidityError{
				PoolId: defaultPoolID,
			},
		},
		"error: current tick is not within current bucket": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			tickModelOverwrite: withTicks(defaultTickModel, []ingesttypes.LiquidityDepthsWithRange{
				{
					LowerTick:       defaultCurrentTick - 2,
					UpperTick:       defaultCurrentTick - 1,
					LiquidityAmount: DefaultLiquidityAmt,
				},
			}),

			expectError: domain.ConcentratedCurrentTickAndBucketMismatchError{
				CurrentTick: defaultCurrentTick,
				LowerTick:   defaultCurrentTick - 2,
				UpperTick:   defaultCurrentTick - 1,
			},
		},
		"error: zero current sqrt price": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			tickModelOverwrite: &ingesttypes.TickModel{
				Ticks: []ingesttypes.LiquidityDepthsWithRange{
					{
						LowerTick:       defaultCurrentTick,
						UpperTick:       defaultCurrentTick + 1,
						LiquidityAmount: DefaultLiquidityAmt,
					},
				},
				CurrentTickIndex: defaultCurrentTick,

				// Note that despite setting HasNoLiquidity to false,
				// the pool is in invalid state. We expect that the ingester
				// will not allow this to happen.
				HasNoLiquidity: false,
			},

			expectError: domain.ConcentratedZeroCurrentSqrtPriceError{PoolId: defaultPoolID},
		},
		"error: not enough liquidity to complete swap": {
			tokenOut:     DefaultCoin1,
			tokenInDenom: Denom0,

			shouldCreateDefaultPosition: true,

			tickModelOverwrite: withTicks(defaultTickModel, []ingesttypes.LiquidityDepthsWithRange{
				{
					LowerTick:       DefaultCurrentTick,
					UpperTick:       DefaultCurrentTick + 1,
					LiquidityAmount: DefaultLiquidityAmt,
				},
			}),

			expectError: domain.ConcentratedNotEnoughLiquidityToCompleteSwapError{
				PoolId:    defaultPoolID,
				AmountOut: DefaultCoin1.String(),
			},
		},
	}

	for name, tc := range tests {
		s.Run(name, func() {
			s.SetupTest()

			var (
				tickModel *ingesttypes.TickModel
				err       error
			)

			pool := s.PrepareConcentratedPool()
			concentratedPool, ok := pool.(*concentratedmodel.Pool)
			s.Require().True(ok)

			if tc.shouldCreateDefaultPosition {
				s.SetupDefaultPosition(concentratedPool.Id)
			}

			// refetch the pool
			pool, err = s.App.ConcentratedLiquidityKeeper.GetConcentratedPoolById(s.Ctx, concentratedPool.Id)
			s.Require().NoError(err)
			concentratedPool, ok = pool.(*concentratedmodel.Pool)
			s.Require().True(ok)

			if tc.tickModelOverwrite != nil {
				tickModel = tc.tickModelOverwrite

			} else if tc.isTickModelNil {
				// For clarity:
				tickModel = nil
			} else {
				// Get liquidity for full range
				ticks, currentTickIndex, err := s.App.ConcentratedLiquidityKeeper.GetTickLiquidityForFullRange(s.Ctx, concentratedPool.Id)
				s.Require().NoError(err)

				tickModel = &ingesttypes.TickModel{
					Ticks:            ticks,
					CurrentTickIndex: currentTickIndex,
					HasNoLiquidity:   false,
				}
			}

			routablePool := pools.RoutableConcentratedPoolImpl{
				ChainPool:    concentratedPool,
				TickModel:    tickModel,
				TokenInDenom: tc.tokenInDenom,
				TakerFee:     osmomath.ZeroDec(),
			}

			tokenIn, err := routablePool.CalculateTokenInByTokenOut(context.TODO(), tc.tokenOut)

			if tc.expectError != nil {
				s.Require().Error(err)
				s.Require().ErrorContains(err, tc.expectError.Error())
				return
			}
			s.Require().NoError(err)

			s.Require().Equal(tc.expectedTokenIn.String(), tokenIn.String())
		})
	}
}
