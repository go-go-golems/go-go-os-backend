package pinoweb

import (
	"encoding/json"
	"net/http"
	"strings"

	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	"github.com/google/uuid"
)

type StrictRequestResolver struct {
	runtimeKey string
}

func NewStrictRequestResolver(runtimeKey string) *StrictRequestResolver {
	key := strings.TrimSpace(runtimeKey)
	if key == "" {
		key = "inventory"
	}
	return &StrictRequestResolver{runtimeKey: key}
}

func (r *StrictRequestResolver) Resolve(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	if req == nil {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "bad request",
		}
	}

	switch req.Method {
	case http.MethodGet:
		return r.resolveWS(req)
	case http.MethodPost:
		return r.resolveChat(req)
	default:
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{
			Status:    http.StatusMethodNotAllowed,
			ClientMsg: "method not allowed",
		}
	}
}

func (r *StrictRequestResolver) resolveWS(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	convID := strings.TrimSpace(req.URL.Query().Get("conv_id"))
	if convID == "" {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "missing conv_id",
		}
	}

	return webhttp.ConversationRequestPlan{
		ConvID:     convID,
		RuntimeKey: r.runtimeKey,
		Overrides:  nil,
	}, nil
}

func (r *StrictRequestResolver) resolveChat(req *http.Request) (webhttp.ConversationRequestPlan, error) {
	var body webhttp.ChatRequestBody
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "bad request",
			Err:       err,
		}
	}
	if body.Prompt == "" && body.Text != "" {
		body.Prompt = body.Text
	}
	if len(body.Overrides) > 0 {
		return webhttp.ConversationRequestPlan{}, &webhttp.RequestResolutionError{
			Status:    http.StatusBadRequest,
			ClientMsg: "runtime overrides are not allowed",
		}
	}

	convID := strings.TrimSpace(body.ConvID)
	if convID == "" {
		convID = uuid.NewString()
	}

	return webhttp.ConversationRequestPlan{
		ConvID:         convID,
		RuntimeKey:     r.runtimeKey,
		Prompt:         body.Prompt,
		Overrides:      nil,
		IdempotencyKey: strings.TrimSpace(body.IdempotencyKey),
	}, nil
}
