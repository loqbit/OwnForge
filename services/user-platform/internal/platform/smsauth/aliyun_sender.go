package smsauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	dypnsclient "github.com/alibabacloud-go/dypnsapi-20170525/v3/client"
	"github.com/alibabacloud-go/tea/dara"
	"github.com/ownforge/ownforge/services/user-platform/internal/platform/config"
	"go.uber.org/zap"
)

var (
	ErrSenderDisabled      = errors.New("sms auth sender is disabled")
	ErrSenderMisconfigured = errors.New("sms auth sender is misconfigured")
	ErrSendFrequency       = errors.New("sms auth frequency limited")
)

// SendVerifyCodeInput 表示发送手机验证码时需要的业务参数。
type SendVerifyCodeInput struct {
	Phone string
	Scene string
}

// SendVerifyCodeResult 表示验证码发送成功后返回的服务商侧元信息。
type SendVerifyCodeResult struct {
	BizID           string
	DebugCode       string
	CooldownSeconds int
}

// CheckVerifyCodeInput 表示校验手机验证码时需要的业务参数。
type CheckVerifyCodeInput struct {
	Phone string
	Code  string
	BizID string
	Scene string
}

// CheckVerifyCodeResult 表示服务商是否认可本次验证码校验请求。
type CheckVerifyCodeResult struct {
	Passed bool
}

// Sender 抽象上游短信验证码服务商的发送与校验能力。
type Sender interface {
	SendVerifyCode(ctx context.Context, input SendVerifyCodeInput) (*SendVerifyCodeResult, error)
	CheckVerifyCode(ctx context.Context, input CheckVerifyCodeInput) (*CheckVerifyCodeResult, error)
}

// AliyunSender 通过阿里云 Dypnsapi 实现验证码发送与校验。
type AliyunSender struct {
	cfg     config.SMSAuthConfig
	log     *zap.Logger
	client  *dypnsclient.Client
	runtime *dara.RuntimeOptions
	initErr error
}

// NewAliyunSender 根据应用配置构造阿里云短信验证码发送器。
func NewAliyunSender(cfg config.SMSAuthConfig, log *zap.Logger) Sender {
	sender := &AliyunSender{
		cfg:     cfg,
		log:     log,
		runtime: &dara.RuntimeOptions{},
	}
	if !cfg.Enabled {
		return sender
	}

	client, err := newAliyunClient(cfg)
	if err != nil {
		sender.initErr = err
		if log != nil {
			log.Error("初始化阿里云短信认证客户端失败", zap.Error(err))
		}
		return sender
	}

	sender.client = client
	return sender
}

// SendVerifyCode 调用阿里云向目标手机号发送验证码短信。
func (s *AliyunSender) SendVerifyCode(ctx context.Context, input SendVerifyCodeInput) (*SendVerifyCodeResult, error) {
	_ = ctx

	if err := s.validateReady(); err != nil {
		return nil, err
	}

	request, err := s.buildSendRequest(input)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.SendSmsVerifyCodeWithOptions(request, s.runtime)
	if err != nil {
		return nil, fmt.Errorf("调用阿里云发送验证码接口失败: %w", err)
	}
	if err := validateSendResponse(resp); err != nil {
		return nil, err
	}

	result := &SendVerifyCodeResult{
		CooldownSeconds: int(s.intervalSeconds()),
	}
	if resp != nil && resp.Body != nil && resp.Body.Model != nil {
		result.BizID = dara.StringValue(resp.Body.Model.BizId)
		result.DebugCode = dara.StringValue(resp.Body.Model.VerifyCode)
	}
	return result, nil
}

// CheckVerifyCode 调用阿里云校验提交的短信验证码是否有效。
func (s *AliyunSender) CheckVerifyCode(ctx context.Context, input CheckVerifyCodeInput) (*CheckVerifyCodeResult, error) {
	_ = ctx

	if err := s.validateReady(); err != nil {
		return nil, err
	}

	request := &dypnsclient.CheckSmsVerifyCodeRequest{
		PhoneNumber:    dara.String(strings.TrimSpace(input.Phone)),
		VerifyCode:     dara.String(strings.TrimSpace(input.Code)),
		CountryCode:    dara.String(s.countryCode()),
		CaseAuthPolicy: dara.Int64(1),
	}
	if schemeName := strings.TrimSpace(s.cfg.SchemeName); schemeName != "" {
		request.SchemeName = dara.String(schemeName)
	}

	resp, err := s.client.CheckSmsVerifyCodeWithOptions(request, s.runtime)
	if err != nil {
		return nil, fmt.Errorf("调用阿里云校验验证码接口失败: %w", err)
	}
	if err := validateCheckResponse(resp); err != nil {
		return nil, err
	}

	return &CheckVerifyCodeResult{
		Passed: strings.EqualFold(dara.StringValue(resp.Body.Model.VerifyResult), "PASS"),
	}, nil
}

// validateReady 确保发送器已启用且底层客户端初始化成功。
func (s *AliyunSender) validateReady() error {
	if !s.cfg.Enabled {
		return ErrSenderDisabled
	}
	if s.initErr != nil {
		return s.initErr
	}
	if s.client == nil {
		return fmt.Errorf("%w: client is nil", ErrSenderMisconfigured)
	}
	return nil
}

