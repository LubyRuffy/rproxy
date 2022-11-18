package checkproxy

import (
	"fmt"
	"net/http"
	"strings"
)

// headerString http response 转换为字符串
func headerString(r *http.Response) string {
	var s string
	s = fmt.Sprintf("%s %d %s\n", r.Proto, r.StatusCode, r.Status)
	for k, v := range r.Header {
		s += k + ": " + strings.Join(v, ",") + "\n"
	}
	return s
}
