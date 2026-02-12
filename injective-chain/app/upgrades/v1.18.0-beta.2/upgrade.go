package v1dot18dot0

import (
	"cosmossdk.io/errors"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/ethereum/go-ethereum/common"

	"github.com/InjectiveLabs/injective-core/injective-chain/app/upgrades"
	tokenfactorytypes "github.com/InjectiveLabs/injective-core/injective-chain/modules/tokenfactory/types"
)

const UpgradeVersion = "v1.18.0-beta.2"

func StoreUpgrades() storetypes.StoreUpgrades {
	return storetypes.StoreUpgrades{}
}

func UpgradeSteps() []*upgrades.UpgradeHandlerStep {
	return []*upgrades.UpgradeHandlerStep{
		upgrades.NewUpgradeHandlerStep(
			"MINT QA TOKENS",
			UpgradeVersion,
			upgrades.TestnetChainID,
			TestnetPrintQATokens,
		),
		upgrades.NewUpgradeHandlerStep(
			"Migrate MintAmountERC20",
			UpgradeVersion,
			upgrades.MainnetChainID,
			MigrateMintAmountERC20,
		),
		upgrades.NewUpgradeHandlerStep(
			"Migrate MintAmountERC20",
			UpgradeVersion,
			upgrades.TestnetChainID,
			MigrateMintAmountERC20,
		),
		upgrades.NewUpgradeHandlerStep(
			"Migrate MintAmountERC20",
			UpgradeVersion,
			upgrades.DevnetChainID,
			MigrateMintAmountERC20,
		),
	}
}

