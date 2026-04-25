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
	"github.com/loqbit/ownforge/services/identity/internal/platform/config"
	"go.uber.org/zap"
)

var (
	ErrSenderDisabled      = errors.New("sms auth sender is disabled")
	ErrSenderMisconfigured = errors.New("sms auth sender is misconfigured")
	ErrSendFrequency       = errors.New("sms auth frequency limited")
)

// SendVerifyCodeInput contains the business parameters required to send a phone verification code.
type SendVerifyCodeInput struct {
	Phone string
	Scene string
}

// SendVerifyCodeResult contains provider-side metadata returned after a verification code is sent successfully.
type SendVerifyCodeResult struct {
	BizID           string
	DebugCode       string
	CooldownSeconds int
}

// CheckVerifyCodeInput contains the business parameters required to verify a phone code.
type CheckVerifyCodeInput struct {
	Phone string
	Code  string
	BizID string
	Scene string
}

// CheckVerifyCodeResult reports whether the provider accepts this verification request.
type CheckVerifyCodeResult struct {
	Passed bool
}

// Sender abstracts the send and verify capabilities of upstream SMS verification providers.
type Sender interface {
	SendVerifyCode(ctx context.Context, input SendVerifyCodeInput) (*SendVerifyCodeResult, error)
	CheckVerifyCode(ctx context.Context, input CheckVerifyCodeInput) (*CheckVerifyCodeResult, error)
}

// AliyunSender implements code sending and verification through Aliyun Dypnsapi.
type AliyunSender struct {
	cfg     config.SMSAuthConfig
	log     *zap.Logger
	client  *dypnsclient.Client
	runtime *dara.RuntimeOptions
	initErr error
}

// NewAliyunSender builds an Aliyun SMS verification sender from application configuration.
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
			log.Error("failed to initialize Alibaba Cloud SMS auth client", zap.Error(err))
		}
		return sender
	}

	sender.client = client
	return sender
}

// SendVerifyCode asks Aliyun to send a verification code SMS to the target phone number.
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
		return nil, fmt.Errorf("failed to call Alibaba Cloud send-code API: %w", err)
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

// CheckVerifyCode asks Aliyun to validate whether the submitted SMS verification code is valid.
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
		return nil, fmt.Errorf("failed to call Alibaba Cloud verify-code API: %w", err)
	}
	if err := validateCheckResponse(resp); err != nil {
		return nil, err
	}

	return &CheckVerifyCodeResult{
		Passed: strings.EqualFold(dara.StringValue(resp.Body.Model.VerifyResult), "PASS"),
	}, nil
}

// validateReady makes sure the sender is enabled and the underlying client is initialized.
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

// buildSendRequest assembles the Aliyun send-code request from configuration and runtime input.
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

// templateParamJSON returns the JSON parameter string required by the Aliyun SMS template.
func (s *AliyunSender) templateParamJSON() (string, error) {
	if value := strings.TrimSpace(s.cfg.TemplateParamJSON); value != "" {
		if !json.Valid([]byte(value)) {
			return "", fmt.Errorf("%w: template_param_json is not valid JSON", ErrSenderMisconfigured)
		}
		return value, nil
	}

	payload := map[string]string{
		"code": "##code##",
		"min":  fmt.Sprintf("%d", maxInt64(1, s.validTimeSeconds()/60)),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal default template parameters: %w", err)
	}
	return string(raw), nil
}

// validateSendResponse checks whether Aliyun accepted the send-code request successfully.
func validateSendResponse(resp *dypnsclient.SendSmsVerifyCodeResponse) error {
	if resp == nil || resp.Body == nil {
		return errors.New("Alibaba Cloud send-code response is empty")
	}
	if !dara.BoolValue(resp.Body.Success) || !strings.EqualFold(dara.StringValue(resp.Body.Code), "OK") {
		if strings.EqualFold(dara.StringValue(resp.Body.Code), "biz.FREQUENCY") {
			return fmt.Errorf("%w(code=%s, message=%s)", ErrSendFrequency, dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
		}
		return fmt.Errorf("Alibaba Cloud send-code failed(code=%s, message=%s)", dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
	}
	return nil
}

// validateCheckResponse checks whether Aliyun returned a valid verification result.
func validateCheckResponse(resp *dypnsclient.CheckSmsVerifyCodeResponse) error {
	if resp == nil || resp.Body == nil {
		return errors.New("Alibaba Cloud verify-code response is empty")
	}
	if !dara.BoolValue(resp.Body.Success) || !strings.EqualFold(dara.StringValue(resp.Body.Code), "OK") {
		return fmt.Errorf("Alibaba Cloud verify-code failed(code=%s, message=%s)", dara.StringValue(resp.Body.Code), dara.StringValue(resp.Body.Message))
	}
	if resp.Body.Model == nil {
		return errors.New("Alibaba Cloud verify-code response is missing Model")
	}
	return nil
}

// newAliyunClient creates the underlying Aliyun Dypnsapi client from SMS configuration.
func newAliyunClient(cfg config.SMSAuthConfig) (*dypnsclient.Client, error) {
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, fmt.Errorf("%w: access key is not configured", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.Region) == "" {
		return nil, fmt.Errorf("%w: region is not configured", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.SignName) == "" {
		return nil, fmt.Errorf("%w: sign_name is not configured", ErrSenderMisconfigured)
	}
	if strings.TrimSpace(cfg.TemplateCode) == "" {
		return nil, fmt.Errorf("%w: template_code is not configured", ErrSenderMisconfigured)
	}

	openapiCfg := &openapiutil.Config{
		AccessKeyId:     dara.String(strings.TrimSpace(cfg.AccessKeyID)),
		AccessKeySecret: dara.String(strings.TrimSpace(cfg.AccessKeySecret)),
		RegionId:        dara.String(strings.TrimSpace(cfg.Region)),
	}
	openapiCfg.Endpoint = dara.String("dypnsapi.aliyuncs.com")
	return dypnsclient.NewClient(openapiCfg)
}

// countryCode returns the configured country code and falls back to mainland China by default.
func (s *AliyunSender) countryCode() string {
	if value := strings.TrimSpace(s.cfg.CountryCode); value != "" {
		return value
	}
	return "86"
}

// codeLength returns the configured code length, or the default when not set.
func (s *AliyunSender) codeLength() int64 {
	return defaultInt64(s.cfg.CodeLength, 6)
}

// intervalSeconds returns the configured resend cooldown in seconds.
func (s *AliyunSender) intervalSeconds() int64 {
	return defaultInt64(s.cfg.IntervalSeconds, 60)
}

// validTimeSeconds returns the configured code validity period in seconds.
func (s *AliyunSender) validTimeSeconds() int64 {
	return defaultInt64(s.cfg.ValidTimeSeconds, 300)
}

// codeType returns the configured character-type policy for verification codes.
func (s *AliyunSender) codeType() int64 {
	return defaultInt64(s.cfg.CodeType, 1)
}

// duplicatePolicy returns the configured behavior for repeated sends during the valid window.
func (s *AliyunSender) duplicatePolicy() int64 {
	return defaultInt64(s.cfg.DuplicatePolicy, 1)
}

// autoRetry returns the configured Aliyun automatic retry policy.
func (s *AliyunSender) autoRetry() int64 {
	return defaultInt64(s.cfg.AutoRetry, 1)
}

// defaultInt64 returns a fallback value when the configured value is missing or invalid.
func defaultInt64(value int64, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

// maxInt64 returns the larger of two int64 values.
func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
