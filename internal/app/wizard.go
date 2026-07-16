package app

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type configWizard struct {
	input  io.Reader
	reader *bufio.Reader
	out    io.Writer
	path   string
}

func configureConfig(input io.Reader, output io.Writer, path string, current Config) (Config, error) {
	wizard := configWizard{input: input, reader: bufio.NewReader(input), out: output, path: path}
	fmt.Fprintln(output, "\n余薪 Yuxin 配置")
	fmt.Fprintln(output, "只修改需要的部分；输入 0 完成。")
	fmt.Fprintln(output, "数据只保存在本机。")
	for {
		wizard.summary(current)
		choice, err := wizard.ask("选择要修改的部分", "0")
		if err != nil {
			return current, err
		}
		if choice == "" || choice == "0" {
			return current, nil
		}
		updated := current
		switch choice {
		case "1":
			updated, err = wizard.editSalary(current)
		case "2":
			updated, err = wizard.editSchedule(current)
		case "3":
			updated, err = wizard.editRetirement(current)
		case "4":
			updated, err = wizard.editAssets(current)
		case "5":
			updated, err = wizard.editMore(current)
		default:
			fmt.Fprintln(output, "  请输入 0 到 5。")
			continue
		}
		if err != nil {
			return current, err
		}
		if reflect.DeepEqual(updated, current) {
			fmt.Fprintln(output, "  未修改。")
			continue
		}
		if err := validateConfig(updated); err != nil {
			fmt.Fprintf(output, "  未保存：%v\n", err)
			continue
		}
		if err := saveConfig(updated, path); err != nil {
			return current, err
		}
		current = updated
		fmt.Fprintf(output, "  已保存：%s\n", path)
	}
}

func (wizard configWizard) summary(config Config) {
	mode := map[string]string{"monthly": "月薪", "daily": "日薪", "hourly": "时薪"}[config.SalaryMode]
	retirement := "关闭"
	if config.ProfileEnabled {
		if config.HideRetirementDate {
			retirement = "已开启（隐私隐藏）"
		} else {
			sex := map[string]string{"male": "男", "female": "女"}[config.Sex]
			retirement = fmt.Sprintf("%d 岁，%s", ageOnDate(config.BirthDate, configDateOnly(time.Now())), sex)
		}
	} else if config.RetirementYears > 0 {
		retirement = fmt.Sprintf("约 %d 年（旧配置）", config.RetirementYears)
	}
	fmt.Fprintln(wizard.out, "\n当前配置")
	fmt.Fprintf(wizard.out, "  1 今日入账  %s %s\n", mode, displayMoney(config.SalaryAmount, config.HideAmounts))
	fmt.Fprintf(wizard.out, "  2 工作时间  %s–%s，周 %s\n", clock(config.StartSecond), clock(config.EndSecond), workdayText(config.Workdays))
	fmt.Fprintf(wizard.out, "  3 退休倒计时 %s\n", retirement)
	assets := "关闭"
	if config.AssetsEnabled {
		assets = displayMoney(config.Assets, config.HideAmounts)
		if config.TargetMonthlySpend > 0 {
			assets += " · 目标 " + displayMoney(config.TargetMonthlySpend, config.HideAmounts) + "/月"
		}
	}
	fmt.Fprintf(wizard.out, "  4 存款       %s\n", assets)
	fmt.Fprintf(wizard.out, "  5 更多设置   刷新 %s\n", formatInterval(config.RefreshInterval))
}

func (wizard configWizard) editSalary(config Config) (Config, error) {
	defaultMode := map[string]string{"monthly": "1", "daily": "2", "hourly": "3"}[config.SalaryMode]
	choice, err := wizard.choice("薪资模式：1 月薪 / 2 日薪 / 3 时薪", defaultMode, "1", "2", "3")
	if err != nil {
		return config, err
	}
	config.SalaryMode = map[string]string{"1": "monthly", "2": "daily", "3": "hourly"}[choice]
	label := map[string]string{"monthly": "月薪", "daily": "日薪", "hourly": "时薪"}[config.SalaryMode]
	config.SalaryAmount, err = wizard.amount(label, config.SalaryAmount, false)
	if err != nil {
		return config, err
	}
	if config.SalaryMode == "monthly" {
		config.MonthlyWorkdays, err = wizard.amount("每月工作天数", config.MonthlyWorkdays, false)
	}
	return config, err
}

