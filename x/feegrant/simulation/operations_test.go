package simulation_test

import (
	"math/rand"
	"testing"
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/cosmos/cosmos-sdk/simapp"
	simappparams "github.com/cosmos/cosmos-sdk/simapp/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/feegrant/simulation"
	"github.com/cosmos/cosmos-sdk/x/feegrant/types"
	"github.com/stretchr/testify/suite"
)

type SimTestSuite struct {
	suite.Suite

	ctx sdk.Context
	app *simapp.SimApp
}

func (suite *SimTestSuite) SetupTest() {
	checkTx := false
	app := simapp.Setup(checkTx)
	suite.app = app
	suite.ctx = app.BaseApp.NewContext(checkTx, tmproto.Header{})
}

func (suite *SimTestSuite) getTestingAccounts(r *rand.Rand, n int) []simtypes.Account {
	app, ctx := suite.app, suite.ctx
	accounts := simtypes.RandomAccounts(r, n)
	require := suite.Require()

	initAmt := sdk.TokensFromConsensusPower(200)
	initCoins := sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, initAmt))

	// add coins to the accounts
	for _, account := range accounts {
		acc := app.AccountKeeper.NewAccountWithAddress(ctx, account.Address)
		app.AccountKeeper.SetAccount(ctx, acc)
		err := app.BankKeeper.SetBalances(ctx, account.Address, initCoins)
		require.NoError(err)
	}

	return accounts
}

func (suite *SimTestSuite) TestWeightedOperations() {
	app, ctx := suite.app, suite.ctx
	require := suite.Require()

	ctx.WithChainID("test-chain")

	cdc := app.AppCodec()
	appParams := make(simtypes.AppParams)

	weightesOps := simulation.WeightedOperations(
		appParams, cdc, app.AccountKeeper,
		app.BankKeeper, app.FeeGrantKeeper,
	)

	s := rand.NewSource(1)
	r := rand.New(s)
	accs := suite.getTestingAccounts(r, 3)

	expected := []struct {
		weight     int
		opMsgRoute string
		opMsgName  string
	}{
		{
			simappparams.DefaultWeightGrantFeeAllowance,
			types.ModuleName,
			types.TypeMsgGrantFeeAllowance,
		},
		{
			simappparams.DefaultWeightRevokeFeeAllowance,
			types.ModuleName,
			types.TypeMsgRevokeFeeAllowance,
		},
	}

	for i, w := range weightesOps {
		operationMsg, _, _ := w.Op()(r, app.BaseApp, ctx, accs, ctx.ChainID())
		// the following checks are very much dependent from the ordering of the output given
		// by WeightedOperations. if the ordering in WeightedOperations changes some tests
		// will fail
		require.Equal(expected[i].weight, w.Weight(), "weight should be the same")
		require.Equal(expected[i].opMsgRoute, operationMsg.Route, "route should be the same")
		require.Equal(expected[i].opMsgName, operationMsg.Name, "operation Msg name should be the same")
	}
}

func (suite *SimTestSuite) TestSimulateMsgGrantFeeAllowance() {
	app, ctx := suite.app, suite.ctx
	require := suite.Require()

	s := rand.NewSource(1)
	r := rand.New(s)
	accounts := suite.getTestingAccounts(r, 3)

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: app.LastBlockHeight() + 1, AppHash: app.LastCommitID().Hash}})

	// execute operation
	op := simulation.SimulateMsgGrantFeeAllowance(app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper)
	operationMsg, futureOperations, err := op(r, app.BaseApp, ctx, accounts, "")
	require.NoError(err)

	var msg types.MsgGrantFeeAllowance
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	require.True(operationMsg.OK)
	require.Equal("cosmos1ghekyjucln7y67ntx7cf27m9dpuxxemn4c8g4r", msg.Granter.String())
	require.Equal("cosmos1p8wcgrjr4pjju90xg6u9cgq55dxwq8j7u4x9a0", msg.Grantee.String())
	require.Equal(types.TypeMsgGrantFeeAllowance, msg.Type())
	require.Equal(types.ModuleName, msg.Route())
	require.Len(futureOperations, 0)
}

func (suite *SimTestSuite) TestSimulateMsgRevokeFeeAllowance() {
	app, ctx := suite.app, suite.ctx
	require := suite.Require()

	s := rand.NewSource(1)
	r := rand.New(s)
	accounts := suite.getTestingAccounts(r, 3)

	// begin a new block
	app.BeginBlock(abci.RequestBeginBlock{Header: tmproto.Header{Height: suite.app.LastBlockHeight() + 1, AppHash: suite.app.LastCommitID().Hash}})

	feeAmt := sdk.TokensFromConsensusPower(200000)
	feeCoins := sdk.NewCoins(sdk.NewCoin("foo", feeAmt))

	granter, grantee := accounts[0], accounts[1]

	app.FeeGrantKeeper.GrantFeeAllowance(
		ctx,
		granter.Address,
		grantee.Address,
		&types.BasicFeeAllowance{
			SpendLimit: feeCoins,
			Expiration: types.ExpiresAtTime(ctx.BlockTime().Add(30 * time.Hour)),
		},
	)

	// execute operation
	op := simulation.SimulateMsgRevokeFeeAllowance(app.AccountKeeper, app.BankKeeper, app.FeeGrantKeeper)
	operationMsg, futureOperations, err := op(r, app.BaseApp, ctx, accounts, "")
	require.NoError(err)

	var msg types.MsgRevokeFeeAllowance
	types.ModuleCdc.UnmarshalJSON(operationMsg.Msg, &msg)

	require.True(operationMsg.OK)
	require.Equal(granter.Address, msg.Granter)
	require.Equal(grantee.Address, msg.Grantee)
	require.Equal(types.TypeMsgRevokeFeeAllowance, msg.Type())
	require.Equal(types.ModuleName, msg.Route())
	require.Len(futureOperations, 0)
}

func TestSimTestSuite(t *testing.T) {
	suite.Run(t, new(SimTestSuite))
}
