package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
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
	fmt.Fprintln(output, "只修改需要的部分；输入 0 即可返回仪表盘。")
	fmt.Fprintln(output, "数据只写入本机；身份证原号码不会保存。")
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
			updated, err = wizard.editRefresh(current)
		case "4":
			updated, err = wizard.editRetirement(current)
		case "5":
			updated, err = wizard.editAssets(current)
		case "6":
			updated, err = wizard.editPrivacy(current)
		default:
			fmt.Fprintln(output, "  请输入 0 到 6。")
			continue
		}
		if err != nil {
			return current, err
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
			retirement = "已配置（日期隐藏）"
		} else {
			retirement = config.BirthDate.Format("2006-01-02") + "，" + map[bool]string{true: "男", false: "女"}[config.Sex == "male"]
		}
	} else if config.RetirementYears > 0 {
		retirement = fmt.Sprintf("默认 %d 年", config.RetirementYears)
	}
	fmt.Fprintln(wizard.out, "\n当前配置")
	fmt.Fprintf(wizard.out, "  1 薪资      %s %s\n", mode, displayMoney(config.SalaryAmount, config.HideAmounts))
	fmt.Fprintf(wizard.out, "  2 工作时间  %s–%s，周 %s\n", clock(config.StartSecond), clock(config.EndSecond), workdayText(config.Workdays))
	fmt.Fprintf(wizard.out, "  3 刷新      %s\n", formatInterval(config.RefreshInterval))
	fmt.Fprintf(wizard.out, "  4 退休      %s\n", retirement)
	fmt.Fprintf(wizard.out, "  5 资产      %d 个账户，合计 %s\n", len(config.AssetItems), displayMoney(config.Assets, config.HideAmounts))
	privacy := "显示全部"
	if config.HideAmounts && config.HideRetirementDate {
		privacy = "隐藏金额和退休年月"
	} else if config.HideAmounts {
		privacy = "隐藏金额"
	} else if config.HideRetirementDate {
		privacy = "隐藏退休年月"
	}
	fmt.Fprintf(wizard.out, "  6 隐私显示  %s\n", privacy)
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
	config.LunchEnabled, err = wizard.yesNo("扣除午休", config.LunchEnabled)
	if err != nil || !config.LunchEnabled {
		return config, err
	}
	config.LunchStart, err = wizard.clockValue("午休开始", config.LunchStart)
	if err != nil {
		return config, err
	}
	config.LunchEnd, err = wizard.clockValue("午休结束", config.LunchEnd)
	if err != nil {
		return config, err
	}
	return config, nil
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

func (wizard configWizard) editRetirement(config Config) (Config, error) {
	fmt.Fprintln(wizard.out, "\n退休设置")
	fmt.Fprintln(wizard.out, "1. 输入身份证号码（自动解析出生日期和性别）")
	fmt.Fprintln(wizard.out, "2. 手动填写出生日期和性别")
	fmt.Fprintln(wizard.out, "3. 修改显示模式和单位")
	fmt.Fprintln(wizard.out, "4. 关闭退休模块")
	fmt.Fprintln(wizard.out, "0. 返回")
	choice, err := wizard.choice("选择", "0", "0", "1", "2", "3", "4")
	if err != nil || choice == "0" {
		return config, err
	}
	if choice == "4" {
		config.ProfileEnabled = false
		config.RetirementYears = 0
		return config, nil
	}
	if choice == "3" {
		if !config.ProfileEnabled && config.RetirementYears == 0 {
			fmt.Fprintln(wizard.out, "  请先启用退休模块。")
			return config, nil
		}
		return wizard.editRetirementDisplay(config)
	}
	var birth time.Time
	sex := ""
	if choice == "1" {
		for {
			value, err := wizard.secret("身份证号码（不会保存原号码）")
			if err != nil {
				return config, err
			}
			birth, sex, err = parseIdentityNumber(value)
			if err == nil {
				break
			}
			fmt.Fprintf(wizard.out, "  %v\n", err)
		}
	} else if choice == "2" {
		defaultBirth := config.BirthDate.Format("2006-01-02")
		if config.BirthDate.IsZero() {
			defaultBirth = "1995-01-01"
		}
		for {
			value, err := wizard.ask("出生日期 YYYY-MM-DD", defaultBirth)
			if err != nil {
				return config, err
			}
			birth, err = parseDate(value)
			if err == nil {
				break
			}
			fmt.Fprintln(wizard.out, "  日期格式不正确。")
		}
		defaultSex := "1"
		if config.Sex == "female" {
			defaultSex = "2"
		}
		value, err := wizard.choice("性别：1 男 / 2 女", defaultSex, "1", "2")
		if err != nil {
			return config, err
		}
		sex = "male"
		if value == "2" {
			sex = "female"
		}
	}
	config.ProfileEnabled = true
	config.RetirementYears = 30
	config.BirthDate = birth
	config.ProgressBirthDate = birth
	config.Sex = sex
	config.FemaleTrack = ""
	if sex == "female" {
		track, err := wizard.choice("女性人员类型：1 原 50 岁 / 2 原 55 岁", "2", "1", "2")
		if err != nil {
			return config, err
		}
		config.FemaleTrack = "55"
		if track == "1" {
			config.FemaleTrack = "50"
		}
	}
	fmt.Fprintf(wizard.out, "  已解析：%s，%s\n", birth.Format("2006-01-02"), map[bool]string{true: "男", false: "女"}[sex == "male"])
	return wizard.editRetirementDisplay(config)
}