func (wizard configWizard) editSchedule(config Config) (Config, error) {
	for {
		value, err := wizard.ask("工作日（1=周一，逗号分隔）", workdayText(config.Workdays))
		if err != nil {
			return config, err
		}
		workdays, err := parseWorkdays("[" + value + "]")
		if err == nil && len(workdays) > 0 {
			config.Workdays = workdays
			break
		}
		fmt.Fprintln(wizard.out, "  工作日格式不正确，请输入 1 到 7。")
	}
	var err error
	config.StartSecond, err = wizard.clockValue("上班时间", config.StartSecond)
	if err != nil {
		return config, err
	}
	config.EndSecond, err = wizard.clockValue("下班时间", config.EndSecond)
	if err != nil {
		return config, err
	}
	currentLunch := 0
	if config.LunchEnabled {
		currentLunch = max(0, (config.LunchEnd-config.LunchStart)/60)
	}
	for {
		value, err := wizard.ask("午休时长（分钟，0 不扣除）", strconv.Itoa(currentLunch))
		if err != nil {
			return config, err
		}
		minutes, err := strconv.Atoi(value)
		workMinutes := (config.EndSecond - config.StartSecond) / 60
		if err != nil || minutes < 0 || minutes >= workMinutes {
			fmt.Fprintf(wizard.out, "  午休时长必须在 0 到 %d 分钟之间。\n", max(0, workMinutes-1))
			continue
		}
		config.LunchEnabled = minutes > 0
		if minutes > 0 {
			duration := minutes * 60
			start := 12 * 3600
			if start < config.StartSecond || start+duration > config.EndSecond {
				start = config.StartSecond + (config.EndSecond-config.StartSecond-duration)/2
			}
			config.LunchStart = start
			config.LunchEnd = start + duration
		}
		return config, nil
	}
}

func (wizard configWizard) editRefresh(config Config) (Config, error) {
	for {
		value, err := wizard.ask("刷新间隔（秒）", strconv.FormatFloat(config.RefreshInterval.Seconds(), 'f', -1, 64))
		if err != nil {
			return config, err
		}
		seconds, err := strconv.ParseFloat(value, 64)
		if err == nil && seconds >= 0.1 && seconds <= 3600 {
			config.RefreshInterval = time.Duration(seconds * float64(time.Second))
			return config, nil
		}
		fmt.Fprintln(wizard.out, "  刷新间隔必须在 0.1 到 3600 秒之间。")
	}
}

func (wizard configWizard) editMore(config Config) (Config, error) {
	choice, err := wizard.choice("更多设置：1 刷新间隔 / 2 自定义口号", "1", "1", "2")
	if err != nil {
		return config, err
	}
	if choice == "1" {
		return wizard.editRefresh(config)
	}
	value, err := wizard.ask("口号", config.Slogan)
	if err != nil {
		return config, err
	}
	config.Slogan = value
	return config, nil
}

func (wizard configWizard) editRetirement(config Config) (Config, error) {
	defaultBirth := "30"
	if config.ProfileEnabled && !config.BirthDate.IsZero() {
		defaultBirth = config.BirthDate.Format("2006-01-02")
	}
	for {
		value, err := wizard.ask("年龄或出生年月（0 关闭，如 30 / 1995-06）", defaultBirth)
		if err != nil {
			return config, err
		}
		if value == "0" {
			today := configDateOnly(time.Now())
			config.ProfileEnabled = false
			config.BirthDate = time.Time{}
			config.Sex = ""
			config.FemaleTrack = ""
			config.RetirementYears = 0
			config.RetirementStart = today
			config.ProgressBirthDate = today
			return config, nil
		}
		today := configDateOnly(time.Now())
		birth, err := parseAgeOrBirth(value, today)
		if err != nil {
			fmt.Fprintf(wizard.out, "  %v\n", err)
			continue
		}
		defaultSex := "1"
		if config.ProfileEnabled && config.Sex == "female" {
			defaultSex = "2"
		}
		choice, err := wizard.choice("性别：1 男 / 2 女", defaultSex, "1", "2")
		if err != nil {
			return config, err
		}
		config.ProfileEnabled = true
		config.BirthDate = birth
		config.ProgressBirthDate = today
		config.RetirementStart = today
		config.Sex = map[string]string{"1": "male", "2": "female"}[choice]
		config.FemaleTrack = ""
		config.RetirementYears = 0
		config.RetirementMode = "full"
		return config, nil
	}
}

