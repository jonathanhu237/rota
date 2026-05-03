package email

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

type emailTemplateView struct {
	Subject               string
	ProductName           string
	OrganizationName      string
	Name                  string
	ActionURL             string
	CTALabel              string
	FallbackLabel         string
	Footer                string
	ExpirationText        string
	NewEmailPartial       string
	RequesterName         string
	ResponderName         string
	RequestTypeLabel      string
	OutcomeLabel          string
	RequesterShiftLabel   string
	CounterpartShiftLabel string
	RequestsURL           string
	HasOrganizationName   bool
	HasCounterpartShift   bool
}

func renderAccountMessage(kind string, data TemplateData) Message {
	language := normalizeLanguage(data.Language)
	view := baseTemplateView(kind, language, data.Name, data.Branding)
	view.ExpirationText = humanizeDurationForLanguage(data.Expiration, language)

	switch kind {
	case KindInvitation:
		view.ActionURL = setupPasswordLink(data.BaseURL, data.Token)
		view.CTALabel = localizedString(language, "Set password", "设置密码")
	case KindPasswordReset:
		view.ActionURL = setupPasswordLink(data.BaseURL, data.Token)
		view.CTALabel = localizedString(language, "Reset password", "重置密码")
	case KindEmailChangeConfirm:
		view.ActionURL = emailChangeConfirmLink(data.BaseURL, data.Token)
		view.CTALabel = localizedString(language, "Confirm email change", "确认邮箱变更")
	case KindEmailChangeNotice:
		view.NewEmailPartial = data.NewEmailPartial
	default:
		view.ActionURL = setupPasswordLink(data.BaseURL, data.Token)
	}

	return renderMessage(kind, language, data.To, view)
}

func renderShiftChangeRequestReceivedMessage(data ShiftChangeRequestReceivedData) Message {
	language := normalizeLanguage(data.Language)
	view := baseTemplateView(KindShiftChangeRequestReceived, language, data.RecipientName, data.Branding)
	view.ActionURL = requestsLink(data.BaseURL)
	view.RequestsURL = view.ActionURL
	view.CTALabel = localizedString(language, "View request", "查看申请")
	view.RequesterName = data.RequesterName
	view.RequestTypeLabel = shiftChangeTypeLabel(data.Type, language)
	view.RequesterShiftLabel = FormatShiftRef(data.RequesterShift, language)
	if data.CounterpartShift != nil {
		view.CounterpartShiftLabel = FormatShiftRef(*data.CounterpartShift, language)
		view.HasCounterpartShift = true
	}

	return renderMessage(KindShiftChangeRequestReceived, language, data.To, view)
}

func renderShiftChangeResolvedMessage(data ShiftChangeResolvedData) Message {
	language := normalizeLanguage(data.Language)
	view := baseTemplateView(KindShiftChangeResolved, language, data.RecipientName, data.Branding)
	view.ActionURL = requestsLink(data.BaseURL)
	view.RequestsURL = view.ActionURL
	view.CTALabel = localizedString(language, "View request", "查看申请")
	view.OutcomeLabel = shiftChangeOutcomeLabel(data.Outcome, language)
	view.Subject = shiftChangeResolvedSubject(data.Outcome, language, data.Branding)
	view.ResponderName = data.ResponderName
	view.RequestTypeLabel = shiftChangeTypeLabel(data.Type, language)
	if !isZeroShiftRef(data.RequesterShift) {
		view.RequesterShiftLabel = FormatShiftRef(data.RequesterShift, language)
	}
	if data.CounterpartShift != nil {
		view.CounterpartShiftLabel = FormatShiftRef(*data.CounterpartShift, language)
		view.HasCounterpartShift = true
	}

	return renderMessage(KindShiftChangeResolved, language, data.To, view)
}

func baseTemplateView(kind string, language string, name string, branding Branding) emailTemplateView {
	normalizedBranding := NormalizeBranding(branding)
	subject := subjectFor(kind, language, normalizedBranding)
	return emailTemplateView{
		Subject:             subject,
		ProductName:         normalizedBranding.ProductName,
		OrganizationName:    normalizedBranding.OrganizationName,
		Name:                displayName(name, language),
		FallbackLabel:       localizedString(language, "If the button does not work, copy and paste this link into your browser:", "如果按钮无法打开，请复制以下链接到浏览器："),
		Footer:              localizedFooter(language, normalizedBranding),
		HasOrganizationName: normalizedBranding.OrganizationName != "",
	}
}