// buildSendRequest 根据配置和运行时输入组装阿里云发码请求。
func (s *AliyunSender) buildSendRequest(input SendVerifyCodeInput) (*dypnsclient.SendSmsVerifyCodeRequest, error) {
	templateParam, err := s.templateParamJSON()
	if err != nil {
		return nil, err
	}

	request := &dypnsclient.SendSmsVerifyCodeRequest{
		SignName:         dara.String(strings.TrimSpace(s.cfg.SignName)),
		TemplateCode:     dara.String(strings.TrimSpace(s.cfg.TemplateCode)),
		PhoneNumber:      dara.String(strings.TrimSpace(input.Phone)),
		TemplateParam:    dara.String(templateParam),
		CodeLength:       dara.Int64(s.codeLength()),
		CountryCode:      dara.String(s.countryCode()),
		Interval:         dara.Int64(s.intervalSeconds()),
		CodeType:         dara.Int64(s.codeType()),
		DuplicatePolicy:  dara.Int64(s.duplicatePolicy()),
		ReturnVerifyCode: dara.Bool(s.cfg.DebugMode),
		AutoRetry:        dara.Int64(s.autoRetry()),
		ValidTime:        dara.Int64(s.validTimeSeconds()),
	}
	if schemeName := strings.TrimSpace(s.cfg.SchemeName); schemeName != "" {
		request.SchemeName = dara.String(schemeName)
	}

	return request, nil
}

// templateParamJSON 返回阿里云短信模板所需的 JSON 参数串。
func (s *AliyunSender) templateParamJSON() (string, error) {
	if value := strings.TrimSpace(s.cfg.TemplateParamJSON); value != "" {
		if !json.Valid([]byte(value)) {
			return "", fmt.Errorf("%w: template_param_json 不是合法 JSON", ErrSenderMisconfigured)
		}
		return value, nil
	}

	payload := map[string]string{
		"code": "##code##",
		"min":  fmt.Sprintf("%d", maxInt64(1, s.validTimeSeconds()/60)),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化默认模板参数失败: %w", err)
	}
	return string(raw), nil
}

// validateSendResponse 检查阿里云是否成功受理发码请求。
func validateSendResponse(resp *dypnsclient.SendSmsVerifyCodeResponse) error {
	if resp == nil || resp.Body == nil {
		return errors.New("阿里云发送验证码响应为空")
	}
	if !dara.BoolValue(resp.Body.Success) || !strings.EqualFold(dara.StringValue(resp.Body.Code), "OK") {
		if strings.EqualFold(dara.StringValue(resp.Body.Code), "biz.FREQUENCY") {
			return fmt.Errorf("%w(code=%s, message=%s)", ErrSendFrequency, dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
		}
		return fmt.Errorf("阿里云发送验证码失败(code=%s, message=%s)", dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
	}
	return nil
}

// validateCheckResponse 检查阿里云是否返回了有效的验码结果。
func validateCheckResponse(resp *dypnsclient.CheckSmsVerifyCodeResponse) error {
	if resp == nil || resp.Body == nil {
		return errors.New("阿里云校验验证码响应为空")
	}
	if !dara.BoolValue(resp.Body.Success) || !strings.EqualFold(dara.StringValue(resp.Body.Code), "OK") {
		return fmt.Errorf("阿里云校验验证码失败(code=%s, message=%s)", dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
	}
	if resp.Body.Model == nil {
		return errors.New("阿里云校验验证码响应缺少 Model")
	}
	return nil
}

// newAliyunClient 根据短信配置创建底层阿里云 Dypnsapi 客户端。
func newAliyunClient(cfg config.SMSAuthConfig) (*dypnsclient.Client, error) {
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, fmt.Errorf("%w: access key 未配置", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("%w: region 未配置", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.SignName) == "" {
		return nil, fmt.Errorf("%w: sign_name 未配置", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.TemplateCode) == "" {
		return nil, fmt.Errorf("%w: template_code 未配置", ErrSenderMisconfigured)
	}

	openapiCfg := &openapiutil.Config{
		AccessKeyId:     dara.String(strings.TrimSpace(cfg.AccessKeyID)),
		AccessKeySecret: dara.String(strings.TrimSpace(cfg.AccessKeySecret)),
		RegionId:        dara.String(strings.TrimSpace(cfg.Region)),
	}
	openapiCfg.Endpoint = dara.String("dypnsapi.aliyuncs.com")
	return dypnsclient.NewClient(openapiCfg)
}

// countryCode 返回配置中的国家码，默认回退为中国大陆区号。
func (s *AliyunSender) countryCode() string {
	if value := strings.TrimSpace(s.cfg.CountryCode); value != "" {
		return value
	}
	return "86"
}

// codeLength 返回配置中的验证码长度，未配置时使用默认值。
func (s *AliyunSender) codeLength() int64 {
	return defaultInt64(s.cfg.CodeLength, 6)
}

// intervalSeconds 返回配置中的重发冷却秒数。
func (s *AliyunSender) intervalSeconds() int64 {
	return defaultInt64(s.cfg.IntervalSeconds, 60)
}

// validTimeSeconds 返回配置中的验证码有效期秒数。
func (s *AliyunSender) validTimeSeconds() int64 {
	return defaultInt64(s.cfg.ValidTimeSeconds, 300)
}

// codeType 返回配置中的验证码字符类型策略。
func (s *AliyunSender) codeType() int64 {
	return defaultInt64(s.cfg.CodeType, 1)
}

// duplicatePolicy 返回有效期内重复发码时的处理策略。
func (s *AliyunSender) duplicatePolicy() int64 {
	return defaultInt64(s.cfg.DuplicatePolicy, 1)
}

// autoRetry 返回配置中的阿里云自动重试策略。
func (s *AliyunSender) autoRetry() int64 {
	return defaultInt64(s.cfg.AutoRetry, 1)
}

// defaultInt64 在配置值未设置或非法时返回回退值。
func defaultInt64(value int64, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

// maxInt64 返回两个 int64 中更大的那个值。
func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