// revive:disable:function-length // we don't care about the length of this function used only for the upgrade
// revive:disable:cognitive-complexity // we don't care about the cognitive complexity of this function used only for the upgrade
// revive:disable:cyclomatic // we don't care about the cyclomatic complexity of this function used only for the upgrade
func TestnetPrintQATokens(ctx sdk.Context, app upgrades.InjectiveApplication, _ log.Logger) error {
	switch ctx.ChainID() {
	case upgrades.TestnetChainID:
	default:
		// no-op on other chains
		return nil
	}

	bk := app.GetBankKeeper()

	const amountNatural = 1000000000 // 1 billion of units

	qaAddresses := []sdk.AccAddress{
		sdk.MustAccAddressFromBech32("inj1jtrwx3g4z9ml7ext6j3a9gx63g3ae28pyd8ehs"),
		sdk.MustAccAddressFromBech32("inj1glff2654qs682u6mpt9m80ev74ha8tlvsr37h8"),
		sdk.MustAccAddressFromBech32("inj16wke7qv7cf58jmcnwfpvrfuhynxv49h7lefj4u"),
		sdk.MustAccAddressFromBech32("inj1nfpg504zj3vllgwnewg8lqs4p3hwh7rkt900v6"),
		sdk.MustAccAddressFromBech32("inj1l4vk5smhetzhtmwqjctj7fsqhazwtzj8ungyle"),
		sdk.MustAccAddressFromBech32("inj1y0zqef7nfqjgpqacmssrdu94sev9en6gj49yfl"),
		sdk.MustAccAddressFromBech32("inj12883wgxk98z32ecnqyx2www7rnvxesg6xcrwqw"),
		sdk.MustAccAddressFromBech32("inj1sdr286pm3mwzl9dtmvgw44x0e4sgyu8yan05qc"),
		sdk.MustAccAddressFromBech32("inj1cnmagva0muacax9njjxp9agy06uff88s0evynt"),
		sdk.MustAccAddressFromBech32("inj1u05e9rz6g280x0zssez7kyqeet3jwy3n8x320g"),
		sdk.MustAccAddressFromBech32("inj1gvp4fup6ymrvtr87xnd3evcqfapk8swd7zttej"),
		sdk.MustAccAddressFromBech32("inj1hvnrer8z6t8jpgemrtu6ucz6679pxyyanvmhu5"),
		sdk.MustAccAddressFromBech32("inj1w8c30ld440xvm2ty75c37nwxaj2062zeukz356"),
		sdk.MustAccAddressFromBech32("inj1rnjpqvc7jreun3fev7qtt7fkuwd2c6emlju87w"),
		sdk.MustAccAddressFromBech32("inj1qlymjpfyflu5cmqyh0qrqrcfx3lpy54rmaekcz"),
		sdk.MustAccAddressFromBech32("inj1682wjf84ydv7my9208raw2lvusnmdmyr6h9cpm"),
		sdk.MustAccAddressFromBech32("inj17eye2vlmf8gtq3rh4l378x7xdwselxt93plg0a"),
		sdk.MustAccAddressFromBech32("inj1gzvxupte7slnerz3r4k623vfwjddustfpf04an"),
		sdk.MustAccAddressFromBech32("inj145jswjcu7fnmsl8gywfv6qwsk2j38hejjp6kk0"),
		sdk.MustAccAddressFromBech32("inj1g4x29mngaulvz4stcnrpq29ursxdrrjg96dccu"),
		sdk.MustAccAddressFromBech32("inj15u7zuzclc8fwe8dgw4uxmawhnc7znv5n6kvshp"),
		sdk.MustAccAddressFromBech32("inj1ag2vlzpnmcyh7mj634sr305kgrs04a69l4sztn"),
		sdk.MustAccAddressFromBech32("inj1j7xnhts0sm2end5k092ft4cqxea0jks66sm2ud"),
		sdk.MustAccAddressFromBech32("inj14cw0anlc5uyn4wcm5rherhglldf2tk9qnnmefh"),
		sdk.MustAccAddressFromBech32("inj1ngwz9x7efl8jqmxhctj0c0qwgh4geyvljdrgtk"),
		sdk.MustAccAddressFromBech32("inj1zt2cdxaaanjvnt52sae3me8rpgmrc8z8n3u6aa"),
		sdk.MustAccAddressFromBech32("inj16thtrqd2xg038pq7uwa94egmlng6rnq399gurs"),
		sdk.MustAccAddressFromBech32("inj1xfkf3kp8mh5zzgjf4r2sq3walpw00h2st86g8y"),
		sdk.MustAccAddressFromBech32("inj1hjqnuly4kwjla33m2h0046py84r80lte8c8xme"),
		sdk.MustAccAddressFromBech32("inj10dkyvmza3yzxtqrrtg7ze76m6nv5g6sg6pmh5r"),
		sdk.MustAccAddressFromBech32("inj126revkxwgttff4m32cgmark2wn6ufrd0sa4g7w"),
		sdk.MustAccAddressFromBech32("inj15hq7f4hdcf48e9alujced4dl50nu454vha3s2l"),
		sdk.MustAccAddressFromBech32("inj1dt46ue8vyfqvx40ystp95m06kkplwelzrqn344"),
		sdk.MustAccAddressFromBech32("inj17g8llfkk09ykc3a2k5mxenrstq837kcnqhpa7j"),
		sdk.MustAccAddressFromBech32("inj1w2w8w6dty3fgdtwqzn02pwjufg4lwstnlj5c3t"),
		sdk.MustAccAddressFromBech32("inj1yvwmtmqvz72fuh5f4dv62qkgsf8lhvpyl3cx2s"),
		sdk.MustAccAddressFromBech32("inj15chluxu2gpehyucwwz6yt8xty9kaplxhufre5x"),
		sdk.MustAccAddressFromBech32("inj158pgwdxd6x58nnkve3dma77dcafwdu6ze3rrdc"),
		sdk.MustAccAddressFromBech32("inj136neqxll993d3edtnrx5ghd2704dcemcufc97h"),
		sdk.MustAccAddressFromBech32("inj1up9s2rly59ahcnwxlff7f6pkfxjgxlzk3vhxte"),
		sdk.MustAccAddressFromBech32("inj1xmyupe062sd6fj4aw4f8l32hdxjv7jwsajnaa8"),
		sdk.MustAccAddressFromBech32("inj15y3qasxc93xvjcdgcff7pdmra7knpthhe38a85"),
		sdk.MustAccAddressFromBech32("inj1nad9xdljt8584p4t9r0ckth8hwgtmpkd90x3ur"),
		sdk.MustAccAddressFromBech32("inj1tje0q36m0d3yqxcztr8q3jtvu4xttp22ahwm2p"),
		sdk.MustAccAddressFromBech32("inj1c05w28cwqwk2aa7kdld5ttxd95epf6h3938pyj"),
		sdk.MustAccAddressFromBech32("inj1n962dq2mqrdr6ya2gkx88c4mqqk6entl0s64g7"),
		sdk.MustAccAddressFromBech32("inj1yc2xuwrrye0r4mm6rp7pvqn3mwypudd9yyzv99"),
		sdk.MustAccAddressFromBech32("inj18vc9g35qh42926aa988ncqdg8cqszvun8f4kux"),
		sdk.MustAccAddressFromBech32("inj1hn65m3lkrdwrhh5mpfkmdhewnmq6kac4zyasv7"),
		sdk.MustAccAddressFromBech32("inj1nd23uj8s78837f97xujc9cgvx7hm4txfy88c5s"),
		sdk.MustAccAddressFromBech32("inj1jwt6v8y555t8e5phrwmtm0xafvqlxrrqs2ndf6"),
		sdk.MustAccAddressFromBech32("inj1lfvcyjfvn4hpv0mza73ntf4zdcywrnvc3h78ru"),
		sdk.MustAccAddressFromBech32("inj1fywxyev00zt8kwp7htw2hd7uerccd0jnlz38vp"),
		sdk.MustAccAddressFromBech32("inj1p86q06qf6qf30a4nrpmy09k7xw6z7d3kwuvwwh"),
		sdk.MustAccAddressFromBech32("inj17sqg790wqh7lk50h3u03ekzeagnsh58am40ta0"),
		sdk.MustAccAddressFromBech32("inj1gv8xdf9dmsy7cm2mq3g8vwgrcqwj9khvq8s385"),
		sdk.MustAccAddressFromBech32("inj1nerwgn55ajrhlvmqsw8r3gqam6tcu3au78h0gc"),
		sdk.MustAccAddressFromBech32("inj14lwes02r960scad0f2n3ejd9wd5vt2nyt9nle9"),
		sdk.MustAccAddressFromBech32("inj18ckqch4myr63ss5jf0l8da57qlc0hdckqnvh7w"),
		sdk.MustAccAddressFromBech32("inj1qfrckzwg8u2scnnv47805c0e0kuq3j0wtaf65w"),
		sdk.MustAccAddressFromBech32("inj1ndhe20g0zn8k43g8tcfdc3k66gf5vvq27kfm5k"),
		sdk.MustAccAddressFromBech32("inj1uxu3rjq5v6znwqy8pxv99afuqeav4f6uayux5p"),
		sdk.MustAccAddressFromBech32("inj1nm4ya6an0fsqx4p2lra0n6daym3ruq5mq2lfvl"),
		sdk.MustAccAddressFromBech32("inj1h8xtrzd4r98u43dd8qg3vrzaa20ewvsylqly9e"),
		sdk.MustAccAddressFromBech32("inj1yu2l44u73sutkyhmld05jnust4rkzpzf9x468n"),
		sdk.MustAccAddressFromBech32("inj1f4xrq6472s7z5830m6nqgrvrv36catjj2mnx86"),
		sdk.MustAccAddressFromBech32("inj1l0q2zd0f372aswp2q7y7jjra8nhzynr2e54550"),
		sdk.MustAccAddressFromBech32("inj13x2n5665lhyg4aw96mnx3cr79ffamhkgpwv7gw"),
		sdk.MustAccAddressFromBech32("inj1xxaj3pvtsaf7mcdxv48mskj4djyjpr64t30ry7"),
		sdk.MustAccAddressFromBech32("inj1f5vx49d9x7j6q57z4ch47wmn73j42p83mcq92x"),
		sdk.MustAccAddressFromBech32("inj1clxfhdt3q2np9tcdcrvmw3wvru82pygnf8x2vx"),
		sdk.MustAccAddressFromBech32("inj1ga8pssyfa0wt3xzlpnayseh8vn78fdf73e3j6s"),
		sdk.MustAccAddressFromBech32("inj1aazunpd3e5jkj6n75cuxc9c6mxm5huq04jk72m"),
		sdk.MustAccAddressFromBech32("inj1484xzgpgz3pvdp0u49ef52dgavt04qxzkljn2h"),
		sdk.MustAccAddressFromBech32("inj1nmmw2n7q77zl2wq8aqrpfxsgsn4alcpnkc58nn"),
		sdk.MustAccAddressFromBech32("inj1ra706w6qfxm9nucuw06ys3gnngl38w5sdx3qsq"),
		sdk.MustAccAddressFromBech32("inj12zwdgtjqhvumgte0y4fwuav38uzcx027kl03m4"),
		sdk.MustAccAddressFromBech32("inj10zdykrnrdq4079vcgalgur4dvrp9kgh28j6exm"),
		sdk.MustAccAddressFromBech32("inj19x5v9l6qc5yfjz9m9llelhwal4p2mctww34w73"),
		sdk.MustAccAddressFromBech32("inj150vnusjz73y3a5efck4m49umsk0zve6da5j49z"),
		sdk.MustAccAddressFromBech32("inj1fkjq3vk7xmdazjhe38hf5qn5hhglspyn2yezgm"),
		sdk.MustAccAddressFromBech32("inj1t056s0p7xfd80xzx2u3argh4vwfekqmk2syy0m"),
		sdk.MustAccAddressFromBech32("inj12hnqgdc2438ugcwz0veu4s97a348745dv79slw"),
		sdk.MustAccAddressFromBech32("inj1lle34ar8wnzrpn8cmcqhpt7e08sp2whtpdzfy0"),
		sdk.MustAccAddressFromBech32("inj1k87elnppeqcsgh0dnc9n45jnkgyc8s8ur5nnax"),
		sdk.MustAccAddressFromBech32("inj139l320uv2fd95wvplyk9zyxw5qy53nf5rr2a23"),
		sdk.MustAccAddressFromBech32("inj1aqn5nwxfdqzj2uaeeuh9xglf527yla20vzxeup"),
		sdk.MustAccAddressFromBech32("inj1dxsascm7her6gxk699hnwr7r6es87jkfe7jjun"),
		sdk.MustAccAddressFromBech32("inj18ytu4vlzu4s2yqjdztcnvg0e96aggndkgqaxtc"),
		sdk.MustAccAddressFromBech32("inj1u9ct6xj9lg4kz3tcxu6sdmh2cw4ztswk8u0wca"),
		sdk.MustAccAddressFromBech32("inj1nz5hu5akgqx99cdl28pgnyxzxjve06gluh3d5s"),
		sdk.MustAccAddressFromBech32("inj1nmyrdrlrd79hfdtylkd69uq2gfdajzarxg3f0z"),
		sdk.MustAccAddressFromBech32("inj1lamtuplytygugl05gw48xg5cpr2y7g7fcg468d"),
		sdk.MustAccAddressFromBech32("inj1x52qv75k7yc6qxw9yzdq4xaczua6mhdxm9u4ng"),
		sdk.MustAccAddressFromBech32("inj1meqvwwrctkkp7nhy53mxydw9txss9jsve64qqn"),
		sdk.MustAccAddressFromBech32("inj1n7p30krkpqjg90vgwl532hhqc38smejg2mgcud"),
		sdk.MustAccAddressFromBech32("inj1kcz680982tfg7adzturt8y39qe6pvcj55edwfg"),
		sdk.MustAccAddressFromBech32("inj19v67nnw972cjfgt7ymvnnx5uk8xz28gkr6d5gw"),
		sdk.MustAccAddressFromBech32("inj1kpdssl7s36zke2tlc0yk7sc905raccdy46gxny"),
		sdk.MustAccAddressFromBech32("inj1padtskge8wc7nvh4lyrz3d6e3g6sl0kpkg6rhl"),
		sdk.MustAccAddressFromBech32("inj1q30p67y3chu3xgk05t8h2y62z73tlpftrkgnzt"),
		sdk.MustAccAddressFromBech32("inj1tg6rlavw5jxenx2upc5u6lk36y724ghsgfnz2q"),
		sdk.MustAccAddressFromBech32("inj1438e3nhxdpr63824rmawcdq6ny6jp3q4axqzq3"),
		sdk.MustAccAddressFromBech32("inj12fx5ptgv8hkampa8kk0c3wsyykm0amhu7xhcuu"),
		sdk.MustAccAddressFromBech32("inj1ly2zp5cllll70zhuwzydz509fasv79zrj8xadu"),
		sdk.MustAccAddressFromBech32("inj1zd8e0mh95hle4l5y2n8ts94pkkzq25vg8aj0xp"),
		sdk.MustAccAddressFromBech32("inj12qlwpreyvg2lqaadta70qlxaf95t94sswkhzcr"),
		sdk.MustAccAddressFromBech32("inj1tttlthf5sfn3u0h75v6t8vx4mw6v8mf3rautrd"),
		sdk.MustAccAddressFromBech32("inj1ytex2mlywzgmpeu5y6csm3v8gmgzugwmnaqc3l"),
		sdk.MustAccAddressFromBech32("inj1ndvjq5seqfv4rhp06waxlaw5zl64fal5axu22r"),
		sdk.MustAccAddressFromBech32("inj1y2vvma7s50qy82h8hwpvddpqf04yz9ufj7zdjf"),
		sdk.MustAccAddressFromBech32("inj13s3m4r7ym3cx8s6zp4p7t4pc638302xc5epxz6"),
		sdk.MustAccAddressFromBech32("inj1lr2ams2htxmta2ymmyvfqpj6ml0m9xsrp7rpzr"),
		sdk.MustAccAddressFromBech32("inj17pr9nehqcfgt4uen7swshwlzsjmrfuredhdrln"),
		sdk.MustAccAddressFromBech32("inj18yspkg8z7x3cnmvcx84tyqf8yex49kl27p2ruu"),
		sdk.MustAccAddressFromBech32("inj1mvzt3u3q6p2hf043v9tx0g502cj87w2v70ngf3"),
		sdk.MustAccAddressFromBech32("inj1x8svlhn0yg0vcxdx8tkf58t8r46gzsdr45lex4"),
		sdk.MustAccAddressFromBech32("inj1uq9zftaxvymwz9c30fcx94j5m2wucdknlkj9uk"),
		sdk.MustAccAddressFromBech32("inj1x3fg72dwtzy06553s2ntzrmywmjezsvukq30g3"),
		sdk.MustAccAddressFromBech32("inj1nluhvgx7qnkskeeq200k782np7aayp566e5nth"),
		sdk.MustAccAddressFromBech32("inj1auqrarw50urwkt0lwxzuxt9kk3kd7v73sdsjmr"),
		sdk.MustAccAddressFromBech32("inj1efe52dqxjrm5873v5tjv3sjgae52vvvemg4kwd"),
		sdk.MustAccAddressFromBech32("inj1whs7e4cc8j759x4jl95lwjtfznhlztv54kayvh"),
		sdk.MustAccAddressFromBech32("inj1rw7whtvg5nxfp54udetuwc73pdzggw565hrxq2"),
		sdk.MustAccAddressFromBech32("inj1smmw6mr3lv6a5kj7x5y66wq6d0ma8am3s4w0vm"),
		sdk.MustAccAddressFromBech32("inj1vkecvp0h4d3ungvvytkjpfgt8yxjm5xxfjw33v"),
		sdk.MustAccAddressFromBech32("inj1jutncyf4uwedlsygwdjxxr3u29u67xykkpumm0"),
		sdk.MustAccAddressFromBech32("inj175ec25sdy7c7f3p4y4akc4s3prr60wtxlev8uh"),
		sdk.MustAccAddressFromBech32("inj1nrhkpzmwhtcclk2dtflwg2u0ztegz96tpfezxy"),
		sdk.MustAccAddressFromBech32("inj1tshy6hgvcp8l9qhczn60x83e6u2uj8x3vx74gt"),
		sdk.MustAccAddressFromBech32("inj18k70cxd82hm8e8xtdn0ezs3r8htn0zjn27th76"),
		sdk.MustAccAddressFromBech32("inj1qpy8dla0ggvv53ad4hpu4tnlf8alz3eql75xsk"),
		sdk.MustAccAddressFromBech32("inj1ne4gflhx7nnaguh38ccny7hqslqk3lrgk6mdak"),
		sdk.MustAccAddressFromBech32("inj1j6j45xuh0fp2724fcy2s74p076mj6l0ngt89fd"),
		sdk.MustAccAddressFromBech32("inj1jw9ft540xles42mdxmpdzdqfv02ewgw63vr63x"),
		sdk.MustAccAddressFromBech32("inj18ve9226e0txv6ps2egdhxffx0dapxrdcw2as8v"),
		sdk.MustAccAddressFromBech32("inj1qtz2xucqmvvvyfhmhdv29exv6try3788e976q8"),
		sdk.MustAccAddressFromBech32("inj1vg78pzwz3wzczcnvhr8z0mygnz23extzaf0eqq"),
		sdk.MustAccAddressFromBech32("inj1m6jmpqz5f689tnd9389eddeg8d82347czvv86p"),
		sdk.MustAccAddressFromBech32("inj16xk6hjqtkfxyeckwly769rcj5u2rd4yts7c7vx"),
		sdk.MustAccAddressFromBech32("inj155neyn7ypl6qfkem9h6nagruka04rvqu6f3c6s"),
		sdk.MustAccAddressFromBech32("inj1dhngzu660an7lc9pffhmdvn8369dxwc6pwmxm0"),
		sdk.MustAccAddressFromBech32("inj1tkau524uysr7t9zf8aam79z7e0hdgmjhxl3ka0"),
		sdk.MustAccAddressFromBech32("inj1es0c3ljzd8es94k78vzufawh7xz2t77fzq4pvr"),
		sdk.MustAccAddressFromBech32("inj1pv654eu5znrrq639chpavtumdqnrfqgawahtmp"),
		sdk.MustAccAddressFromBech32("inj16mvw3m7wzch997rpgksx3fcujvy96xeec2rus8"),
		sdk.MustAccAddressFromBech32("inj14fqcsljrnk3khf2dv4rwss044m77cexsg38zf6"),
		sdk.MustAccAddressFromBech32("inj1z2tgkrjk0sr6e46fa7grppen7jet2v26lv9624"),
		sdk.MustAccAddressFromBech32("inj1fz82mys40k9kvx9tdyu0nf379l94vlnt7lputf"),
		sdk.MustAccAddressFromBech32("inj1s0m352knrm09mx896yqw35vmlrsxmzvj3rqlzf"),
		sdk.MustAccAddressFromBech32("inj16xr3kmj8xc2mnj8pe64h8wht6ezuxgpg8dav4n"),
		sdk.MustAccAddressFromBech32("inj1u3947ukr429yy0xuf5f8l9vehxqlyc7u9u80nx"),
		sdk.MustAccAddressFromBech32("inj1y6hj5z657xwz8l0zulwdjd4ktm2txwa4r2l99w"),
		sdk.MustAccAddressFromBech32("inj1w3chwl2z4mtjwasypfcqrn47kuegzd55tuf5ta"),
		sdk.MustAccAddressFromBech32("inj1d862fwf6zjf5plvq3zwv29lkutvv86vyzfwyhw"),
		sdk.MustAccAddressFromBech32("inj1yvmgefgzjperyyew95wuzn3d67jyfnekuhhw9v"),
		sdk.MustAccAddressFromBech32("inj1wah6wlfy5jawqylpnu4jdx3z4k38j0a5t4wy6j"),
		sdk.MustAccAddressFromBech32("inj1tkr2vh0yg26xmm5l9kugzezvwtrpqpn2uxk9pd"),
		sdk.MustAccAddressFromBech32("inj1dnln08rrnvtsk0mlku4k30wysqpg7f6nr08kad"),
		sdk.MustAccAddressFromBech32("inj13hsn6apex9pvka0mpsr3pee3r6slh7af0evz33"),
		sdk.MustAccAddressFromBech32("inj1hpv4z9yw8r3zu2h4p7ksdu36qmd57f5udk0ppf"),
		sdk.MustAccAddressFromBech32("inj1qhnnj6nf8s4agd0nqcjw2mmzzdk0grug20fz76"),
		sdk.MustAccAddressFromBech32("inj14za00f5pwr05r6aazemr9zafnzwavnynyzhxqq"),
		sdk.MustAccAddressFromBech32("inj1sas4uelyqwdnlks9zetg5qumfnrcvmy0mr8pux"),
		sdk.MustAccAddressFromBech32("inj1gqu6hgch0aulmm0ky3cea4lmh9w4vkyd4nadww"),
		sdk.MustAccAddressFromBech32("inj1qjddm2lvaxgq2h4zx4jxy38nqrvumf9cjr6cmp"),
		sdk.MustAccAddressFromBech32("inj15ultr974g062l7hhukk9yvptlhl3elgmahuyhz"),
		sdk.MustAccAddressFromBech32("inj19m284w8xxskjn87aanyjusra362r2u7uuth8qj"),
		sdk.MustAccAddressFromBech32("inj185eks8vkfnj8uaswhug0sdc20asmhkxg92wc6u"),
		sdk.MustAccAddressFromBech32("inj1mq62fl8jjl4zhw02ggskkyzv075egwl55lwm87"),
		sdk.MustAccAddressFromBech32("inj1ngax0zzsjt2dfc7d6pgdgm62z2dprj7m6g7y38"),
		sdk.MustAccAddressFromBech32("inj1z0q200qfcn84q6tpvse0h2vaqw3zrx7etcsd9l"),
		sdk.MustAccAddressFromBech32("inj1fkm8yllzk2erdsqfhsmlqcyp720pawuf9x69cp"),
		sdk.MustAccAddressFromBech32("inj1rwnvxv42zpngynfhf0gmdh8en4cau9s6r74g5h"),
		sdk.MustAccAddressFromBech32("inj1e2wftarup0lmsev0lczqgdqe7y28vd74jm768x"),
		sdk.MustAccAddressFromBech32("inj1sumqn7sqyhvxp677nrh2hvzhyzq9g3qq57gdgs"),
		sdk.MustAccAddressFromBech32("inj1v47a3e69ymp33nn5spsdrlqmjpcu8uu7tplt9v"),
		sdk.MustAccAddressFromBech32("inj1xwccxqw9wksmgzyqfn0vsdae0ffwhe253flwcu"),
		sdk.MustAccAddressFromBech32("inj1sh4fudervgpenvl386p9aqtpttwutwx6mnf603"),
		sdk.MustAccAddressFromBech32("inj16nej25lacmccwq58eydsu0399t584nqa8h7zev"),
		sdk.MustAccAddressFromBech32("inj15w473553s650wmr0qqd3xt6n82ld2pn4ax746m"),
		sdk.MustAccAddressFromBech32("inj1cn5y3rn352wqfhgaclzk63czacjfmqf7sqkhaq"),
		sdk.MustAccAddressFromBech32("inj1rpj9lm8d90xz4xh6nhdvzafknlsvmrvtr2fx5d"),
		sdk.MustAccAddressFromBech32("inj1xvg20f06xt2hs8qxjw3jdksx23vzdgx3le0csg"),
		sdk.MustAccAddressFromBech32("inj1jnqdr3khtjyvm6qk5gaxg5afkv2thawq8hflhe"),
		sdk.MustAccAddressFromBech32("inj1yp0jenaglwwh4c94f2pur559g5yq3sxdxsl4yh"),
		sdk.MustAccAddressFromBech32("inj1yg7n2wykhch38vt2a2xf98tqyn6z9ax0yz42sw"),
		sdk.MustAccAddressFromBech32("inj12p4zm9qhnufmdpmjeqa3fjtx3sqj6ecjd4cgm0"),
		sdk.MustAccAddressFromBech32("inj1plq4zy35rkcpdr65tp7urkdn3ff06x5xha6e2w"),
		sdk.MustAccAddressFromBech32("inj1rey99nt8q0ua07a2ny44j96gajmr8dqetfuta6"),
		sdk.MustAccAddressFromBech32("inj1whpvrnrdkggqq9rhjjy7647ut86szguugnhfae"),
		sdk.MustAccAddressFromBech32("inj1w9g5hp8nkzqnwgnamlltm6tltjg5qrgcl6p2hq"),
		sdk.MustAccAddressFromBech32("inj1p4t4h4jnh2u8ccm8d0s4ss9qh6hzdech9pgl6v"),
		sdk.MustAccAddressFromBech32("inj1ydwslfshnsafcrwyw7xm3eaq655gs85udmx82r"),
		sdk.MustAccAddressFromBech32("inj1k78hxv5wtgyf28z3tga9snuns0xjnzfkh4wgy6"),
		sdk.MustAccAddressFromBech32("inj1tk0vzwrnar230g50uq6wwqnvhp06gs8crmawtq"),
		sdk.MustAccAddressFromBech32("inj1ssaaq4tchc4dcnupfmdygu484ugvj8razeehdg"),
		sdk.MustAccAddressFromBech32("inj1t0uc4z2hmklvr3yeawqp9sljx6xvrqm82d6n4m"),
		sdk.MustAccAddressFromBech32("inj13m737jp5w4w3xvxf5gl0ellyx8y94he4kqusu6"),
		sdk.MustAccAddressFromBech32("inj1gv2ndycf60tke58590y05t49tpz6lwke5vx6gj"),
	}

	// USDT token
	tokenAmount := getOneCoin(ctx, bk, "peggy0x87aB3B4C8661e07D6372361211B96ed4Dc36B1B5")
	if tokenAmount.IsPositive() {
		tokenAmount.Amount = tokenAmount.Amount.MulRaw(amountNatural)
		if err := bankMintTo(
			ctx,
			bk,
			tokenAmount,
			qaAddresses,
		); err != nil {
			return err
		}
	}

	// INJ native coin
	tokenAmount = sdk.NewCoin("inj", math.NewIntWithDecimal(1, 18))
	tokenAmount.Amount = tokenAmount.Amount.MulRaw(amountNatural)
	return bankMintTo(
		ctx,
		bk,
		tokenAmount,
		qaAddresses,
	)
}

