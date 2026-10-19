package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/openware-io/open-green-pass/internal/platform/db"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
)

// AssistantResponder resolves the team's approved model connection and uses
// the same provider boundary as generation. It never accepts a token or URL
// from the browser.
type AssistantResponder struct{ db *db.DB }

const assistantModelAttemptTimeout = 12 * time.Second

func NewAssistantResponder(database *db.DB) *AssistantResponder {
	return &AssistantResponder{db: database}
}

func (r *AssistantResponder) Respond(ctx context.Context, page, question string) (string, string, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok || teamID <= 0 {
		return "", "", errors.New("assistant model requires tenant")
	}
	var input modelInput
	err := r.db.WithTenant(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT provider,model_key,protocol,base_url,COALESCE(secret_ref,'')
			FROM mdl_model WHERE team_id=$1 AND status='approved'
			AND credential_status IN ('configured','verified')
			ORDER BY configured_at DESC NULLS LAST, id DESC LIMIT 1`, teamID).
			Scan(&input.Provider, &input.Model, &input.Protocol, &input.BaseURL, &input.SecretRef)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", errors.New("no approved AI model is configured for this team")
	}
	if err != nil {
		return "", "", err
	}
	token, err := resolveModelToken(ctx, input.SecretRef)
	if err != nil {
		return "", "", err
	}
	prompt := fmt.Sprintf(`你是 GreenPass 软件工程测试治理平台的 AI 助手。请基于“%s”页面上下文回答用户问题。
用户问题：%s

要求：
1. 只讨论测试治理、质量门禁、用例、执行、证据和审计相关内容。
2. 不要声称已经执行了任何界面操作、提交了任何表单或修改了系统数据。
3. 如果上下文事实不足，明确说明缺少哪些证据，不要编造运行编号、规则结果或数据。
4. 给出清晰的结论、依据和下一步建议，中文回答，控制在 500 字以内。`, page, question)
	call := func() (string, error) {
		attemptCtx, cancel := context.WithTimeout(ctx, assistantModelAttemptTimeout)
		defer cancel()
		return callModel(attemptCtx, input, token, prompt)
	}
	answer, err := call()
	if err != nil {
		// Provider gateways can transiently close an otherwise valid request at
		// their edge. One bounded retry avoids exposing that transient as a
		// misleading "conversation write" failure in the UI.
		if ctx.Err() != nil {
			return "", input.Provider + ":" + input.Model, ctx.Err()
		}
		answer, err = call()
		if err != nil {
			return "", input.Provider + ":" + input.Model, err
		}
	}
	return strings.TrimSpace(answer), input.Provider + ":" + input.Model, nil
}
