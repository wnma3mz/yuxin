package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const anonymousShareTimeout = 20 * time.Second

var (
	publicAPIURL = "https://nubeymzysjmlwgzjpstl.supabase.co"
	publicAPIKey = "sb_publishable_XgQpwxr2v3hhi6YL2hR0GQ_v_7N4amE"
)

type anonymousContribution struct {
	MonthlySalaryCNY         int64   `json:"p_monthly_salary_cny"`
	DailyWorkMinutes         int     `json:"p_daily_work_minutes"`
	WorkdaysPerWeek          int     `json:"p_workdays_per_week"`
	SavingsCNY               *int64  `json:"p_savings_cny"`
	RetirementYearsRemaining *int    `json:"p_retirement_years_remaining"`
	MessageKind              *string `json:"-"`
	MessageText              *string `json:"-"`
}

type anonymousPreviewWriter struct {
	writer io.Writer
	err    error
}

func (writer *anonymousPreviewWriter) Write(value []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	written, err := writer.writer.Write(value)
	if err == nil && written != len(value) {
		err = io.ErrShortWrite
	}
	if err != nil {
		writer.err = err
	}
	return written, err
}

func (writer *anonymousPreviewWriter) result() error {
	if writer.err == nil {
		return nil
	}
	return fmt.Errorf("显示匿名贡献预览：%w", writer.err)
}

func runAnonymousShare(input io.Reader, output io.Writer, config Config, now time.Time) error {
	payload, confirmed, err := collectAnonymousContribution(input, output, config, now)
	if err != nil || !confirmed {
		return err
	}
	endpoint, key, err := anonymousAPIConfig()
	if err != nil {
		return err
	}
	client := &http.Client{
		Timeout: anonymousShareTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("公开数据服务不应重定向")
		},
	}
	if err := submitAnonymousContribution(context.Background(), client, endpoint, key, payload); err != nil {
		return err
	}
	fmt.Fprintln(output, "匿名贡献成功。数值将在隐私门槛满足后进入后续公开统计。")
	if payload.MessageText != nil {
		messageEndpoint := strings.TrimSuffix(endpoint, "submit_public_data") + "submit_public_message"
		if err := submitAnonymousMessage(context.Background(), client, messageEndpoint, key, *payload.MessageKind, *payload.MessageText); err != nil {
			fmt.Fprintln(output, "匿名回声未能送达；数值已成功提交，请勿重复提交数值。")
			return nil
		}
		fmt.Fprintln(output, "匿名回声已单独提交，将在审核通过后公开展示。")
	}
	return nil
}

