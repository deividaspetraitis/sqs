package parsing_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/osmosis-labs/osmosis/v28/ingest/types/json"
	"github.com/osmosis-labs/sqs/domain/mocks"
	ingesttypes "github.com/osmosis-labs/sqs/ingest/types"
	"github.com/osmosis-labs/sqs/router/usecase/routertesting"
	"github.com/osmosis-labs/sqs/router/usecase/routertesting/parsing"

	"github.com/osmosis-labs/osmosis/osmomath"
	concentratedmodel "github.com/osmosis-labs/osmosis/v28/x/concentrated-liquidity/model"
	poolmanagertypes "github.com/osmosis-labs/osmosis/v28/x/poolmanager/types"
)

const testFileName = "pools.json"

var (
	zeroMinPoolLiquidityCap                   = osmomath.ZeroInt()
	testPoolToMarshal       ingesttypes.PoolI = &mocks.MockRoutablePool{
		ChainPoolModel: &concentratedmodel.Pool{
			Id:                   1,
			Token0:               routertesting.Denom0,
			Token1:               routertesting.Denom1,
			CurrentTickLiquidity: routertesting.DefaultLiquidityAmt,
			CurrentTick:          routertesting.DefaultCurrentTick,
			TickSpacing:          1,
			LastLiquidityUpdate:  time.Unix(1, 1).UTC(),
			SpreadFactor:         routertesting.DefaultSpreadFactor,
			CurrentSqrtPrice:     osmomath.OneBigDec(),
		},
		PoolLiquidityCap: osmomath.OneInt(),
		Balances:         routertesting.DefaultPoolBalances,
		Denoms:           []string{routertesting.Denom0, routertesting.Denom1},
		SpreadFactor:     routertesting.DefaultSpreadFactor,
		PoolType:         poolmanagertypes.Concentrated,
	}

	defaultTickModel = ingesttypes.TickModel{
		Ticks: []ingesttypes.LiquidityDepthsWithRange{
			{
				LiquidityAmount: osmomath.OneDec(),
				LowerTick:       1,
				UpperTick:       2,
			},
		},
		CurrentTickIndex: 0,
		HasNoLiquidity:   false,
	}
)

// This test validates that ReadPools can read a file from the state.
func TestReadPoolsFileFromState(t *testing.T) {
	t.Skip("This test is not meant to be run in CI. Use for debugging only")

	pools, _, err := parsing.ReadPools(testFileName)
	require.NoError(t, err)

	require.NotEmpty(t, pools)
	require.Greater(t, len(pools), 500)

	for _, pool := range pools {
		err := pool.Validate(zeroMinPoolLiquidityCap)
		if err != nil {
			t.Logf("pool %d failed validation: %s", pool.GetId(), err)
		}
	}
}

// This test validates that StorePools succesfull stores pools to a file
// that ReadPools can then read back into the system.
func TestStoreFilesAndReadBack(t *testing.T) {

	t.Skip("This test is not meant to be run in CI. Use for debugging only")

	// Delete test file if exists since the system under test recreates it.
	_, err := os.Stat(testFileName)
	if err == nil {
		err = os.Remove(testFileName)
		require.NoError(t, err)
	}

	err = parsing.StorePools([]ingesttypes.PoolI{testPoolToMarshal}, map[uint64]*ingesttypes.TickModel{
		testPoolToMarshal.GetId(): &defaultTickModel,
	}, testFileName)
	require.NoError(t, err)

	pools, _, err := parsing.ReadPools(testFileName)
	require.NoError(t, err)

	require.Equal(t, 1, len(pools))
	for _, pool := range pools {
		require.NoError(t, pool.Validate(zeroMinPoolLiquidityCap))
	}
}

// This test validates that unmarshalling and marshalling a pool works as expected.
func TestMarshalUnmarshalPool(t *testing.T) {
	serializedPools, err := parsing.MarshalPool(testPoolToMarshal)
	require.NoError(t, err)

	var interimPools parsing.SerializedPool
	err = json.Unmarshal(serializedPools, &interimPools)
	require.NoError(t, err)

	unmarshalledPool, err := parsing.UnmarshalPool(interimPools)
	require.NoError(t, err)

	require.Equal(t, testPoolToMarshal.GetUnderlyingPool(), unmarshalledPool.GetUnderlyingPool())
	require.Equal(t, testPoolToMarshal.GetSQSPoolModel(), unmarshalledPool.GetSQSPoolModel())
}

// This test validates that unmarshalling and marshalling a taker fee map works as expected.
func TestMarshalUnmarshalTakerFeeMap(t *testing.T) {
	takerFeeMap := ingesttypes.TakerFeeMap{
		ingesttypes.DenomPair{
			Denom0: routertesting.Denom0,
			Denom1: routertesting.Denom1,
		}: osmomath.OneDec(),
	}

	takerFeeMapBz, err := json.Marshal(takerFeeMap)
	require.NoError(t, err)

	unmarshalledTakerFeeMap := ingesttypes.TakerFeeMap{}
	err = json.Unmarshal(takerFeeMapBz, &unmarshalledTakerFeeMap)
	require.NoError(t, err)

	require.Equal(t, takerFeeMap, unmarshalledTakerFeeMap)
}