func (wizard configWizard) editRetirementDisplay(config Config) (Config, error) {
	modeDefault := "1"
	if config.RetirementMode == "countdown" {
		modeDefault = "2"
	}
	mode, err := wizard.choice("显示模式：1 完整 / 2 轻量（只显示距离）", modeDefault, "1", "2")
	if err != nil {
		return config, err
	}
	config.RetirementMode = map[string]string{"1": "full", "2": "countdown"}[mode]
	unitDefault := map[string]string{"years": "1", "months": "2", "days": "3", "workdays": "4"}[config.RetirementUnit]
	unit, err := wizard.choice("距离单位：1 年 / 2 月 / 3 日 / 4 工作日", unitDefault, "1", "2", "3", "4")
	if err != nil {
		return config, err
	}
	config.RetirementUnit = map[string]string{"1": "years", "2": "months", "3": "days", "4": "workdays"}[unit]
	return config, nil
}

func (wizard configWizard) editPrivacy(config Config) (Config, error) {
	var err error
	config.HideAmounts, err = wizard.yesNo("在仪表盘隐藏全部金额", config.HideAmounts)
	if err != nil {
		return config, err
	}
	config.HideRetirementDate, err = wizard.yesNo("隐藏预计退休年月", config.HideRetirementDate)
	return config, err
}

func (wizard configWizard) editAssets(config Config) (Config, error) {
	items := append([]AssetItem(nil), config.AssetItems...)
	if config.AssetsEnabled && len(items) == 0 {
		items = []AssetItem{{Name: "当前余额", Kind: "checking", Balance: config.Assets}}
	}
	for {
		primaryBalance := 0.0
		if len(items) > 0 {
			primaryBalance = items[0].Balance
		}
		fmt.Fprintln(wizard.out, "\n资产设置")
		fmt.Fprintf(wizard.out, "当前余额：    %s\n", money(primaryBalance))
		fmt.Fprintf(wizard.out, "应急保留金：%s\n", money(config.Reserve))
		fmt.Fprintf(wizard.out, "其他账户：  %d 个\n", max(0, len(items)-1))
		fmt.Fprintln(wizard.out, "1. 修改当前余额（支持 20w / 200k）")
		fmt.Fprintln(wizard.out, "2. 修改应急保留金")
		fmt.Fprintln(wizard.out, "3. 管理账户明细")
		fmt.Fprintln(wizard.out, "4. 关闭资产模块")
		fmt.Fprintln(wizard.out, "0. 返回")
		choice, err := wizard.ask("选择", "0")
		if err != nil {
			return config, err
		}
		switch choice {
		case "0":
			config.AssetItems = items
			config.Assets = assetTotal(items)
			config.AssetsEnabled = len(items) > 0
			return config, nil
		case "1":
			balance, err := wizard.amount("当前余额", primaryBalance, true)
			if err != nil {
				return config, err
			}
			if len(items) == 0 {
				items = append(items, AssetItem{Name: "当前余额", Kind: "checking", Balance: balance})
			} else {
				items[0].Balance = balance
			}
		case "2":
			config.Reserve, err = wizard.amount("应急保留金", config.Reserve, true)
			if err != nil {
				return config, err
			}
		case "3":
			items, err = wizard.manageAccounts(items)
			if err != nil {
				return config, err
			}
		case "4":
			confirmed, err := wizard.yesNo("关闭会删除全部账户，确认继续", false)
			if err != nil {
				return config, err
			}
			if confirmed {
				items = nil
			}
		default:
			fmt.Fprintln(wizard.out, "  请输入 0 到 4。")
		}
	}
}

