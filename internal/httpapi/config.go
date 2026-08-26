package httpapi

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func Addr(fs *flag.FlagSet) *string {
	def := "127.0.0.1:19081"
	if p := os.Getenv("PORT"); p != "" {
		if _, e := strconv.Atoi(p); e == nil {
			def = "127.0.0.1:" + p
		} else if strings.Contains(p, ":") {
			def = p
		}
	}
	return fs.String("addr", def, "监听地址")
}
func NormalizeAddr(v string) string {
	if strings.HasPrefix(v, ":") {
		return "127.0.0.1" + v
	}
	if !strings.Contains(v, ":") {
		return fmt.Sprintf("127.0.0.1:%s", v)
	}
	return v
}
