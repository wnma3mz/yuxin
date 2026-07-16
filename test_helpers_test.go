package main

func testFullConfig() Config {
	config := defaultConfig()
	config.RetirementYears = 30
	config.ProfileEnabled = true
	config.AssetsEnabled = true
	config.Assets = 100000
	config.AssetItems = []AssetItem{{Name: "当前余额", Kind: "checking", Balance: 100000}}
	return config
}
