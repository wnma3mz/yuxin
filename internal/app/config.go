package app

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed data/default-config.toml
var bundledDefaultConfig []byte

const (
	maxConfigFileSize  = 1 << 20
	maxAssetItems      = 100
	maxAssetNameRunes  = 80
	maxSloganRunes     = 40
	maxMoneyAmount     = 1_000_000_000_000
	maxMonthlyWorkdays = 31
	defaultSlogan      = "摸鱼有数，下班有期。"
)

type Config struct {
	RefreshInterval    time.Duration
	Slogan             string
	RetirementYears    int
	RetirementStart    time.Time
	ProgressBirthDate  time.Time
	RetirementMode     string
	RetirementUnit     string
	SalaryMode         string
	SalaryAmount       float64
	MonthlyWorkdays    float64
	Workdays           map[time.Weekday]bool
	StartSecond        int
	EndSecond          int
	LunchEnabled       bool
	LunchStart         int
	LunchEnd           int
	ProfileEnabled     bool
	BirthDate          time.Time
	Sex                string
	FemaleTrack        string
	AssetsEnabled      bool
	Assets             float64
	BalanceStartDate   time.Time
	AssetItems         []AssetItem
	Reserve            float64
	TargetMonthlySpend float64
	HideAmounts        bool
	HideRetirementDate bool
	balanceDateMissing bool
}

type AssetItem struct {
	Name    string
	Kind    string
	Balance float64
}

func defaultConfig() Config {
	today := configDateOnly(time.Now())
	birth := mustDate("1995-01-01")
	return Config{
		RefreshInterval:   time.Second,
		Slogan:            defaultSlogan,
		RetirementYears:   0,
		RetirementStart:   today,
		ProgressBirthDate: birth,
		RetirementMode:    "full",
		RetirementUnit:    "days",
		SalaryMode:        "monthly",
		SalaryAmount:      8000,
		MonthlyWorkdays:   22,
		Workdays: map[time.Weekday]bool{
			time.Monday: true, time.Tuesday: true, time.Wednesday: true,
			time.Thursday: true, time.Friday: true,
		},
		StartSecond:        9 * 3600,
		EndSecond:          18 * 3600,
		LunchEnabled:       true,
		LunchStart:         12 * 3600,
		LunchEnd:           13 * 3600,
		ProfileEnabled:     false,
		BirthDate:          birth,
		Sex:                "male",
		BalanceStartDate:   today,
		AssetsEnabled:      false,
		balanceDateMissing: true,
	}
}

func normalizedPrivacyConfig(config Config) Config {
	if config.HideRetirementDate {
		config.HideAmounts = true
	}
	return config
}