func renderMessage(kind string, language string, to string, view emailTemplateView) Message {
	return Message{
		Kind:     kind,
		To:       to,
		Subject:  view.Subject,
		Body:     mustRenderText(kind, language, view),
		HTMLBody: mustRenderHTML(kind, language, view),
	}
}

func mustRenderText(kind string, language string, view emailTemplateView) string {
	filename := fmt.Sprintf("templates/%s.%s.txt.tmpl", kind, language)
	parsed, err := texttemplate.ParseFS(templateFS, filename)
	if err != nil {
		panic(err)
	}

	var body bytes.Buffer
	if err := parsed.Execute(&body, view); err != nil {
		panic(err)
	}
	return body.String()
}

func mustRenderHTML(kind string, language string, view emailTemplateView) string {
	filename := fmt.Sprintf("templates/%s.%s.html.tmpl", kind, language)
	parsed, err := htmltemplate.ParseFS(templateFS, "templates/layout.html.tmpl", filename)
	if err != nil {
		panic(err)
	}

	var body bytes.Buffer
	if err := parsed.ExecuteTemplate(&body, "layout", view); err != nil {
		panic(err)
	}
	return body.String()
}

func subjectFor(kind string, language string, branding Branding) string {
	normalizedBranding := NormalizeBranding(branding)
	productName := normalizedBranding.ProductName
	subjects := map[string]map[string]string{
		"en": {
			KindInvitation:                 fmt.Sprintf("Invitation to %s", productName),
			KindPasswordReset:              fmt.Sprintf("Reset your %s password", productName),
			KindEmailChangeConfirm:         fmt.Sprintf("Confirm your %s email change", productName),
			KindEmailChangeNotice:          fmt.Sprintf("%s email change requested", productName),
			KindShiftChangeRequestReceived: "New shift change request",
			KindShiftChangeResolved:        "Shift change request update",
		},
		"zh": {
			KindInvitation:                 fmt.Sprintf("%s 邀请", productName),
			KindPasswordReset:              fmt.Sprintf("重置 %s 密码", productName),
			KindEmailChangeConfirm:         fmt.Sprintf("确认 %s 邮箱变更", productName),
			KindEmailChangeNotice:          fmt.Sprintf("%s 邮箱变更请求", productName),
			KindShiftChangeRequestReceived: "新的换班申请",
			KindShiftChangeResolved:        "换班申请状态更新",
		},
	}
	subject := subjects[normalizeLanguage(language)][kind]
	if subject == "" {
		subject = subjects["en"][kind]
	}
	return fmt.Sprintf("[%s] %s", productName, subject)
}

func shiftChangeResolvedSubject(outcome ShiftChangeOutcome, language string, branding Branding) string {
	productName := NormalizeBranding(branding).ProductName
	if normalizeLanguage(language) == "zh" {
		return fmt.Sprintf("[%s] 换班申请%s", productName, shiftChangeOutcomeLabel(outcome, language))
	}
	return fmt.Sprintf("[%s] Shift change request %s", productName, shiftChangeOutcomeLabel(outcome, language))
}

func displayName(name string, language string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		return trimmed
	}
	return localizedString(language, "there", "你好")
}

func localizedString(language string, en string, zh string) string {
	if normalizeLanguage(language) == "zh" {
		return zh
	}
	return en
}

func localizedFooter(language string, branding Branding) string {
	normalizedBranding := NormalizeBranding(branding)
	if normalizeLanguage(language) == "zh" {
		if normalizedBranding.OrganizationName != "" {
			return fmt.Sprintf("这是一封来自%s的%s自动通知邮件。", normalizedBranding.OrganizationName, normalizedBranding.ProductName)
		}
		if normalizedBranding.ProductName == DefaultProductName {
			return "这是一封 Rota 自动通知邮件。"
		}
		return fmt.Sprintf("这是一封%s自动通知邮件。", normalizedBranding.ProductName)
	}
	if normalizedBranding.OrganizationName != "" {
		return fmt.Sprintf("This is an automated %s notification from %s.", normalizedBranding.ProductName, normalizedBranding.OrganizationName)
	}
	return fmt.Sprintf("This is an automated %s notification.", normalizedBranding.ProductName)
}

func humanizeDurationForLanguage(duration time.Duration, language string) string {
	if normalizeLanguage(language) == "zh" {
		switch {
		case duration%time.Hour == 0:
			return fmt.Sprintf("%d 小时", int(duration/time.Hour))
		case duration%time.Minute == 0:
			return fmt.Sprintf("%d 分钟", int(duration/time.Minute))
		default:
			return duration.String()
		}
	}
	return humanizeDuration(duration)
}