// getOneCoin queries decimals to get a proper sdk.Coin value representing 1 coin, i.e. for 1 INJ = 1e18 inj
func getOneCoin(ctx sdk.Context, bk bankkeeper.Keeper, denom string) sdk.Coin {
	meta, ok := bk.GetDenomMetaData(ctx, denom)
	if !ok {
		// return 0
		return sdk.NewCoin(denom, math.ZeroInt())
	}

	var exponent int
	for _, unit := range meta.DenomUnits {
		if unit.Exponent > 1 {
			exponent = int(unit.Exponent)
			break
		}
	}

	if exponent < 2 {
		// default to 18
		exponent = 18
	}

	// result value is n*10^dec
	return sdk.NewCoin(denom, math.NewIntWithDecimal(1, exponent))
}

func bankMintTo(
	ctx sdk.Context,
	bk bankkeeper.Keeper,
	amount sdk.Coin,
	mintTo []sdk.AccAddress,
) error {
	// premultiply amount by the number of recipients
	mintAmount := sdk.NewCoin(amount.Denom, amount.Amount.MulRaw(int64(len(mintTo))))
	err := bk.MintCoins(ctx, tokenfactorytypes.ModuleName, sdk.NewCoins(mintAmount))
	if err != nil {
		return errors.Wrap(err, "failed to mint coins")
	}

	for _, recipient := range mintTo {
		if err := bk.SendCoinsFromModuleToAccount(
			ctx,
			tokenfactorytypes.ModuleName,
			recipient,
			sdk.NewCoins(amount),
		); err != nil {
			return errors.Wrapf(err, "failed to send coins from module to account %s", recipient.String())
		}
	}

	return nil
}