// loadConfig parses the TOML subset written by Yuxin. Keeping the parser in the
// standard library avoids pulling a third-party decoder into the executable.
func loadConfig(path string) (Config, error) {
	config := defaultConfig()
	if path == "" {
		return config, validateConfig(config)
	}

	info, err := os.Stat(path)
	if err != nil {
		return config, err
	}
	if !info.Mode().IsRegular() {
		return config, fmt.Errorf("配置路径不是普通文件")
	}
	if info.Size() > maxConfigFileSize {
		return config, fmt.Errorf("配置文件不能超过 %d KiB", maxConfigFileSize/1024)
	}
	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()
	limited := &io.LimitedReader{R: file, N: maxConfigFileSize + 1}

	section := ""
	sawAssets := false
	sawProfile := false
	sawProfileBirth := false
	sawProfileSex := false
	assetIndex := -1
	seenKeys := make(map[string]bool)
	scanner := bufio.NewScanner(limited)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[[") && strings.HasSuffix(line, "]]") {
			section = strings.TrimSpace(line[2 : len(line)-2])
			if section != "assets" {
				return config, fmt.Errorf("第 %d 行不支持表数组 %s", lineNumber, section)
			}
			if section == "assets" && !sawAssets {
				config.Assets = 0
				config.AssetItems = nil
				config.AssetsEnabled = true
				sawAssets = true
			}
			if section == "assets" {
				if len(config.AssetItems) >= maxAssetItems {
					return config, fmt.Errorf("资产账户不能超过 %d 个", maxAssetItems)
				}
				config.AssetItems = append(config.AssetItems, AssetItem{Name: "未命名账户", Kind: "other"})
				assetIndex = len(config.AssetItems) - 1
			}
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			if section != "salary" && section != "schedule" && section != "profile" && section != "privacy" {
				return config, fmt.Errorf("第 %d 行不支持配置表 %s", lineNumber, section)
			}
			if section == "profile" {
				if !sawProfile {
					config.BirthDate = time.Time{}
					config.Sex = ""
					config.FemaleTrack = ""
				}
				sawProfile = true
				config.ProfileEnabled = true
			}
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return config, fmt.Errorf("第 %d 行不是有效的 key = value", lineNumber)
		}
		key := strings.TrimSpace(parts[0])
		keyID := section + "." + key
		if section == "assets" {
			keyID = fmt.Sprintf("assets[%d].%s", assetIndex, key)
		}
		if seenKeys[keyID] {
			return config, fmt.Errorf("第 %d 行重复配置 %s", lineNumber, keyID)
		}
		seenKeys[keyID] = true
		if !supportedConfigKey(section, key) {
			return config, fmt.Errorf("第 %d 行不支持配置 %s.%s", lineNumber, section, key)
		}
		if section == "profile" {
			sawProfileBirth = sawProfileBirth || key == "birth_date"
			sawProfileSex = sawProfileSex || key == "sex"
		}
		value, err := scalarValue(parts[1])
		if err != nil {
			return config, fmt.Errorf("第 %d 行 %s.%s: %w", lineNumber, section, key, err)
		}
		if section == "assets" && assetIndex >= 0 {
			item := &config.AssetItems[assetIndex]
			switch key {
			case "name":
				item.Name = value
			case "kind":
				item.Kind = value
			case "balance":
				amount, err := parseAmount(value)
				if err != nil || amount < 0 {
					if err == nil {
						err = fmt.Errorf("资产余额不能小于 0")
					}
					return config, fmt.Errorf("第 %d 行 assets.balance: %w", lineNumber, err)
				}
				item.Balance = amount
				config.Assets += amount
			}
			continue
		}
		if err := applyConfigValue(&config, section, key, value); err != nil {
			return config, fmt.Errorf("第 %d 行 %s.%s: %w", lineNumber, section, key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return config, err
	}
	if limited.N <= 0 {
		return config, fmt.Errorf("配置文件不能超过 %d KiB", maxConfigFileSize/1024)
	}

	// Enable flags only control defaults. An explicitly supplied table/array takes
	// precedence over the corresponding flag.
	if !sawAssets && !config.AssetsEnabled {
		config.Assets = 0
		config.AssetItems = nil
	}
	if !sawProfile && !config.ProfileEnabled {
		config.FemaleTrack = ""
	}
	if sawProfile && (!sawProfileBirth || !sawProfileSex) {
		return config, fmt.Errorf("profile 必须提供 birth_date 和 sex")
	}
	config = normalizedPrivacyConfig(config)
	return config, validateConfig(config)
}

func createDefaultConfig(path string) error {
	return createDefaultConfigUsing(path, func(writer io.Writer) error {
		_, err := writer.Write(bundledDefaultConfig)
		return err
	})
}

func createDefaultConfigUsing(path string, write func(io.Writer) error) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			_ = os.Remove(path)
		}
		if err == nil {
			err = closeErr
		}
	}()
	if err := write(file); err != nil {
		return err
	}
	return nil
}