func (wizard configWizard) manageAccounts(items []AssetItem) ([]AssetItem, error) {
	for {
		fmt.Fprintln(wizard.out, "\n账户明细")
		for index, item := range items {
			fmt.Fprintf(wizard.out, "%d. %s  %s\n", index+1, item.Name, money(item.Balance))
		}
		if len(items) == 0 {
			fmt.Fprintln(wizard.out, "（尚未添加账户）")
		}
		choice, err := wizard.ask("a 添加 / e 编辑 / d 删除 / 0 返回", "0")
		if err != nil || choice == "0" {
			return items, err
		}
		if choice == "a" {
			item, err := wizard.readAccount(AssetItem{Name: fmt.Sprintf("账户 %d", len(items)+1), Kind: "checking"})
			if err != nil {
				return items, err
			}
			items = append(items, item)
			continue
		}
		if choice != "e" && choice != "d" {
			fmt.Fprintln(wizard.out, "  请输入 a、e、d 或 0。")
			continue
		}
		value, err := wizard.ask("账户序号", "1")
		if err != nil {
			return items, err
		}
		index, err := strconv.Atoi(value)
		index--
		if err != nil || index < 0 || index >= len(items) {
			fmt.Fprintln(wizard.out, "  账户序号不存在。")
			continue
		}
		if choice == "d" {
			items = append(items[:index], items[index+1:]...)
		} else {
			items[index], err = wizard.readAccount(items[index])
			if err != nil {
				return items, err
			}
		}
	}
}

func (wizard configWizard) readAccount(item AssetItem) (AssetItem, error) {
	name, err := wizard.ask("账户名称", item.Name)
	if err != nil {
		return item, err
	}
	item.Name = name
	defaultKind := map[string]string{"checking": "1", "deposit": "2", "other": "3"}[item.Kind]
	if defaultKind == "" {
		defaultKind = "3"
	}
	kind, err := wizard.choice("类型：1 银行卡活期 / 2 定期存款 / 3 其他", defaultKind, "1", "2", "3")
	if err != nil {
		return item, err
	}
	item.Kind = map[string]string{"1": "checking", "2": "deposit", "3": "other"}[kind]
	item.Balance, err = wizard.amount("余额", item.Balance, true)
	return item, err
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

func (wizard configWizard) secret(label string) (string, error) {
	fmt.Fprintf(wizard.out, "%s: ", label)
	restore := func() {}
	hidden := false
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, terminalSignals()...)
	if file, ok := wizard.input.(*os.File); ok {
		restore, hidden = prepareHiddenInput(file)
	}
	var restoreOnce sync.Once
	restoreTerminal := func() { restoreOnce.Do(restore) }
	done := make(chan struct{})
	if hidden {
		defer signal.Stop(interrupt)
		go func() {
			select {
			case <-interrupt:
				restoreTerminal()
				fmt.Fprintln(wizard.out)
				os.Exit(130)
			case <-done:
			}
		}()
	} else {
		signal.Stop(interrupt)
	}
	line, err := wizard.reader.ReadString('\n')
	close(done)
	restoreTerminal()
	if hidden {
		fmt.Fprintln(wizard.out)
	}
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), nil
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

func (wizard configWizard) yesNo(label string, current bool) (bool, error) {
	for {
		hint := "y/N"
		if current {
			hint = "Y/n"
		}
		fmt.Fprintf(wizard.out, "%s [%s]: ", label, hint)
		line, err := wizard.reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			return current, err
		}
		value := strings.ToLower(strings.TrimSpace(line))
		if value == "" {
			return current, nil
		}
		if value == "y" || value == "yes" || value == "是" {
			return true, nil
		}
		if value == "n" || value == "no" || value == "否" {
			return false, nil
		}
		fmt.Fprintln(wizard.out, "  请输入 y 或 n。")
	}
}

func parseIdentityNumber(value string) (time.Time, string, error) {
	number := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(value), " ", ""))
	birthText := ""
	sequence := byte('0')
	if len(number) == 18 {
		for index := 0; index < 17; index++ {
			if number[index] < '0' || number[index] > '9' {
				return time.Time{}, "", fmt.Errorf("18 位身份证号码格式不正确")
			}
		}
		if !strings.ContainsRune("0123456789X", rune(number[17])) {
			return time.Time{}, "", fmt.Errorf("18 位身份证号码格式不正确")
		}
		weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
		checks := "10X98765432"
		sum := 0
		for index := 0; index < 17; index++ {
			sum += int(number[index]-'0') * weights[index]
		}
		if number[17] != checks[sum%11] {
			return time.Time{}, "", fmt.Errorf("身份证校验码不正确")
		}
		birthText, sequence = number[6:14], number[16]
	} else if len(number) == 15 {
		for index := range number {
			if number[index] < '0' || number[index] > '9' {
				return time.Time{}, "", fmt.Errorf("身份证号码必须是 18 位或旧版 15 位")
			}
		}
		birthText, sequence = "19"+number[6:12], number[14]
	} else {
		return time.Time{}, "", fmt.Errorf("身份证号码必须是 18 位或旧版 15 位")
	}
	birth, err := time.ParseInLocation("20060102", birthText, time.Local)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("身份证中的出生日期不正确")
	}
	sex := "female"
	if (sequence-'0')%2 == 1 {
		sex = "male"
	}
	return birth, sex, nil
}

func assetTotal(items []AssetItem) float64 {
	total := 0.0
	for _, item := range items {
		total += item.Balance
	}
	return total
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
