package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func validateConfig(config Config) error {
	return validateConfigAt(config, time.Now())
}

func validateConfigAt(config Config, now time.Time) error {
	today := configDateOnly(now)
	if config.RefreshInterval < 100*time.Millisecond || config.RefreshInterval > time.Hour {
		return fmt.Errorf("刷新间隔必须在 0.1 到 3600 秒之间")
	}
	if utf8.RuneCountInString(config.Slogan) > maxSloganRunes || strings.IndexFunc(config.Slogan, unicode.IsControl) >= 0 {
		return fmt.Errorf("口号不能超过 %d 个字且不能包含控制字符", maxSloganRunes)
	}
	if config.RetirementYears < 0 || config.RetirementYears > 100 {
		return fmt.Errorf("默认退休年数必须在 0 到 100 之间")
	}
	if config.RetirementMode != "full" && config.RetirementMode != "countdown" {
		return fmt.Errorf("退休显示模式必须是 full 或 countdown")
	}
	if config.RetirementUnit != "years" && config.RetirementUnit != "months" && config.RetirementUnit != "days" && config.RetirementUnit != "workdays" {
		return fmt.Errorf("退休单位必须是 years、months、days 或 workdays")
	}
	if config.ProgressBirthDate.After(today) {
		return fmt.Errorf("进度出生日期不能晚于今天")
	}
	if config.SalaryMode != "monthly" && config.SalaryMode != "daily" && config.SalaryMode != "hourly" && config.SalaryMode != "annual" {
		return fmt.Errorf("薪资模式必须是 monthly、daily、hourly 或 annual")
	}
	if config.SalaryAmount <= 0 || config.SalaryAmount > maxMoneyAmount {
		return fmt.Errorf("薪资金额必须大于 0 且不超过 %s", configNumber(maxMoneyAmount))
	}
	if config.MonthlyWorkdays <= 0 || config.MonthlyWorkdays > maxMonthlyWorkdays {
		return fmt.Errorf("每月工作天数必须在 0 到 %d 之间", maxMonthlyWorkdays)
	}
	if len(config.Workdays) == 0 {
		return fmt.Errorf("至少需要选择一个工作日")
	}
	if config.EndSecond <= config.StartSecond {
		return fmt.Errorf("下班时间必须晚于上班时间")
	}
	if config.LunchEnabled && !(config.StartSecond <= config.LunchStart && config.LunchStart < config.LunchEnd && config.LunchEnd <= config.EndSecond) {
		return fmt.Errorf("午休时间必须位于工作时间内")
	}
	if config.ProfileEnabled {
		if config.BirthDate.IsZero() {
			return fmt.Errorf("出生日期不能为空")
		}
		if config.BirthDate.After(today) {
			return fmt.Errorf("出生日期不能晚于今天")
		}
		if config.Sex != "male" && config.Sex != "female" {
			return fmt.Errorf("性别必须是 male 或 female")
		}
		if config.Sex != "female" && config.FemaleTrack != "" {
			return fmt.Errorf("只有女性自动估算可以配置 female_track")
		}
		if config.Sex == "female" && config.FemaleTrack != "" && config.FemaleTrack != "50" && config.FemaleTrack != "55" {
			return fmt.Errorf("女性 female_track 只能是 50 或 55")
		}
	}
	if config.Reserve < 0 || config.Reserve > maxMoneyAmount {
		return fmt.Errorf("保留金必须在 0 到 %s 之间", configNumber(maxMoneyAmount))
	}
	if config.TargetMonthlySpend < 0 || config.TargetMonthlySpend > maxMoneyAmount {
		return fmt.Errorf("目标每月可花必须在 0 到 %s 之间", configNumber(maxMoneyAmount))
	}
	if config.WishAmount < 0 || config.WishAmount > maxMoneyAmount {
		return fmt.Errorf("心愿金额必须在 0 到 %s 之间", configNumber(maxMoneyAmount))
	}
	if config.WishName != "" && (strings.TrimSpace(config.WishName) == "" || utf8.RuneCountInString(config.WishName) > maxSloganRunes || strings.IndexFunc(config.WishName, unicode.IsControl) >= 0) {
		return fmt.Errorf("心愿名称不能超过 %d 个字且不能包含控制字符", maxSloganRunes)
	}
	if (config.WishName == "") != (config.WishAmount == 0) {
		return fmt.Errorf("心愿名称和金额必须同时设置或同时关闭")
	}
	if config.TargetMonthlySpend > 0 && config.WishAmount > 0 {
		return fmt.Errorf("躺平目标和心愿目标只能开启一个")
	}
	if config.WishAmount > 0 && (config.WishStartDate.IsZero() || configDateOnly(config.WishStartDate).After(today)) {
		return fmt.Errorf("心愿起算日不能为空或晚于今天")
	}
	if config.Assets < 0 || config.Assets > maxMoneyAmount {
		return fmt.Errorf("资产余额必须在 0 到 %s 之间", configNumber(maxMoneyAmount))
	}
	if len(config.AssetItems) > maxAssetItems {
		return fmt.Errorf("资产账户不能超过 %d 个", maxAssetItems)
	}
	for _, item := range config.AssetItems {
		if item.Balance < 0 || item.Balance > maxMoneyAmount {
			return fmt.Errorf("资产余额必须在 0 到 %s 之间", configNumber(maxMoneyAmount))
		}
		if utf8.RuneCountInString(item.Name) > maxAssetNameRunes {
			return fmt.Errorf("账户名称不能超过 %d 个字符", maxAssetNameRunes)
		}
		if strings.IndexFunc(item.Name, unicode.IsControl) >= 0 {
			return fmt.Errorf("账户名称不能包含控制字符")
		}
	}
	return nil
}
