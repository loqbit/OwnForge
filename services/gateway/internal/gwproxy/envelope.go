// Package gwproxy provides integration helpers for gRPC-Gateway and the Gin gateway,
// including response envelope wrapping and gRPC-Gateway Mux setup.
package gwproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ownforge/ownforge/pkg/errs"
)

// responseRecorder intercepts gRPC-Gateway HTTP responses so they can be wrapped in an envelope.
type responseRecorder struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	wroteHdr   bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHdr {
		return
	}
	r.wroteHdr = true
	r.statusCode = code
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHdr {
		r.WriteHeader(http.StatusOK)
	}
	return r.body.Write(b)
}

// WrapHandler wraps raw proto JSON output from gRPC-Gateway into the gateway's standard envelope format.
//
// successful response: {"code": 0, "msg": "success", "data": <proto_json>}
// error response: {"code": <err_code>, "msg": "<err_msg>", "data": null}
//
// This keeps the migration fully transparent to the frontend, with no API-call changes required.
func WrapHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &responseRecorder{
			ResponseWriter: w,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		h.ServeHTTP(rec, r)

		w.Header().Set("Content-Type", "application/json")

		if rec.statusCode >= 400 {
			// Error path: parse the gRPC-Gateway error output and wrap it again.
			writeErrorEnvelope(w, rec.statusCode, rec.body.Bytes())
			return
		}

		// success path: wrap the original proto JSON in an envelope
		rawData := rec.body.Bytes()

		// Handle empty response bodies, such as DeleteSnippet-like endpoints.
		if len(rawData) == 0 {
			rawData = []byte("null")
		}

		envelope := fmt.Sprintf(`{"code":%d,"msg":"success","data":%s}`, errs.Success, string(rawData))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(envelope))
	})
}

// grpcGatewayError is the default gRPC-Gateway error JSON structure.
type grpcGatewayError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// writeErrorEnvelope converts a gRPC-Gateway error response into the gateway envelope format.
func writeErrorEnvelope(w http.ResponseWriter, httpStatus int, body []byte) {
	var gwErr grpcGatewayError
	if err := json.Unmarshal(body, &gwErr); err != nil {
		// If parsing fails, generate a generic error.
		w.WriteHeader(httpStatus)
		w.Write([]byte(fmt.Sprintf(
			`{"code":%d,"msg":"system busy","data":null}`, errs.ServerErr,
		)))
		return
	}

	// map gRPC status codes to gateway business codes
	errCode := mapGRPCStatusToCode(gwErr.Code)
	errMsg := gwErr.Message
	if errMsg == "" {
		errMsg = "system busy"
	}

	w.WriteHeader(httpStatus)
	w.Write([]byte(fmt.Sprintf(
		`{"code":%d,"msg":%s,"data":null}`, errCode, strconv.Quote(errMsg),
	)))
}

// mapGRPCStatusToCode maps a gRPC status code to the gateway's standard business error code.
func mapGRPCStatusToCode(grpcCode int) int {
	switch grpcCode {
	case 3: // InvalidArgument
		return errs.ParamErr
	case 5: // NotFound
		return errs.NotFound
	case 7: // PermissionDenied
		return errs.Forbidden
	case 16: // Unauthenticated
		return errs.Unauthorized
	default:
		return errs.ServerErr
	}
}