func anonymousAPIConfig() (string, string, error) {
	endpoint := strings.TrimSpace(os.Getenv("YUXIN_PUBLIC_API_URL"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(publicAPIURL)
	}
	key := strings.TrimSpace(os.Getenv("YUXIN_PUBLIC_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(publicAPIKey)
	}
	if endpoint == "" || key == "" {
		return "", "", errors.New("公开数据服务尚未配置")
	}
	endpoint = strings.TrimRight(endpoint, "/") + "/rest/v1/rpc/submit_public_data"
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) {
		return "", "", errors.New("公开数据服务地址无效")
	}
	return endpoint, key, nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func collectAnonymousContribution(input io.Reader, output io.Writer, config Config, now time.Time) (anonymousContribution, bool, error) {
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	preview := &anonymousPreviewWriter{writer: output}
	output = preview
	payload := baseAnonymousContribution(config)
	if payload.MonthlySalaryCNY < 100 || payload.MonthlySalaryCNY > 10_000_000 {
		return payload, false, errors.New("折算月薪超出匿名看板支持的 100–10,000,000 元范围")
	}
	if payload.DailyWorkMinutes < 60 || payload.DailyWorkMinutes > 960 {
		return payload, false, errors.New("每日净工时超出匿名看板支持的 1–16 小时范围")
	}
	if payload.WorkdaysPerWeek < 1 || payload.WorkdaysPerWeek > 7 {
		return payload, false, errors.New("每周工作天数超出匿名看板支持的 1–7 天范围")
	}
	fmt.Fprintln(output, "余薪匿名数据贡献")
	fmt.Fprintln(output, "结构化数据不包含配置文件、姓名、单位、城市、年龄、生日、性别或设备标识。")
	fmt.Fprintln(output, "提示：确认上传后将连接 Supabase，其服务日志可能包含 IP、请求时间和 User-Agent。")
	if err := preview.result(); err != nil {
		return payload, false, err
	}

	if config.AssetsEnabled {
		include, err := readYesNo(reader, output, "是否包含当前存款？[y/N]: ")
		if err != nil {
			return payload, false, err
		}
		if include {
			salary := CalculateSalary(now, config)
			value := roundInt64(liveBalanceAt(config, now, salary.EarnedToday), 1000)
			if value < 0 || value > 1_000_000_000_000 {
				return payload, false, errors.New("实时存款超出匿名看板支持的 0–1,000,000,000,000 元范围")
			}
			payload.SavingsCNY = &value
		}
	}
	if config.ProfileEnabled || config.RetirementYears > 0 {
		include, err := readYesNo(reader, output, "是否包含距离退休年数？[y/N]: ")
		if err != nil {
			return payload, false, err
		}
		if include {
			var retirement RetirementSnapshot
			if config.ProfileEnabled {
				retirement, err = CalculateRetirement(config, now)
				if err != nil {
					return payload, false, err
				}
			} else {
				retirement = CalculateDefaultRetirement(config, now)
			}
			years := int(float64(retirement.RemainingDays) / averageDaysPerYear)
			payload.RetirementYearsRemaining = &years
		}
	}

	if _, err := io.WriteString(output, "匿名回声：0 跳过 / 1 建议 / 2 吐槽 / 3 许愿 / 4 打气 [0]: "); err != nil {
		return payload, false, preview.result()
	}
	choice, err := readLine(reader)
	if err != nil {
		return payload, false, err
	}
	choice = strings.TrimSpace(choice)
	if choice != "" && choice != "0" {
		kind := map[string]string{"1": "advice", "2": "rant", "3": "wish", "4": "encourage"}[choice]
		if kind == "" {
			return payload, false, errors.New("匿名回声类型必须是 0–4")
		}
		if _, err := io.WriteString(output, "一句话（最多 80 字；请勿填写姓名、单位、地址或联系方式；不支持链接）: "); err != nil {
			return payload, false, preview.result()
		}
		message, err := readLine(reader)
		if err != nil {
			return payload, false, err
		}
		message = strings.TrimSpace(message)
		if err := validateAnonymousMessage(message); err != nil {
			return payload, false, err
		}
		payload.MessageKind, payload.MessageText = &kind, &message
	}

	fmt.Fprintln(output, "\n即将匿名贡献：")
	fmt.Fprintf(output, "  月薪（折算）  %s\n", money(float64(payload.MonthlySalaryCNY)))
	fmt.Fprintf(output, "  每日净工时    %s\n", formatMinutes(payload.DailyWorkMinutes))
	fmt.Fprintf(output, "  每周工作天数  %d 天\n", payload.WorkdaysPerWeek)
	writeOptionalAnonymousPreview(output, payload)
	fmt.Fprintln(output, "\n我的位置（仅在本地按固定区间计算）：")
	fmt.Fprintf(output, "  月薪          %s\n", salaryInterval(payload.MonthlySalaryCNY))
	fmt.Fprintf(output, "  每日净工时    %s\n", workMinutesInterval(payload.DailyWorkMinutes))
	if payload.SavingsCNY != nil {
		fmt.Fprintf(output, "  当前存款      %s\n", savingsInterval(*payload.SavingsCNY))
	}
	if payload.RetirementYearsRemaining != nil {
		fmt.Fprintf(output, "  距离退休      %s\n", retirementInterval(*payload.RetirementYearsRemaining))
	}
	fmt.Fprintln(output, "原始记录不会公开，只会进入总体聚合统计。")
	fmt.Fprint(output, "输入 SHARE 确认上传：")
	if err := preview.result(); err != nil {
		return payload, false, err
	}
	confirmation, err := readLine(reader)
	if err != nil {
		return payload, false, err
	}
	if strings.TrimSpace(confirmation) != "SHARE" {
		fmt.Fprintln(output, "已取消匿名贡献。")
		return payload, false, preview.result()
	}
	return payload, true, nil
}

func baseAnonymousContribution(config Config) anonymousContribution {
	workdays := 0
	for _, enabled := range config.Workdays {
		if enabled {
			workdays++
		}
	}
	workMinutes := roundInt(effectiveWorkSeconds(config)/60, 30)
	monthly := config.SalaryAmount
	switch config.SalaryMode {
	case "daily":
		monthly = config.SalaryAmount * float64(workdays) * 52 / 12
	case "hourly":
		monthly = config.SalaryAmount * float64(workMinutes) / 60 * float64(workdays) * 52 / 12
	case "annual":
		monthly = config.SalaryAmount / 12
	}
	return anonymousContribution{
		MonthlySalaryCNY: roundInt64(monthly, 100),
		DailyWorkMinutes: workMinutes,
		WorkdaysPerWeek:  workdays,
	}
}

func roundInt64(value float64, unit int64) int64 {
	return int64(math.Round(value/float64(unit))) * unit
}

func roundInt(value, unit int) int {
	return int(math.Round(float64(value)/float64(unit))) * unit
}

func validateAnonymousMessage(value string) error {
	if value == "" {
		return errors.New("匿名回声不能为空")
	}
	if len([]rune(value)) > 80 {
		return errors.New("匿名回声不能超过 80 个字符")
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "www.") {
		return errors.New("匿名回声暂不支持链接")
	}
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("匿名回声只能填写单行文本")
	}
	return nil
}

func readYesNo(reader *bufio.Reader, output io.Writer, prompt string) (bool, error) {
	if _, err := io.WriteString(output, prompt); err != nil {
		return false, fmt.Errorf("显示匿名贡献预览：%w", err)
	}
	value, err := readLine(reader)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "n", "no":
		return false, nil
	case "y", "yes":
		return true, nil
	default:
		return false, errors.New("请输入 y 或 n")
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("读取输入：%w", err)
	}
	if errors.Is(err, io.EOF) && value == "" {
		return "", errors.New("输入已结束")
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func writeOptionalAnonymousPreview(output io.Writer, payload anonymousContribution) {
	if payload.SavingsCNY != nil {
		fmt.Fprintf(output, "  当前存款      %s\n", money(float64(*payload.SavingsCNY)))
	}
	if payload.RetirementYearsRemaining != nil {
		fmt.Fprintf(output, "  距离退休      %d 年\n", *payload.RetirementYearsRemaining)
	}
	if payload.MessageText != nil {
		fmt.Fprintf(output, "  匿名回声      %s\n", strconv.Quote(*payload.MessageText))
	}
}

func formatMinutes(minutes int) string {
	if minutes%60 == 0 {
		return fmt.Sprintf("%d 小时", minutes/60)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", minutes/60, minutes%60)
}

func salaryInterval(value int64) string {
	switch {
	case value < 3000:
		return "3千以下"
	case value < 5000:
		return "3–5千"
	case value < 8000:
		return "5–8千"
	case value < 12000:
		return "8千–1.2万"
	case value < 20000:
		return "1.2–2万"
	case value < 30000:
		return "2–3万"
	default:
		return "3万以上"
	}
}

func workMinutesInterval(value int) string {
	switch {
	case value < 360:
		return "6小时以下"
	case value < 480:
		return "6–8小时"
	case value < 600:
		return "8–10小时"
	case value < 720:
		return "10–12小时"
	default:
		return "12小时以上"
	}
}

func savingsInterval(value int64) string {
	switch {
	case value < 10000:
		return "1万以下"
	case value < 50000:
		return "1–5万"
	case value < 100000:
		return "5–10万"
	case value < 300000:
		return "10–30万"
	case value < 1000000:
		return "30–100万"
	default:
		return "100万以上"
	}
}

func retirementInterval(value int) string {
	switch {
	case value <= 10:
		return "10年以内"
	case value <= 20:
		return "11–20年"
	case value <= 30:
		return "21–30年"
	case value <= 40:
		return "31–40年"
	default:
		return "40年以上"
	}
}

func submitAnonymousContribution(ctx context.Context, client *http.Client, endpoint, key string, payload anonymousContribution) error {
	return postAnonymousPayload(ctx, client, endpoint, key, payload)
}

func submitAnonymousMessage(ctx context.Context, client *http.Client, endpoint, key, kind, message string) error {
	return postAnonymousPayload(ctx, client, endpoint, key, map[string]string{
		"p_message_kind": kind,
		"p_message_text": message,
	})
}

func postAnonymousPayload(ctx context.Context, client *http.Client, endpoint, key string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("编码匿名数据：%w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建匿名贡献请求：%w", err)
	}
	request.Header.Set("apikey", key)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Prefer", "return=minimal")
	request.Header.Set("User-Agent", "yuxin/"+version)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("连接公开数据服务：%w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("公开数据服务返回 HTTP %d：%s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}
