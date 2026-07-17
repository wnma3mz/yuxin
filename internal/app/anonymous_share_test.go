package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBaseAnonymousContributionRoundsAndConvertsLocally(t *testing.T) {
	config := defaultConfig()
	config.SalaryMode = "hourly"
	config.SalaryAmount = 50.4
	config.MonthlyWorkdays = 21.75
	config.StartSecond = 9 * 3600
	config.EndSecond = 18*3600 + 10*60
	config.LunchEnabled = true
	config.LunchStart = 12 * 3600
	config.LunchEnd = 13 * 3600

	payload := baseAnonymousContribution(config)
	if payload.MonthlySalaryCNY%100 != 0 || payload.DailyWorkMinutes%30 != 0 {
		t.Fatalf("payload was not coarsened: %+v", payload)
	}
	if payload.WorkdaysPerWeek != 5 {
		t.Fatalf("workdays = %d, want 5", payload.WorkdaysPerWeek)
	}
	if payload.MonthlySalaryCNY != 8700 {
		t.Fatalf("hourly monthly equivalent = %d, want 8700", payload.MonthlySalaryCNY)
	}
}

func TestBaseAnonymousContributionAnnualizesNonMonthlySalary(t *testing.T) {
	for _, test := range []struct {
		name   string
		mode   string
		amount float64
		want   int64
	}{
		{name: "monthly", mode: "monthly", amount: 8000, want: 8000},
		{name: "daily", mode: "daily", amount: 500, want: 10800},
		{name: "hourly", mode: "hourly", amount: 50, want: 8700},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := defaultConfig()
			config.SalaryMode = test.mode
			config.SalaryAmount = test.amount
			config.MonthlyWorkdays = 31
			if got := baseAnonymousContribution(config).MonthlySalaryCNY; got != test.want {
				t.Fatalf("monthly equivalent = %d, want %d", got, test.want)
			}
		})
	}
}