func saveConfig(config Config, path string) error {
	config = normalizedPrivacyConfig(config)
	if err := validateConfig(config); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	items := config.AssetItems
	if config.AssetsEnabled && len(items) == 0 {
		items = []AssetItem{{Name: "当前余额", Kind: "checking", Balance: config.Assets}}
	}
	workdays := make([]string, 0, len(config.Workdays))
	for day := 1; day <= 7; day++ {
		if config.Workdays[time.Weekday(day%7)] {
			workdays = append(workdays, strconv.Itoa(day))
		}
	}
	q := strconv.Quote
	lines := []string{
		"version = 1",
		"refresh_interval = " + strconv.FormatFloat(config.RefreshInterval.Seconds(), 'f', -1, 64),
		"slogan = " + q(config.Slogan),
		"retirement_years = " + strconv.Itoa(config.RetirementYears),
		"retirement_start_date = " + q(config.RetirementStart.Format("2006-01-02")),
		"progress_birth_date = " + q(config.ProgressBirthDate.Format("2006-01-02")),
		"retirement_mode = " + q(config.RetirementMode),
		"retirement_unit = " + q(config.RetirementUnit),
		"reserve = " + q(configNumber(config.Reserve)),
		"target_monthly_spend = " + q(configNumber(config.TargetMonthlySpend)),
		"balance_start_date = " + q(config.BalanceStartDate.Format("2006-01-02")),
		"assets_enabled = " + strconv.FormatBool(config.AssetsEnabled),
		"profile_enabled = " + strconv.FormatBool(config.ProfileEnabled),
		"",
		"[salary]",
		"mode = " + q(config.SalaryMode),
		"amount = " + q(configNumber(config.SalaryAmount)),
		"monthly_workdays = " + q(configNumber(config.MonthlyWorkdays)),
		"",
		"[schedule]",
		"workdays = [" + strings.Join(workdays, ", ") + "]",
		"start = " + q(clock(config.StartSecond)),
		"end = " + q(clock(config.EndSecond)),
		"lunch_enabled = " + strconv.FormatBool(config.LunchEnabled),
		"lunch_start = " + q(clock(config.LunchStart)),
		"lunch_end = " + q(clock(config.LunchEnd)),
		"",
		"[privacy]",
		"hide_amounts = " + strconv.FormatBool(config.HideAmounts),
		"hide_retirement_date = " + strconv.FormatBool(config.HideRetirementDate),
	}
	if config.ProfileEnabled {
		lines = append(lines, "", "[profile]",
			"birth_date = "+q(config.BirthDate.Format("2006-01-02")),
			"sex = "+q(config.Sex))
		if config.FemaleTrack == "50" {
			lines = append(lines, "female_track = "+q(config.FemaleTrack))
		}
	}
	if config.AssetsEnabled {
		for _, item := range items {
			name, kind := item.Name, item.Kind
			if name == "" {
				name = "未命名账户"
			}
			if kind == "" {
				kind = "other"
			}
			lines = append(lines, "", "[[assets]]",
				"name = "+q(name),
				"kind = "+q(kind),
				"balance = "+q(configNumber(item.Balance)))
		}
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".yuxin-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func configNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func supportedConfigKey(section, key string) bool {
	supported := map[string]bool{
		".version": true, ".refresh_interval": true, ".slogan": true, ".retirement_years": true,
		".retirement_start_date": true, ".progress_birth_date": true, ".reserve": true,
		".target_monthly_spend": true,
		".balance_start_date":   true,
		".retirement_mode":      true, ".retirement_unit": true,
		".assets_enabled": true, ".profile_enabled": true,
		"salary.mode": true, "salary.amount": true, "salary.monthly_workdays": true,
		"schedule.workdays": true, "schedule.start": true, "schedule.end": true,
		"schedule.lunch_enabled": true, "schedule.lunch_start": true, "schedule.lunch_end": true,
		"profile.birth_date": true, "profile.sex": true, "profile.female_track": true,
		"privacy.hide_amounts": true, "privacy.hide_retirement_date": true,
		"assets.name": true, "assets.kind": true, "assets.balance": true,
	}
	return supported[section+"."+key]
}

func applyConfigValue(config *Config, section, key, value string) error {
	switch section + "." + key {
	case ".version":
		version, err := strconv.Atoi(value)
		if err != nil || version != 1 {
			return fmt.Errorf("只支持 version = 1")
		}
	case ".refresh_interval":
		seconds, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("必须是数字")
		}
		config.RefreshInterval = time.Duration(seconds * float64(time.Second))
	case ".slogan":
		config.Slogan = value
	case ".retirement_years":
		years, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("必须是整数")
		}
		config.RetirementYears = years
	case ".retirement_start_date":
		parsed, err := parseDate(value)
		if err != nil {
			return err
		}
		config.RetirementStart = parsed
	case ".progress_birth_date":
		parsed, err := parseDate(value)
		if err != nil {
			return err
		}
		config.ProgressBirthDate = parsed
	case ".retirement_mode":
		config.RetirementMode = value
	case ".retirement_unit":
		config.RetirementUnit = value
	case ".reserve":
		amount, err := parseAmount(value)
		if err != nil {
			return err
		}
		config.Reserve = amount
	case ".target_monthly_spend":
		amount, err := parseAmount(value)
		if err != nil {
			return err
		}
		config.TargetMonthlySpend = amount
	case ".balance_start_date":
		parsed, err := parseDate(value)
		if err != nil {
			return err
		}
		config.BalanceStartDate = parsed
		config.balanceDateMissing = false
	case ".assets_enabled":
		enabled, err := parseBool(value)
		if err != nil {
			return err
		}
		config.AssetsEnabled = enabled
	case ".profile_enabled":
		enabled, err := parseBool(value)
		if err != nil {
			return err
		}
		config.ProfileEnabled = enabled
	case "salary.mode":
		config.SalaryMode = value
	case "salary.amount":
		amount, err := parseAmount(value)
		if err != nil {
			return err
		}
		config.SalaryAmount = amount
	case "salary.monthly_workdays":
		amount, err := parseAmount(value)
		if err != nil {
			return err
		}
		config.MonthlyWorkdays = amount
	case "schedule.workdays":
		workdays, err := parseWorkdays(value)
		if err != nil {
			return err
		}
		config.Workdays = workdays
	case "schedule.start":
		seconds, err := parseClock(value)
		if err != nil {
			return err
		}
		config.StartSecond = seconds
	case "schedule.end":
		seconds, err := parseClock(value)
		if err != nil {
			return err
		}
		config.EndSecond = seconds
	case "schedule.lunch_enabled":
		enabled, err := parseBool(value)
		if err != nil {
			return err
		}
		config.LunchEnabled = enabled
	case "schedule.lunch_start":
		seconds, err := parseClock(value)
		if err != nil {
			return err
		}
		config.LunchStart = seconds
	case "schedule.lunch_end":
		seconds, err := parseClock(value)
		if err != nil {
			return err
		}
		config.LunchEnd = seconds
	case "profile.birth_date":
		birth, err := parseDate(value)
		if err != nil {
			return err
		}
		config.BirthDate = birth
	case "profile.sex":
		config.Sex = value
	case "profile.female_track":
		config.FemaleTrack = value
	case "privacy.hide_amounts":
		enabled, err := parseBool(value)
		if err != nil {
			return err
		}
		config.HideAmounts = enabled
	case "privacy.hide_retirement_date":
		enabled, err := parseBool(value)
		if err != nil {
			return err
		}
		config.HideRetirementDate = enabled
	case "assets.balance":
		amount, err := parseAmount(value)
		if err != nil {
			return err
		}
		if amount < 0 {
			return fmt.Errorf("资产余额不能小于 0")
		}
		config.Assets += amount
	}
	return nil
}

