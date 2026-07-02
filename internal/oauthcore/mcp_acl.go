package oauthcore

import "encoding/json"

type AnonymousMCPPolicy struct {
	PublicTools []string
}

type jsonRPCRequest struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type toolCallParams struct {
	Name string `json:"name"`
}

func (p AnonymousMCPPolicy) PayloadAllowed(body []byte) (allowed bool, toolsList bool, reason string) {
	var batch []json.RawMessage
	if err := json.Unmarshal(body, &batch); err == nil {
		if len(batch) == 0 {
			return false, false, "empty_batch"
		}
		containsToolsList := false
		for _, raw := range batch {
			ok, isToolsList, reason := p.messageAllowed(raw)
			if !ok {
				return false, false, reason
			}
			containsToolsList = containsToolsList || isToolsList
		}
		return true, containsToolsList, ""
	}

	return p.messageAllowed(body)
}

func (p AnonymousMCPPolicy) messageAllowed(raw []byte) (allowed bool, toolsList bool, reason string) {
	var msg jsonRPCRequest
	if err := json.Unmarshal(raw, &msg); err != nil {
		return false, false, "invalid_json"
	}

	switch msg.Method {
	case "initialize", "notifications/initialized", "ping":
		return true, false, ""
	case "tools/list":
		return true, true, ""
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return false, false, "invalid_tool_call_params"
		}
		if params.Name == "" {
			return false, false, "missing_tool_name"
		}
		if p.IsPublicTool(params.Name) {
			return true, false, ""
		}
		return false, false, "tool_not_public"
	default:
		return false, false, "method_not_public"
	}
}

func (p AnonymousMCPPolicy) IsPublicTool(name string) bool {
	for _, tool := range p.PublicTools {
		if tool == name {
			return true
		}
	}
	return false
}