func TestCollectAnonymousContributionRejectsUnsupportedRoundedSalary(t *testing.T) {
	config := defaultConfig()
	config.SalaryAmount = 49
	_, _, err := collectAnonymousContribution(strings.NewReader(""), io.Discard, config, time.Now())
	if err == nil || !strings.Contains(err.Error(), "折算月薪超出") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectAnonymousContributionRejectsUnsupportedWorkSchedule(t *testing.T) {
	for _, minutes := range []int{30, 990} {
		config := defaultConfig()
		config.LunchEnabled = false
		config.StartSecond = 0
		config.EndSecond = minutes * 60
		_, _, err := collectAnonymousContribution(strings.NewReader(""), io.Discard, config, time.Now())
		if err == nil || !strings.Contains(err.Error(), "1–16 小时") {
			t.Fatalf("%d minute schedule error = %v", minutes, err)
		}
	}

	config := defaultConfig()
	config.Workdays = map[time.Weekday]bool{time.Monday: false}
	_, _, err := collectAnonymousContribution(strings.NewReader(""), io.Discard, config, time.Now())
	if err == nil || !strings.Contains(err.Error(), "1–7 天") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectAnonymousContributionAcceptsWorkScheduleBoundaries(t *testing.T) {
	for _, minutes := range []int{60, 960} {
		config := defaultConfig()
		config.LunchEnabled = false
		config.StartSecond = 0
		config.EndSecond = minutes * 60
		_, confirmed, err := collectAnonymousContribution(strings.NewReader("0\nCANCEL\n"), io.Discard, config, time.Now())
		if err != nil || confirmed {
			t.Fatalf("%d minute schedule = confirmed %t, error %v", minutes, confirmed, err)
		}
	}
}

func TestCollectAnonymousContributionUsesLiveBalance(t *testing.T) {
	config := defaultConfig()
	config.AssetsEnabled = true
	config.Assets = 10_000
	config.BalanceStartDate = mustDate("2026-07-13")
	payload, confirmed, err := collectAnonymousContribution(
		strings.NewReader("y\n0\nCANCEL\n"),
		io.Discard,
		config,
		time.Date(2026, 7, 17, 18, 0, 0, 0, time.Local),
	)
	if err != nil || confirmed {
		t.Fatalf("collect = confirmed %t, error %v", confirmed, err)
	}
	if payload.SavingsCNY == nil || *payload.SavingsCNY != 12_000 {
		t.Fatalf("live savings = %v, want 12000", payload.SavingsCNY)
	}
}

func TestCollectAnonymousContributionRejectsLiveBalanceAboveServerLimit(t *testing.T) {
	config := defaultConfig()
	config.AssetsEnabled = true
	config.Assets = 1_000_000_000_000
	config.BalanceStartDate = mustDate("2026-07-16")
	_, _, err := collectAnonymousContribution(
		strings.NewReader("y\n"),
		io.Discard,
		config,
		time.Date(2026, 7, 17, 18, 0, 0, 0, time.Local),
	)
	if err == nil || !strings.Contains(err.Error(), "实时存款超出") {
		t.Fatalf("error = %v", err)
	}
}

func TestCollectAnonymousContributionPreviewsOptionalDataAndLocalIntervals(t *testing.T) {
	config := defaultConfig()
	config.AssetsEnabled = true
	config.Assets = 12_345
	config.ProfileEnabled = true
	config.BirthDate = mustDate("1995-06-01")
	config.Sex = "male"
	input := strings.NewReader("y\ny\n3\n希望大家都能准点下班\nSHARE\n")
	var output strings.Builder

	payload, confirmed, err := collectAnonymousContribution(input, &output, config, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC))
	if err != nil || !confirmed {
		t.Fatalf("collect = confirmed %t, error %v", confirmed, err)
	}
	if payload.SavingsCNY == nil || *payload.SavingsCNY != 12_000 {
		t.Fatalf("rounded savings = %v", payload.SavingsCNY)
	}
	if payload.RetirementYearsRemaining == nil {
		t.Fatalf("profile fields = %+v", payload)
	}
	retirement, err := CalculateRetirement(config, time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	wantYears := int(float64(retirement.RemainingDays) / averageDaysPerYear)
	if *payload.RetirementYearsRemaining != wantYears {
		t.Fatalf("retirement years = %d, want %d", *payload.RetirementYearsRemaining, wantYears)
	}
	for _, want := range []string{"Supabase", "我的位置（仅在本地", "8千–1.2万", "输入 SHARE"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("preview missing %q:\n%s", want, output.String())
		}
	}
}

func TestRunAnonymousShareMakesNoRequestWithoutFinalConfirmation(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")

	if err := runAnonymousShare(strings.NewReader("0\nNO\n"), io.Discard, defaultConfig(), time.Now()); err != nil {
		t.Fatalf("cancelled share: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("cancelled share made %d requests", requests.Load())
	}
}

func TestRunAnonymousShareSendsMinimalRoundedPayloadAfterConfirmation(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/rest/v1/rpc/submit_public_data" {
			t.Errorf("path = %q", request.URL.Path)
		}
		if request.Header.Get("apikey") != "sb_publishable_test" || request.Header.Get("Authorization") != "Bearer sb_publishable_test" {
			t.Errorf("missing public API headers")
		}
		if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		output.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")
	var output strings.Builder

	if err := runAnonymousShare(strings.NewReader("0\nSHARE\n"), &output, defaultConfig(), time.Now()); err != nil {
		t.Fatalf("runAnonymousShare: %v", err)
	}
	for _, forbidden := range []string{"name", "email", "birth_date", "sex", "ip", "user_agent", "device_id", "config", "p_age_band", "p_target_monthly_spend_cny", "p_message_kind", "p_message_text"} {
		if _, exists := got[forbidden]; exists {
			t.Errorf("payload contains forbidden field %q", forbidden)
		}
	}
	if got["p_monthly_salary_cny"] != float64(8000) || got["p_daily_work_minutes"] != float64(480) {
		t.Fatalf("payload = %#v", got)
	}
	if !strings.Contains(output.String(), "匿名贡献成功") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestRunAnonymousShareSendsMessageInSeparateRequest(t *testing.T) {
	var paths []string
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		bodies = append(bodies, body)
		output.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")

	input := strings.NewReader("4\n今天也辛苦了\nSHARE\n")
	if err := runAnonymousShare(input, io.Discard, defaultConfig(), time.Now()); err != nil {
		t.Fatalf("runAnonymousShare: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/rest/v1/rpc/submit_public_data" || paths[1] != "/rest/v1/rpc/submit_public_message" {
		t.Fatalf("paths = %#v", paths)
	}
	if _, exists := bodies[0]["p_message_text"]; exists {
		t.Fatalf("numeric payload contains message: %#v", bodies[0])
	}
	if bodies[1]["p_message_kind"] != "encourage" || bodies[1]["p_message_text"] != "今天也辛苦了" {
		t.Fatalf("message payload = %#v", bodies[1])
	}
}

func TestRunAnonymousShareTreatsMessageFailureAsPartialSuccess(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if request.URL.Path == "/rest/v1/rpc/submit_public_message" {
			http.Error(output, "moderation unavailable", http.StatusServiceUnavailable)
			return
		}
		output.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")
	var output strings.Builder

	err := runAnonymousShare(strings.NewReader("3\n希望准点下班\nSHARE\n"), &output, defaultConfig(), time.Now())
	if err != nil || requests.Load() != 2 {
		t.Fatalf("partial submission = requests %d, error %v", requests.Load(), err)
	}
	if !strings.Contains(output.String(), "数值已成功提交，请勿重复提交数值") {
		t.Fatalf("output = %q", output.String())
	}
}

type failingAnonymousPreviewWriter struct{}

func (failingAnonymousPreviewWriter) Write([]byte) (int, error) {
	return 0, errors.New("terminal closed")
}

func TestRunAnonymousShareDoesNotConnectWhenPreviewFails(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")

	err := runAnonymousShare(strings.NewReader("0\nSHARE\n"), failingAnonymousPreviewWriter{}, defaultConfig(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "显示匿名贡献预览") {
		t.Fatalf("preview error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("failed preview made %d requests", requests.Load())
	}
}

func TestValidateAnonymousMessage(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("愿", 81), "访问 https://example.com", "第一行\n第二行"} {
		if err := validateAnonymousMessage(value); err == nil {
			t.Errorf("validateAnonymousMessage(%q) succeeded", value)
		}
	}
	if err := validateAnonymousMessage("希望大家准点下班"); err != nil {
		t.Fatalf("valid message: %v", err)
	}
}

func TestAnonymousShareRejectsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, request *http.Request) {
		http.Redirect(output, request, "https://example.com/collect", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	t.Setenv("YUXIN_PUBLIC_API_URL", server.URL)
	t.Setenv("YUXIN_PUBLIC_API_KEY", "sb_publishable_test")

	err := runAnonymousShare(strings.NewReader("0\nSHARE\n"), io.Discard, defaultConfig(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "不应重定向") {
		t.Fatalf("redirect error = %v", err)
	}
}

func TestSubmitAnonymousContributionRejectsErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(output http.ResponseWriter, _ *http.Request) {
		http.Error(output, "policy rejected", http.StatusForbidden)
	}))
	defer server.Close()
	err := submitAnonymousContribution(context.Background(), server.Client(), server.URL, "key", anonymousContribution{})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "policy rejected") {
		t.Fatalf("response error = %v", err)
	}
}

func TestAnonymousIntervalsMatchPublishedBuckets(t *testing.T) {
	if salaryInterval(11_900) != "8千–1.2万" || workMinutesInterval(600) != "10–12小时" || savingsInterval(1_000_000) != "100万以上" || retirementInterval(31) != "31–40年" {
		t.Fatal("interval boundary mismatch")
	}
}