func MigrateMintAmountERC20(ctx sdk.Context, app upgrades.InjectiveApplication, logger log.Logger) error {
	k := app.GetPeggyKeeper()
	bankKeeper := app.GetBankKeeper()

	// Get all rate limits
	rateLimits := k.GetRateLimits(ctx)
	for _, rl := range rateLimits {
		tokenAddr := common.HexToAddress(rl.TokenAddress)
		isCosmosOriginated, denom := k.ERC20ToDenomLookup(ctx, tokenAddr)
		if !isCosmosOriginated {
			// Get current supply
			supply := bankKeeper.GetSupply(ctx, denom)

			// Set MintAmountERC20 (this uses the NEW logic which is sdk.Int-based)
			k.SetMintAmountERC20(ctx, tokenAddr, supply.Amount)

			logger.Info("Migrated MintAmountERC20", "token", rl.TokenAddress, "amount", supply.Amount.String())
		} else {
			// For Cosmos-originated tokens, MintAmountERC20 logic is unused (skipped in both deposits and withdrawals).
			// We delete any existing value to clean up the state.
			k.DeleteMintAmountERC20(ctx, tokenAddr)
			logger.Info("Deleted MintAmountERC20 for Cosmos-originated token", "token", rl.TokenAddress)
		}
	}
	return nil
}
