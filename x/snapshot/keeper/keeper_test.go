package keeper

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/axelarnetwork/axelar-core/testutils"
	"github.com/axelarnetwork/axelar-core/testutils/fake"
	"github.com/axelarnetwork/axelar-core/x/snapshot/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/libs/log"

	sdkExported "github.com/cosmos/cosmos-sdk/x/staking/exported"
	staking "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var stringGen = testutils.RandStrings(5, 50).Distinct()

// Cases to test
var testCases = []struct {
	numValidators, totalPower int
}{
	{
		numValidators: 5,
		totalPower:    50,
	},
	{
		numValidators: 10,
		totalPower:    100,
	},
	{
		numValidators: 3,
		totalPower:    10,
	},
}

func init() {
	// Necessary if tests execute with the real sdk staking keeper
	cdc := testutils.Codec()
	cdc.RegisterConcrete("", "string", nil)
	staking.RegisterCodec(cdc)

}

// Tests the snapshot functionality
func TestSnapshots(t *testing.T) {
	for i, params := range testCases {
		t.Run(fmt.Sprintf("Test-%d", i), func(t *testing.T) {
			ctx := sdk.NewContext(fake.NewMultiStore(), abci.Header{}, false, log.TestingLogger())
			cdc := testutils.Codec()
			validators := genValidators(t, params.numValidators, params.totalPower)
			staker := newMockStaker(validators...)

			assert.True(t, staker.GetLastTotalPower(ctx).Equal(sdk.NewInt(int64(params.totalPower))))

			keeper := NewKeeper(cdc, sdk.NewKVStoreKey("staking"), staker)

			_, ok := keeper.GetSnapshot(ctx, 0)

			assert.False(t, ok)
			assert.Equal(t, keeper.GetLatestRound(ctx), int64(-1))

			_, ok = keeper.GetLatestSnapshot(ctx)

			assert.False(t, ok)

			err := keeper.TakeSnapshot(ctx)

			assert.NoError(t, err)

			snapshot, ok := keeper.GetSnapshot(ctx, 0)

			assert.True(t, ok)
			assert.Equal(t, keeper.GetLatestRound(ctx), int64(0))
			for i, val := range validators {
				assert.Equal(t, val.GetConsensusPower(), snapshot.Validators[i].GetConsensusPower())
				assert.Equal(t, val.GetOperator(), snapshot.Validators[i].GetOperator())
			}

			err = keeper.TakeSnapshot(ctx)

			assert.Error(t, err)

			ctx = ctx.WithBlockTime(ctx.BlockTime().Add(interval + 100))

			err = keeper.TakeSnapshot(ctx)

			assert.NoError(t, err)

			snapshot, ok = keeper.GetSnapshot(ctx, 1)

			assert.True(t, ok)
			assert.Equal(t, keeper.GetLatestRound(ctx), int64(1))
			for i, val := range validators {
				assert.Equal(t, val.GetConsensusPower(), snapshot.Validators[i].GetConsensusPower())
				assert.Equal(t, val.GetOperator(), snapshot.Validators[i].GetOperator())
			}
		})
	}
}

// This function returns a set of validators whose voting power adds up to the specified total power
func genValidators(t *testing.T, numValidators, totalConsPower int) []sdkExported.ValidatorI {
	t.Logf("Total Power: %v", totalConsPower)

	validators := make([]sdkExported.ValidatorI, numValidators)

	quotient, remainder := totalConsPower/numValidators, totalConsPower%numValidators

	for i := 0; i < numValidators; i++ {
		power := quotient
		if i == 0 {
			power += remainder
		}

		validators[i] = staking.Validator{
			OperatorAddress: sdk.ValAddress(stringGen.Next()),
			Tokens:          sdk.TokensFromConsensusPower(int64(power)),
			Status:          sdk.Bonded,
		}
	}

	return validators
}

var _ types.StakingKeeper = mockStaker{}

type mockStaker struct {
	validators []sdkExported.ValidatorI
	totalPower sdk.Int
}

func newMockStaker(validators ...sdkExported.ValidatorI) *mockStaker {
	keeper := &mockStaker{
		make([]sdkExported.ValidatorI, 0),
		sdk.ZeroInt(),
	}

	for _, val := range validators {
		keeper.validators = append(keeper.validators, val)
		keeper.totalPower = keeper.totalPower.AddRaw(val.GetConsensusPower())
	}

	return keeper
}

func (k mockStaker) GetLastTotalPower(_ sdk.Context) (power sdk.Int) {
	return k.totalPower
}

func (k mockStaker) IterateLastValidators(_ sdk.Context, fn func(index int64, validator sdkExported.ValidatorI) (stop bool)) {
	for i, val := range k.validators {
		fn(int64(i), val)
	}
}

func (k mockStaker) Validator(_ sdk.Context, addr sdk.ValAddress) sdkExported.ValidatorI {
	for _, validator := range k.validators {
		if bytes.Equal(validator.GetOperator(), addr) {
			return validator
		}
	}

	return nil
}