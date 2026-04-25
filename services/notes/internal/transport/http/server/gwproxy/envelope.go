package gwproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/loqbit/ownforge/pkg/errs"
)

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

// WrapHandler wraps raw grpc-gateway JSON into the standard {code,msg,data} envelope.
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
			writeErrorEnvelope(w, rec.statusCode, rec.body.Bytes())
			return
		}

		rawData := rec.body.Bytes()
		if len(rawData) == 0 {
			rawData = []byte("null")
		}

		envelope := fmt.Sprintf(`{"code":%d,"msg":"success","data":%s}`, errs.Success, string(rawData))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(envelope))
	})
}

type grpcGatewayError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func writeErrorEnvelope(w http.ResponseWriter, httpStatus int, body []byte) {
	var gwErr grpcGatewayError
	if err := json.Unmarshal(body, &gwErr); err != nil {
		w.WriteHeader(httpStatus)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"code":%d,"msg":"system busy","data":null}`, errs.ServerErr)))
		return
	}

	errCode := mapGRPCStatusToCode(gwErr.Code)
	errMsg := gwErr.Message
	if errMsg == "" {
		errMsg = "system busy"
	}

	w.WriteHeader(httpStatus)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"code":%d,"msg":%s,"data":null}`, errCode, strconv.Quote(errMsg))))
}

func mapGRPCStatusToCode(grpcCode int) int {
	switch grpcCode {
	case 3:
		return errs.ParamErr
	case 5:
		return errs.NotFound
	case 7:
		return errs.Forbidden
	case 16:
		return errs.Unauthorized
	default:
		return errs.ServerErr
	}
}
