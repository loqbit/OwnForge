package outbox

import "encoding/json"

// EncodePayload encodes any business event as a JSON payload.
// This keeps the service layer from calling json.Marshal everywhere.
func EncodePayload(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// EncodeHeaders encodes event headers as JSON.
// When headers are empty, it returns nil so callers can persist them conditionally.
// Headers may be unused for now, but reserving them makes it easier to add trace_id, source, and similar metadata later.
func EncodeHeaders(headers map[string]string) (json.RawMessage, error) {
	if len(headers) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(headers)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// NewJSONRecord builds an outbox record whose payload is JSON.
// This is the recommended entry point for direct use in the business layer:
// give it a business payload and it returns a standardized Record.
func NewJSONRecord(id, aggregateType, aggregateID, eventType string, payload any, headers map[string]string) (*Record, error) {
	payloadBytes, err := EncodePayload(payload)
	if err != nil {
		return nil, err
	}

	headerBytes, err := EncodeHeaders(headers)
	if err != nil {
		return nil, err
	}

	return NewRecord(id, aggregateType, aggregateID, eventType, payloadBytes, headerBytes), nil
}
