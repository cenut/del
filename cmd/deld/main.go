package main
import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
	"github.com/DEL-ORG/del/accounts"
	"github.com/DEL-ORG/del/accounts/keystore"
	"github.com/DEL-ORG/del/cmd/utils"
	"github.com/DEL-ORG/del/common"
	"github.com/DEL-ORG/del/console"
	"github.com/DEL-ORG/del/eth"
	"github.com/DEL-ORG/del/ethclient"
	"github.com/DEL-ORG/del/internal/debug"
	"github.com/DEL-ORG/del/log"
	"github.com/DEL-ORG/del/metrics"
	"github.com/DEL-ORG/del/node"
	"gopkg.in/urfave/cli.v1"
	"github.com/DEL-ORG/del/restrpc/rpcydoon"
	"math/rand"
)
var (
	gitCommit = ""
	relOracle = common.HexToAddress("0xfa7b9770ca4cb04296cac84f37736d4041251cdf")
	app = utils.NewApp(gitCommit, "the Double Entry Ledger command line interface")
	nodeFlags = []cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.BootnodesV4Flag,
		utils.BootnodesV5Flag,
		utils.DataDirFlag,
		utils.KeyStoreDirFlag,
		utils.DashboardEnabledFlag,
		utils.DashboardAddrFlag,
		utils.DashboardPortFlag,
		utils.DashboardRefreshFlag,
		utils.DashboardAssetsFlag,
		
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		utils.GCModeFlag,
		
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheGCFlag,
		utils.TrieCacheGenFlag,
		utils.ListenPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.EtherbaseFlag,
		utils.GasPriceFlag,
		utils.MinerThreadsFlag,
		utils.MiningEnabledFlag,
		utils.TargetGasLimitFlag,
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV5Flag,
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		
		utils.TestnetFlag,
		utils.NetworkIdFlag,
		utils.RPCCORSDomainFlag,
		utils.RPCVirtualHostsFlag,
		utils.MetricsEnabledFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.ExtraDataFlag,
		configFileFlag,
	}
	rpcFlags = []cli.Flag{
		utils.RPCEnabledFlag,
		utils.RPCListenAddrFlag,
		utils.RPCPortFlag,
		utils.RPCApiFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
	}
	whisperFlags = []cli.Flag{
		utils.WhisperEnabledFlag,
		utils.WhisperMaxMessageSizeFlag,
		utils.WhisperMinPOWFlag,
	}
)
func init() {
	app.Action = geth
	app.HideVersion = true 
	app.Copyright = "Copyright 2018-2028 The Double Entry Ledger Authors"
	app.Commands = []cli.Command{
		importCommand,
		exportCommand,
		copydbCommand,
		removedbCommand,
		dumpCommand,
		monitorCommand,
		accountCommand,
		consoleCommand,
		attachCommand,
		javascriptCommand,
		
		versionCommand,
		
		dumpConfigCommand,
	}
	sort.Sort(cli.CommandsByName(app.Commands))
	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, rpcFlags...)
	app.Flags = append(app.Flags, consoleFlags...)
	app.Flags = append(app.Flags, debug.Flags...)
	app.Before = func(ctx *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		go metrics.CollectProcessMetrics(3 * time.Second)
		utils.SetupNetwork(ctx)
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		console.Stdin.Close() 
		return nil
	}
}
func HandleGC() {
	for {
		startTime := time.Now()
		runtime.GC()
		endTime := time.Now()
		duration := endTime.Sub(startTime)
		ms := duration.Nanoseconds() / 1e6
		log.Debug("deld GC", "ms", ms)
		rtime := 60 + rand.Intn(60)
		time.Sleep(time.Second * time.Duration(rtime))
	}
}
func main() {
	rand.Seed(int64(time.Now().Nanosecond()))
	go HandleGC()
	rpcydoon.GetInstance().Init()
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func geth(ctx *cli.Context) error {
	node := makeFullNode(ctx)
	startNode(ctx, node)
	node.Wait()
	return nil
}
func startNode(ctx *cli.Context, stack *node.Node) {
	utils.StartNode(stack)
	ks := stack.AccountManager().Backends(keystore.KeyStoreType)[0].(*keystore.KeyStore)
	passwords := utils.MakePasswordList(ctx)
	unlocks := strings.Split(ctx.GlobalString(utils.UnlockedAccountFlag.Name), ",")
	for i, account := range unlocks {
		if trimmed := strings.TrimSpace(account); trimmed != "" {
			unlockAccount(ctx, ks, trimmed, i, passwords)
		}
	}
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)
	go func() {
		rpcClient, err := stack.Attach()
		if err != nil {
			utils.Fatalf("Failed to attach to self: %v", err)
		}
		stateReader := ethclient.NewClient(rpcClient)
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)
				if event.Wallet.URL().Scheme == "ledger" {
					event.Wallet.SelfDerive(accounts.DefaultLedgerBaseDerivationPath, stateReader)
				} else {
					event.Wallet.SelfDerive(accounts.DefaultBaseDerivationPath, stateReader)
				}
			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()
	if ctx.GlobalBool(utils.MiningEnabledFlag.Name) || ctx.GlobalBool(utils.DeveloperFlag.Name) {
		if ctx.GlobalBool(utils.LightModeFlag.Name) || ctx.GlobalString(utils.SyncModeFlag.Name) == "light" {
			utils.Fatalf("Light clients do not support mining")
		}
		var ethereum *eth.Ethereum
		if err := stack.Service(&ethereum); err != nil {
			utils.Fatalf("Ethereum service not running: %v", err)
		}
		if threads := ctx.GlobalInt(utils.MinerThreadsFlag.Name); threads > 0 {
			type threaded interface {
				SetThreads(threads int)
			}
			if th, ok := ethereum.Engine().(threaded); ok {
				th.SetThreads(threads)
			}
		}
		ethereum.TxPool().SetGasPrice(utils.GlobalBig(ctx, utils.GasPriceFlag.Name))
		if err := ethereum.StartMining(true); err != nil {
			utils.Fatalf("Failed to start mining: %v", err)
		}
	}
}
