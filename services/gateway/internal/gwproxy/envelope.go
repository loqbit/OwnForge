// Package gwproxy 提供 gRPC-Gateway 与 Gin 网关的集成工具，
// 包括响应信封包装和 gRPC-Gateway Mux 的初始化。
package gwproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ownforge/ownforge/pkg/errs"
)

// responseRecorder 拦截 gRPC-Gateway 的 HTTP 响应以便包装为信封格式。
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

// WrapHandler 将 gRPC-Gateway 的 raw proto JSON 输出包装为网关统一信封格式。
//
// 成功响应: {"code": 0, "msg": "success", "data": <proto_json>}
// 错误响应: {"code": <err_code>, "msg": "<err_msg>", "data": null}
//
// 这保证了迁移对前端完全透明，不需要修改任何 API 调用逻辑。
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
			// 错误路径：解析 gRPC-Gateway 的错误输出并重新包装
			writeErrorEnvelope(w, rec.statusCode, rec.body.Bytes())
			return
		}

		// 成功路径：用信封包装原始 proto JSON
		rawData := rec.body.Bytes()

		// 处理空响应体（如 DeleteSnippet 等返回少量数据的场景）
		if len(rawData) == 0 {
			rawData = []byte("null")
		}

		envelope := fmt.Sprintf(`{"code":%d,"msg":"success","data":%s}`, errs.Success, string(rawData))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(envelope))
	})
}

// grpcGatewayError 是 gRPC-Gateway 默认的错误 JSON 结构。
type grpcGatewayError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// writeErrorEnvelope 将 gRPC-Gateway 的错误响应转换为网关信封格式。
func writeErrorEnvelope(w http.ResponseWriter, httpStatus int, body []byte) {
	var gwErr grpcGatewayError
	if err := json.Unmarshal(body, &gwErr); err != nil {
		// 解析失败，生成通用错误
		w.WriteHeader(httpStatus)
		w.Write([]byte(fmt.Sprintf(
			`{"code":%d,"msg":"系统繁忙","data":null}`, errs.ServerErr,
		)))
		return
	}

	// 映射 gRPC 错误码到网关业务码
	errCode := mapGRPCStatusToCode(gwErr.Code)
	errMsg := gwErr.Message
	if errMsg == "" {
		errMsg = "系统繁忙"
	}

	w.WriteHeader(httpStatus)
	w.Write([]byte(fmt.Sprintf(
		`{"code":%d,"msg":%s,"data":null}`, errCode, strconv.Quote(errMsg),
	)))
}

// mapGRPCStatusToCode 将 gRPC status code 映射为网关统一的业务错误码。
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