func parseAmount(value string) (float64, error) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), ",", ""))
	multiplier := 1.0
	for _, suffix := range []struct {
		text  string
		scale float64
	}{
		{"万", 10000},
		{"w", 10000},
		{"k", 1000},
	} {
		if strings.HasSuffix(value, suffix.text) {
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix.text))
			multiplier = suffix.scale
			break
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("必须是数字")
	}
	result := number * multiplier
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return 0, fmt.Errorf("必须是有限数字")
	}
	return result, nil
}

func parseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("必须使用 HH:MM 格式")
	}
	return parsed.Hour()*3600 + parsed.Minute()*60, nil
}

func parseDate(value string) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("必须使用 YYYY-MM-DD 格式")
	}
	return parsed, nil
}

func parseWorkdays(value string) (map[time.Weekday]bool, error) {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value[0] != '[' || value[len(value)-1] != ']' {
		return nil, fmt.Errorf("必须是整数数组")
	}
	result := make(map[time.Weekday]bool)
	content := strings.TrimSpace(value[1 : len(value)-1])
	if content == "" {
		return result, nil
	}
	for _, item := range strings.Split(content, ",") {
		day, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil {
			return nil, fmt.Errorf("必须是整数数组")
		}
		if day < 1 || day > 7 {
			return nil, fmt.Errorf("工作日必须在 1 到 7 之间")
		}
		result[time.Weekday(day%7)] = true
	}
	return result, nil
}

func parseBool(value string) (bool, error) {
	if value != "true" && value != "false" {
		return false, fmt.Errorf("必须是布尔值")
	}
	return value == "true", nil
}

func scalarValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("缺少值")
	}
	if value[0] == '"' {
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("字符串格式错误")
		}
		return unquoted, nil
	}
	return value, nil
}

func stripComment(line string) string {
	inString := false
	escaped := false
	for index, character := range line {
		if escaped {
			escaped = false
			continue
		}
		if character == '\\' && inString {
			escaped = true
			continue
		}
		if character == '"' {
			inString = !inString
			continue
		}
		if character == '#' && !inString {
			return line[:index]
		}
	}
	return line
}

func mustDate(value string) time.Time {
	parsed, err := parseDate(value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func configDateOnly(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}