func parseAgeOrBirth(value string, today time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if age, err := strconv.Atoi(value); err == nil {
		if age < 1 || age > 100 {
			return time.Time{}, fmt.Errorf("年龄必须在 1 到 100 岁之间。")
		}
		return today.AddDate(-age, 0, 0), nil
	}

	normalized := strings.NewReplacer("年", "-", "月", "-", "日", "", "/", "-", ".", "-").Replace(value)
	normalized = strings.TrimSuffix(normalized, "-")
	for _, layout := range []string{"2006-1-2", "2006-1"} {
		birth, err := time.ParseInLocation(layout, normalized, today.Location())
		if err != nil {
			continue
		}
		if birth.After(today) || ageOnDate(birth, today) > 100 {
			return time.Time{}, fmt.Errorf("出生日期必须在过去 100 年内。")
		}
		return birth, nil
	}
	return time.Time{}, fmt.Errorf("请输入年龄或出生年月，如 30、1995-06 或 1995-06-18。")
}

func ageOnDate(birth, today time.Time) int {
	age := today.Year() - birth.Year()
	if today.Month() < birth.Month() || today.Month() == birth.Month() && today.Day() < birth.Day() {
		age--
	}
	return age
}

func cyclePrivacy(config Config) Config {
	switch {
	case config.HideRetirementDate:
		config.HideAmounts = false
		config.HideRetirementDate = false
	case config.HideAmounts:
		config.HideRetirementDate = true
	default:
		config.HideAmounts = true
	}
	return config
}

func (wizard configWizard) editAssets(config Config) (Config, error) {
	amount, err := wizard.amount("存款金额（0 关闭）", config.Assets, true)
	if err != nil {
		return config, err
	}
	config.Reserve = 0
	config.Assets = amount
	config.AssetsEnabled = amount > 0
	if amount == 0 {
		config.AssetItems = nil
		config.TargetMonthlySpend = 0
	} else {
		config.AssetItems = []AssetItem{{Name: "存款", Kind: "deposit", Balance: amount}}
		config.TargetMonthlySpend, err = wizard.amount("目标每月可花（0 关闭）", config.TargetMonthlySpend, true)
		if err != nil {
			return config, err
		}
	}
	return config, nil
}

func (wizard configWizard) amount(label string, current float64, allowZero bool) (float64, error) {
	for {
		value, err := wizard.ask(label, configNumber(current))
		if err != nil {
			return current, err
		}
		amount, err := parseAmount(value)
		if err == nil && (amount > 0 || allowZero && amount == 0) {
			return amount, nil
		}
		if allowZero {
			fmt.Fprintln(wizard.out, "  请输入大于或等于 0 的金额，可使用 20w、20万或 200k。")
		} else {
			fmt.Fprintln(wizard.out, "  请输入大于 0 的金额，可使用 20w、20万或 200k。")
		}
	}
}

func (wizard configWizard) ask(label, defaultValue string) (string, error) {
	if defaultValue == "" {
		fmt.Fprintf(wizard.out, "%s: ", label)
	} else {
		fmt.Fprintf(wizard.out, "%s [%s]: ", label, defaultValue)
	}
	line, err := wizard.reader.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = defaultValue
	}
	return value, nil
}

func (wizard configWizard) choice(label, defaultValue string, allowed ...string) (string, error) {
	valid := make(map[string]bool, len(allowed))
	for _, value := range allowed {
		valid[value] = true
	}
	for {
		value, err := wizard.ask(label, defaultValue)
		if err != nil {
			return "", err
		}
		if valid[value] {
			return value, nil
		}
		fmt.Fprintf(wizard.out, "  请输入 %s。\n", strings.Join(allowed, "、"))
	}
}

func (wizard configWizard) clockValue(label string, current int) (int, error) {
	for {
		value, err := wizard.ask(label, clock(current))
		if err != nil {
			return current, err
		}
		parsed, err := parseClock(value)
		if err == nil {
			return parsed, nil
		}
		fmt.Fprintf(wizard.out, "  %s必须使用 HH:MM 格式。\n", label)
	}
}

func workdayText(workdays map[time.Weekday]bool) string {
	days := make([]string, 0, len(workdays))
	for day := 1; day <= 7; day++ {
		if workdays[time.Weekday(day%7)] {
			days = append(days, strconv.Itoa(day))
		}
	}
	return strings.Join(days, ",")
}
